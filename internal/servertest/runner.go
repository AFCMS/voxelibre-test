// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
)

const (
	startupTimeout = 15 * time.Second
	cleanupTimeout = 10 * time.Second
	stopTimeout    = 5 * time.Second
	pollInterval   = 250 * time.Millisecond
	readinessText  = `Server for gameid="voxelibre" listening on`
)

type Suite struct {
	engine            container.Engine
	image             string
	pullPolicy        container.PullPolicy
	voxeLibreCloneDir string
	output            io.Writer
	versions          []luanti.ServerVersion
	after             func(time.Duration) <-chan time.Time
	pollAfter         func(time.Duration) <-chan time.Time
}

func NewSuite(
	engine container.Engine,
	image string,
	pullPolicy container.PullPolicy,
	voxeLibreCloneDir string,
	output io.Writer,
) *Suite {
	if output == nil {
		output = io.Discard
	}
	return &Suite{
		engine:            engine,
		image:             image,
		pullPolicy:        pullPolicy,
		voxeLibreCloneDir: voxeLibreCloneDir,
		output:            output,
		versions:          luanti.SupportedServerVersions(),
		after:             time.After,
		pollAfter:         time.After,
	}
}

func (suite *Suite) Run(ctx context.Context) error {
	if err := suite.engine.EnsureImage(ctx, suite.image, suite.pullPolicy); err != nil {
		return fmt.Errorf("prepare container image %q: %w", suite.image, err)
	}

	var failures []error
	completed := 0
	for _, version := range suite.versions {
		if err := ctx.Err(); err != nil {
			failures = append(failures, fmt.Errorf("startup suite canceled: %w", err))
			break
		}

		fmt.Fprintf(suite.output, "START Luanti %s\n", version.Version)
		err := suite.runVersion(ctx, version)
		completed++
		if err != nil {
			fmt.Fprintf(suite.output, "FAIL  Luanti %s: %v\n", version.Version, err)
			failures = append(failures, fmt.Errorf("Luanti %s: %w", version.Version, err))
			continue
		}
		fmt.Fprintf(suite.output, "PASS  Luanti %s\n", version.Version)
	}

	if len(failures) > 0 {
		return fmt.Errorf("%d of %d completed server startup tests failed: %w", len(failures), completed, errors.Join(failures...))
	}
	fmt.Fprintf(suite.output, "PASS  all %d server startup tests\n", completed)
	return nil
}

func (suite *Suite) runVersion(ctx context.Context, version luanti.ServerVersion) (resultErr error) {
	spec := container.ContainerSpec{
		Image:      suite.image,
		Entrypoint: version.Entrypoint,
		Arguments: []string{
			"--config", "/etc/minetest/minetest.conf",
			"--world", "/var/lib/minetest/worlds/startup-test",
			"--gameid", "voxelibre",
			"--logfile", "/dev/null",
		},
		AnonymousVolumes: []string{"/var/lib/minetest"},
		BindMounts: []container.BindMount{
			{
				Source:   suite.voxeLibreCloneDir,
				Target:   "/var/lib/minetest/games/voxelibre",
				ReadOnly: true,
			},
		},
	}

	startupContext, cancelStartup := context.WithCancel(ctx)
	defer cancelStartup()
	timeout := suite.after(startupTimeout)
	timedOut := make(chan struct{})
	go func() {
		select {
		case <-timeout:
			close(timedOut)
			cancelStartup()
		case <-startupContext.Done():
		}
	}()

	containerID, err := suite.engine.Start(startupContext, spec)
	if err != nil {
		if channelClosed(timedOut) {
			return fmt.Errorf("server did not become ready within %s", startupTimeout)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("start container: %w", err)
	}

	containerExited := false
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()

		var cleanupErrors []error
		if !containerExited {
			if err := suite.engine.Stop(cleanupContext, containerID, stopTimeout); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("stop container: %w", err))
			}
		}
		if err := suite.engine.Remove(cleanupContext, containerID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove container and volumes: %w", err))
		}
		resultErr = errors.Join(resultErr, errors.Join(cleanupErrors...))
	}()

	var previousLogs []byte
	for {
		logs, logsErr := suite.engine.ReadLogs(startupContext, containerID)
		if logsErr != nil {
			if channelClosed(timedOut) {
				return fmt.Errorf("server did not become ready within %s", startupTimeout)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read container logs: %w", logsErr)
		}
		if err := writeNewLogs(suite.output, previousLogs, logs); err != nil {
			return fmt.Errorf("write container logs: %w", err)
		}
		previousLogs = append(previousLogs[:0], logs...)
		if channelClosed(timedOut) {
			return fmt.Errorf("server did not become ready within %s", startupTimeout)
		}
		if bytes.Contains(logs, []byte(readinessText)) {
			return nil
		}

		running, stateErr := suite.engine.IsRunning(startupContext, containerID)
		if stateErr != nil {
			if channelClosed(timedOut) {
				return fmt.Errorf("server did not become ready within %s", startupTimeout)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("inspect container state: %w", stateErr)
		}
		if !running {
			exitCode, waitErr := suite.engine.Wait(startupContext, containerID)
			containerExited = waitErr == nil
			if waitErr != nil {
				return fmt.Errorf("wait for container: %w", waitErr)
			}
			return fmt.Errorf("container exited with status %d before becoming ready", exitCode)
		}

		select {
		case <-timedOut:
			return fmt.Errorf("server did not become ready within %s", startupTimeout)
		case <-suite.pollAfter(pollInterval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func channelClosed(channel <-chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
}

func writeNewLogs(output io.Writer, previous, current []byte) error {
	newLogs := current
	if bytes.HasPrefix(current, previous) {
		newLogs = current[len(previous):]
	}
	if len(newLogs) == 0 {
		return nil
	}
	_, err := output.Write(newLogs)
	return err
}
