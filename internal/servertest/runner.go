// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/workflowcmd"
)

const (
	suiteTimeout   = 60 * time.Second
	cleanupTimeout = 10 * time.Second
	stopTimeout    = 5 * time.Second
	pollInterval   = 250 * time.Millisecond
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
		if ctx.Err() != nil {
			break
		}

		logOutput, err := startWorkflowLogGroup(suite.output, fmt.Sprintf("Luanti %s server logs", version.Version))
		if err != nil {
			return err
		}
		results, runErr := suite.runVersion(ctx, version, logOutput)
		if endErr := logOutput.End(); endErr != nil {
			return errors.Join(runErr, endErr)
		}
		completed++
		if runErr != nil || results.Failed > 0 {
			failure := runErr
			if failure == nil {
				failure = fmt.Errorf("%d of %d Lua unit tests failed", results.Failed, len(results.Tests))
			}
			failures = append(failures, fmt.Errorf("luanti %s: %w", version.Version, failure))
			if runErr != nil {
				if err := writeTestOutput(suite.output, "FAIL  Luanti %s: %v\n", version.Version, runErr); err != nil {
					return errors.Join(failure, err)
				}
			} else {
				if err := writeTestOutput(suite.output, "FAIL  Luanti %s\n", version.Version); err != nil {
					return errors.Join(failure, err)
				}
			}
			continue
		}
		if err := writeTestOutput(suite.output, "PASS  Luanti %s\n", version.Version); err != nil {
			return err
		}
	}

	if len(failures) > 0 || ctx.Err() != nil {
		message := fmt.Sprintf("%d of %d completed server unit-test suites failed", len(failures), completed)
		if ctx.Err() != nil {
			message = "unit-test suite canceled; " + message
		}
		return &suiteFailure{message: message, cause: errors.Join(errors.Join(failures...), ctx.Err())}
	}
	return writeTestOutput(suite.output, "PASS  all %d server unit-test suites\n", completed)
}

// Details have already been printed in their version's output. Return a short
// summary for Cobra while preserving causes for errors.Is/errors.As callers.
type suiteFailure struct {
	message string
	cause   error
}

func (failure *suiteFailure) Error() string { return failure.message }
func (failure *suiteFailure) Unwrap() error { return failure.cause }

func (suite *Suite) runVersion(
	ctx context.Context,
	version luanti.ServerVersion,
	logOutput io.Writer,
) (results suiteResults, resultErr error) {
	spec := container.ContainerSpec{
		Image:      suite.image,
		Entrypoint: version.Entrypoint,
		Arguments: []string{
			"--config", "/etc/minetest/unittests.conf",
			"--world", "/var/lib/minetest/worlds/unit-tests",
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

	runContext, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	timedOut := make(chan struct{})
	timeout := suite.after(suiteTimeout)
	go func() {
		select {
		case <-timeout:
			close(timedOut)
			cancelRun()
		case <-runContext.Done():
		}
	}()
	defer func() {
		if channelClosed(timedOut) {
			resultErr = errors.Join(fmt.Errorf("unit-test suite did not finish within %s", suiteTimeout), resultErr)
		} else if ctx.Err() != nil {
			resultErr = errors.Join(ctx.Err(), resultErr)
		}
	}()

	containerID, err := suite.engine.Start(runContext, spec)
	if err != nil {
		return results, fmt.Errorf("start container: %w", err)
	}

	containerExited := false
	defer func() {
		cancelRun()
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

	presenter := &logPresenter{output: logOutput}
	defer func() { resultErr = errors.Join(resultErr, presenter.flush()) }()
	readLogs := func() error {
		logs, err := suite.engine.ReadLogs(runContext, containerID)
		if err != nil {
			return fmt.Errorf("read container logs: %w", err)
		}
		if err := presenter.snapshot(logs); err != nil {
			return fmt.Errorf("present container logs: %w", err)
		}
		return nil
	}
	for {
		if err := runContext.Err(); err != nil {
			return results, err
		}
		if err := readLogs(); err != nil {
			return results, err
		}
		running, err := suite.engine.IsRunning(runContext, containerID)
		if err != nil {
			return results, fmt.Errorf("inspect container state: %w", err)
		}
		if !running {
			exitCode, err := suite.engine.Wait(runContext, containerID)
			if err != nil {
				return results, fmt.Errorf("wait for container: %w", err)
			}
			containerExited = true
			// The server may have emitted its final records since the last poll.
			logsErr := readLogs()
			var exitErr error
			if exitCode != 0 {
				exitErr = fmt.Errorf("container exited with status %d", exitCode)
			}
			if logsErr != nil {
				return results, errors.Join(exitErr, logsErr)
			}
			if err := presenter.flush(); err != nil {
				return results, errors.Join(exitErr, err)
			}
			var parseErr error
			results, parseErr = presenter.results()
			return results, errors.Join(exitErr, parseErr)
		}
		select {
		case <-suite.pollAfter(pollInterval):
		case <-runContext.Done():
			return results, runContext.Err()
		}
	}
}

func startWorkflowLogGroup(output io.Writer, title string) (*workflowcmd.Group, error) {
	return workflowcmd.StartGroup(output, title)
}

func writeTestOutput(output io.Writer, format string, arguments ...any) error {
	if _, err := fmt.Fprintf(output, format, arguments...); err != nil {
		return fmt.Errorf("write server test output: %w", err)
	}
	return nil
}

func channelClosed(channel <-chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
}
