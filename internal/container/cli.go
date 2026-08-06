// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const engineProbeTimeout = 5 * time.Second

type commandRunner interface {
	Run(ctx context.Context, executable string, arguments []string, stdout, stderr io.Writer) error
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, executable string, arguments []string, stdout, stderr io.Writer) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

type pathFinder func(string) (string, error)

type CLIEngine struct {
	name       string
	executable string
	runner     commandRunner
}

func NewCLIEngine(ctx context.Context, preference string) (*CLIEngine, error) {
	return selectCLIEngine(ctx, preference, osCommandRunner{}, exec.LookPath)
}

func selectCLIEngine(ctx context.Context, preference string, runner commandRunner, find pathFinder) (*CLIEngine, error) {
	preference = strings.ToLower(strings.TrimSpace(preference))
	candidates := []string{preference}
	if preference == "auto" {
		candidates = []string{"docker", "podman"}
	}

	var failures []error
	for _, candidate := range candidates {
		executable, err := find(candidate)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s CLI not found: %w", candidate, err))
			continue
		}

		probeContext, cancel := context.WithTimeout(ctx, engineProbeTimeout)
		var stderr bytes.Buffer
		err = runner.Run(probeContext, executable, []string{"info"}, io.Discard, &stderr)
		cancel()
		if err != nil {
			failures = append(failures, commandError(candidate+" info", err, stderr.String()))
			continue
		}

		return &CLIEngine{name: candidate, executable: executable, runner: runner}, nil
	}

	if len(failures) == 0 {
		return nil, fmt.Errorf("unsupported container engine %q", preference)
	}
	return nil, fmt.Errorf("no usable container engine: %w", errors.Join(failures...))
}

func (engine *CLIEngine) EnsureImage(ctx context.Context, image string, policy PullPolicy) error {
	switch policy {
	case PullAlways:
		return engine.pull(ctx, image)
	case PullMissing:
		if engine.imageExists(ctx, image) {
			return nil
		}
		return engine.pull(ctx, image)
	case PullNever:
		if engine.imageExists(ctx, image) {
			return nil
		}
		return fmt.Errorf("image %q is not available locally and pull policy is never", image)
	default:
		return fmt.Errorf("unsupported pull policy %q", policy)
	}
}

func (engine *CLIEngine) imageExists(ctx context.Context, image string) bool {
	return engine.runner.Run(ctx, engine.executable, []string{"image", "inspect", image}, io.Discard, io.Discard) == nil
}

func (engine *CLIEngine) pull(ctx context.Context, image string) error {
	var stderr bytes.Buffer
	if err := engine.runner.Run(ctx, engine.executable, []string{"pull", image}, io.Discard, &stderr); err != nil {
		return commandError(engine.name+" pull", err, stderr.String())
	}
	return nil
}

func (engine *CLIEngine) Start(ctx context.Context, spec ContainerSpec) (string, error) {
	arguments := engine.runArguments(spec)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := engine.runner.Run(ctx, engine.executable, arguments, &stdout, &stderr); err != nil {
		return "", commandError(engine.name+" run", err, stderr.String())
	}
	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		return "", fmt.Errorf("%s run returned an empty container ID", engine.name)
	}
	return containerID, nil
}

func (engine *CLIEngine) Create(ctx context.Context, image string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	arguments := []string{"create", "--pull=never", image}
	if err := engine.runner.Run(ctx, engine.executable, arguments, &stdout, &stderr); err != nil {
		return "", commandError(engine.name+" create", err, stderr.String())
	}
	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		return "", fmt.Errorf("%s create returned an empty container ID", engine.name)
	}
	return containerID, nil
}

func (engine *CLIEngine) CopyFrom(
	ctx context.Context,
	containerID string,
	sourcePath string,
	destinationPath string,
) error {
	var stderr bytes.Buffer
	source := containerID + ":" + sourcePath
	if err := engine.runner.Run(ctx, engine.executable, []string{"cp", source, destinationPath}, io.Discard, &stderr); err != nil {
		return commandError(engine.name+" cp", err, stderr.String())
	}
	return nil
}

func (engine *CLIEngine) runArguments(spec ContainerSpec) []string {
	arguments := []string{"run", "--detach", "--pull=never"}
	if engine.name == "podman" && len(spec.BindMounts) > 0 {
		arguments = append(arguments, "--security-opt", "label=disable")
	}
	for _, target := range spec.AnonymousVolumes {
		arguments = append(arguments, "--mount", "type=volume,dst="+target)
	}
	for _, mount := range spec.BindMounts {
		value := "type=bind,src=" + mount.Source + ",dst=" + mount.Target
		if mount.ReadOnly {
			value += ",readonly"
		}
		arguments = append(arguments, "--mount", value)
	}
	if spec.Entrypoint != "" {
		arguments = append(arguments, "--entrypoint", spec.Entrypoint)
	}
	arguments = append(arguments, spec.Image)
	arguments = append(arguments, spec.Arguments...)
	return arguments
}

func (engine *CLIEngine) ReadLogs(ctx context.Context, containerID string) ([]byte, error) {
	var output bytes.Buffer
	if err := engine.runner.Run(ctx, engine.executable, []string{"logs", containerID}, &output, &output); err != nil {
		return nil, commandError(engine.name+" logs", err, output.String())
	}
	return output.Bytes(), nil
}

func (engine *CLIEngine) IsRunning(ctx context.Context, containerID string) (bool, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	arguments := []string{"inspect", "--format", "{{.State.Running}}", containerID}
	if err := engine.runner.Run(ctx, engine.executable, arguments, &stdout, &stderr); err != nil {
		return false, commandError(engine.name+" inspect", err, stderr.String())
	}
	running, err := strconv.ParseBool(strings.TrimSpace(stdout.String()))
	if err != nil {
		return false, fmt.Errorf("parse %s container running state %q: %w", engine.name, strings.TrimSpace(stdout.String()), err)
	}
	return running, nil
}

func (engine *CLIEngine) Wait(ctx context.Context, containerID string) (int, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := engine.runner.Run(ctx, engine.executable, []string{"wait", containerID}, &stdout, &stderr); err != nil {
		return 0, commandError(engine.name+" wait", err, stderr.String())
	}
	exitCodeText := strings.TrimSpace(stdout.String())
	exitCode, err := strconv.Atoi(exitCodeText)
	if err != nil {
		return 0, fmt.Errorf("parse %s container exit code %q: %w", engine.name, exitCodeText, err)
	}
	return exitCode, nil
}

func (engine *CLIEngine) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	seconds := int((timeout + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	var stderr bytes.Buffer
	arguments := []string{"stop", "--time", strconv.Itoa(seconds), containerID}
	if err := engine.runner.Run(ctx, engine.executable, arguments, io.Discard, &stderr); err != nil {
		return commandError(engine.name+" stop", err, stderr.String())
	}
	return nil
}

func (engine *CLIEngine) Remove(ctx context.Context, containerID string) error {
	var stderr bytes.Buffer
	arguments := []string{"rm", "--force", "--volumes", containerID}
	if err := engine.runner.Run(ctx, engine.executable, arguments, io.Discard, &stderr); err != nil {
		return commandError(engine.name+" rm", err, stderr.String())
	}
	return nil
}

func commandError(operation string, err error, stderr string) error {
	message := strings.TrimSpace(stderr)
	if message == "" {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: %w: %s", operation, err, message)
}
