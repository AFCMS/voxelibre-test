// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package lint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
)

const (
	luaLSEntrypoint = "/opt/LuaLS/bin/lua-language-server"
	reportPath      = "/tmp/log/check.json"
	cleanupTimeout  = 10 * time.Second
)

type Runtime interface {
	EnsureImage(context.Context, string, container.PullPolicy) error
	Start(context.Context, container.ContainerSpec) (string, error)
	ReadLogs(context.Context, string) ([]byte, error)
	Wait(context.Context, string) (int, error)
	CopyFrom(context.Context, string, string, string) error
	Remove(context.Context, string) error
}

type Runner struct {
	runtime          Runtime
	image            string
	pullPolicy       container.PullPolicy
	cloneDirectory   string
	checkLevel       luals.CheckLevel
	display          Display
	workingDirectory func() (string, error)
}

func NewRunner(
	runtime Runtime,
	image string,
	pullPolicy container.PullPolicy,
	cloneDirectory string,
	checkLevel luals.CheckLevel,
	display Display,
) *Runner {
	if display == nil {
		display = NewGitHubDisplay(nil)
	}
	return &Runner{
		runtime:          runtime,
		image:            image,
		pullPolicy:       pullPolicy,
		cloneDirectory:   cloneDirectory,
		checkLevel:       checkLevel,
		display:          display,
		workingDirectory: os.Getwd,
	}
}

func (runner *Runner) Run(ctx context.Context) (resultErr error) {
	if err := runner.runtime.EnsureImage(ctx, runner.image, runner.pullPolicy); err != nil {
		return fmt.Errorf("prepare tools container image %q: %w", runner.image, err)
	}

	spec := container.ContainerSpec{
		Image:      runner.image,
		Entrypoint: luaLSEntrypoint,
		Arguments: []string{
			"--check", containerCheckoutPath,
			"--check_format", "json",
			"--logpath", "./log",
			"--metapath", "./meta",
			"--checklevel", runner.checkLevel.LuaLSArgument(),
		},
		BindMounts: []container.BindMount{
			{Source: runner.cloneDirectory, Target: containerCheckoutPath, ReadOnly: true},
		},
	}
	containerID, err := runner.runtime.Start(ctx, spec)
	if err != nil {
		return fmt.Errorf("start LuaLS container: %w", err)
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		if err := runner.runtime.Remove(cleanupContext, containerID); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove LuaLS container and volumes: %w", err))
		}
	}()

	exitCode, err := runner.runtime.Wait(ctx, containerID)
	if err != nil {
		return fmt.Errorf("wait for LuaLS container: %w", err)
	}
	if err := runner.writeLogs(ctx, containerID); err != nil {
		return err
	}
	if exitCode != 0 && exitCode != 1 {
		return fmt.Errorf("LuaLS container exited with status %d", exitCode)
	}

	temporaryDirectory, err := os.MkdirTemp("", "vltest-luals-")
	if err != nil {
		return fmt.Errorf("create LuaLS report directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(temporaryDirectory); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove LuaLS report directory: %w", err))
		}
	}()
	if err := runner.runtime.CopyFrom(ctx, containerID, reportPath, temporaryDirectory); err != nil {
		return fmt.Errorf("copy LuaLS report from container: %w", err)
	}

	reportFile, err := os.Open(filepath.Join(temporaryDirectory, "check.json"))
	if err != nil {
		return fmt.Errorf("open LuaLS report: %w", err)
	}
	diagnostics, parseErr := luals.ParseReport(reportFile)
	closeErr := reportFile.Close()
	if parseErr != nil || closeErr != nil {
		return errors.Join(parseErr, closeErr)
	}

	workingDirectory, err := runner.workingDirectory()
	if err != nil {
		return fmt.Errorf("get working directory for LuaLS diagnostics: %w", err)
	}
	annotations, counts, err := ConvertDiagnostics(
		diagnostics,
		runner.cloneDirectory,
		workingDirectory,
		runner.checkLevel,
	)
	if err != nil {
		return err
	}
	for _, annotation := range annotations {
		if err := runner.display.WriteDiagnostic(annotation, workingDirectory); err != nil {
			return err
		}
	}
	if err := runner.display.WriteSummary(counts); err != nil {
		return err
	}
	if counts.Errors > 0 {
		return fmt.Errorf("LuaLS reported %d error diagnostics", counts.Errors)
	}
	return nil
}

func (runner *Runner) writeLogs(ctx context.Context, containerID string) error {
	logs, err := runner.runtime.ReadLogs(ctx, containerID)
	if err != nil {
		return fmt.Errorf("read LuaLS container logs: %w", err)
	}
	return runner.display.WriteLogs(logs)
}
