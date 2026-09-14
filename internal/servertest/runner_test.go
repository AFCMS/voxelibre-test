// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
)

type fakeEngine struct {
	mu sync.Mutex

	ensureCalls int
	startSpecs  []container.ContainerSpec
	stopIDs     []string
	removeIDs   []string

	ensureErr   error
	stopErr     error
	removeErr   error
	startHook   func(context.Context, container.ContainerSpec, int) (string, error)
	logsHook    func(context.Context, string) ([]byte, error)
	runningHook func(context.Context, string) (bool, error)
	waitHook    func(context.Context, string) (int, error)
}

func (engine *fakeEngine) EnsureImage(context.Context, string, container.PullPolicy) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.ensureCalls++
	return engine.ensureErr
}

func (engine *fakeEngine) Start(ctx context.Context, spec container.ContainerSpec) (string, error) {
	engine.mu.Lock()
	index := len(engine.startSpecs)
	engine.startSpecs = append(engine.startSpecs, spec)
	engine.mu.Unlock()
	if engine.startHook != nil {
		return engine.startHook(ctx, spec, index)
	}
	return "container-" + spec.Entrypoint, nil
}

func (engine *fakeEngine) ReadLogs(ctx context.Context, id string) ([]byte, error) {
	if engine.logsHook != nil {
		return engine.logsHook(ctx, id)
	}
	return []byte(passingLogs), nil
}

func (engine *fakeEngine) IsRunning(ctx context.Context, id string) (bool, error) {
	if engine.runningHook != nil {
		return engine.runningHook(ctx, id)
	}
	return false, nil
}

func (engine *fakeEngine) Wait(ctx context.Context, id string) (int, error) {
	if engine.waitHook != nil {
		return engine.waitHook(ctx, id)
	}
	return 0, nil
}

func (engine *fakeEngine) Stop(_ context.Context, id string, _ time.Duration) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.stopIDs = append(engine.stopIDs, id)
	return engine.stopErr
}

func (engine *fakeEngine) Remove(_ context.Context, id string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.removeIDs = append(engine.removeIDs, id)
	return engine.removeErr
}

func TestSuiteStartsEverySupportedVersion(t *testing.T) {
	engine := &fakeEngine{}
	output := &lockedBuffer{}
	suite := NewSuite(engine, "image:local", container.PullNever, "/host/VoxeLibre", output)
	wantTests := len(luanti.SupportedServerVersions())

	if err := suite.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if engine.ensureCalls != 1 {
		t.Fatalf("EnsureImage calls = %d, want 1", engine.ensureCalls)
	}
	if len(engine.startSpecs) != wantTests || len(engine.stopIDs) != 0 || len(engine.removeIDs) != wantTests {
		t.Fatalf("lifecycle counts: starts=%d stops=%d removes=%d", len(engine.startSpecs), len(engine.stopIDs), len(engine.removeIDs))
	}
	for _, spec := range engine.startSpecs {
		if spec.Image != "image:local" {
			t.Fatalf("image = %q", spec.Image)
		}
		if len(spec.AnonymousVolumes) != 1 || spec.AnonymousVolumes[0] != "/var/lib/minetest" {
			t.Fatalf("anonymous volumes = %#v", spec.AnonymousVolumes)
		}
		if len(spec.BindMounts) != 1 || spec.BindMounts[0].Source != "/host/VoxeLibre" || !spec.BindMounts[0].ReadOnly {
			t.Fatalf("bind mounts = %#v", spec.BindMounts)
		}
		arguments := strings.Join(spec.Arguments, " ")
		for _, expected := range []string{"--config /etc/minetest/unittests.conf", "--gameid voxelibre", "--logfile /dev/null", "--world /var/lib/minetest/worlds/unit-tests"} {
			if !strings.Contains(arguments, expected) {
				t.Fatalf("arguments %q do not contain %q", arguments, expected)
			}
		}
	}
	if !strings.Contains(output.String(), fmt.Sprintf("PASS  all %d server unit-test suites", wantTests)) {
		t.Fatalf("output = %q", output.String())
	}
	if count := strings.Count(output.String(), "::group::Luanti "); count != wantTests {
		t.Fatalf("log group starts = %d, want %d; output = %q", count, wantTests, output.String())
	}
	if count := strings.Count(output.String(), "::endgroup::"); count != wantTests {
		t.Fatalf("log group ends = %d, want %d; output = %q", count, wantTests, output.String())
	}
	for _, version := range suite.versions {
		group := "::group::Luanti " + version.Version + " server logs\n"
		if !strings.Contains(output.String(), group) {
			t.Fatalf("output does not contain %q: %q", group, output.String())
		}
	}
}

func TestSuiteFailsOnEarlyExitWithoutStoppingExitedContainer(t *testing.T) {
	engine := &fakeEngine{
		logsHook:    func(context.Context, string) ([]byte, error) { return nil, nil },
		runningHook: func(context.Context, string) (bool, error) { return false, nil },
		waitHook:    func(context.Context, string) (int, error) { return 42, nil },
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(failureDetails(err), "exited with status 42") {
		t.Fatalf("got %v, want early-exit error", err)
	}
	if len(engine.stopIDs) != 0 || len(engine.removeIDs) != 1 {
		t.Fatalf("cleanup: stops=%d removes=%d", len(engine.stopIDs), len(engine.removeIDs))
	}
}

func TestSuiteUsesHardcodedSuiteTimeout(t *testing.T) {
	engine := &fakeEngine{
		logsHook: func(ctx context.Context, _ string) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() },
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]
	suite.after = func(duration time.Duration) <-chan time.Time {
		if duration != 60*time.Second {
			t.Fatalf("timeout = %s, want 60s", duration)
		}
		channel := make(chan time.Time, 1)
		channel <- time.Now()
		return channel
	}

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(failureDetails(err), "within 1m0s") {
		t.Fatalf("got %v, want timeout error", err)
	}
	if len(engine.stopIDs) != 1 || len(engine.removeIDs) != 1 {
		t.Fatalf("cleanup: stops=%d removes=%d", len(engine.stopIDs), len(engine.removeIDs))
	}
}

func TestSuiteTimeoutIncludesContainerStart(t *testing.T) {
	engine := &fakeEngine{
		startHook: func(ctx context.Context, _ container.ContainerSpec, _ int) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]
	suite.after = func(duration time.Duration) <-chan time.Time {
		if duration != 60*time.Second {
			t.Fatalf("timeout = %s, want 60s", duration)
		}
		channel := make(chan time.Time, 1)
		channel <- time.Now()
		return channel
	}

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(failureDetails(err), "within 1m0s") {
		t.Fatalf("got %v, want timeout error", err)
	}
	if len(engine.stopIDs) != 0 || len(engine.removeIDs) != 0 {
		t.Fatalf("cleanup without a container ID: stops=%d removes=%d", len(engine.stopIDs), len(engine.removeIDs))
	}
}

func TestSuiteContinuesAfterVersionFailure(t *testing.T) {
	engine := &fakeEngine{
		startHook: func(_ context.Context, spec container.ContainerSpec, index int) (string, error) {
			if index == 1 {
				return "", errors.New("start failed")
			}
			return "container-" + spec.Entrypoint, nil
		},
	}
	output := &lockedBuffer{}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", output)
	wantTests := len(luanti.SupportedServerVersions())

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("1 of %d completed server unit-test suites failed", wantTests)) {
		t.Fatalf("got %v, want aggregated failure", err)
	}
	if len(engine.startSpecs) != wantTests {
		t.Fatalf("started %d versions, want %d", len(engine.startSpecs), wantTests)
	}
	annotation := "FAIL  Luanti 5.15.2: start container: start failed\n"
	if count := strings.Count(output.String(), annotation); count != 1 {
		t.Fatalf("failure messages = %d, want 1; output = %q", count, output.String())
	}
	if count := strings.Count(output.String(), "::endgroup::"); count != wantTests {
		t.Fatalf("log group ends = %d, want %d; output = %q", count, wantTests, output.String())
	}
}

func TestWorkflowLogGroupClosesOnNewLine(t *testing.T) {
	engine := &fakeEngine{
		logsHook: func(context.Context, string) ([]byte, error) {
			return []byte(passingLogs + "shutdown"), nil
		},
	}
	output := &lockedBuffer{}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", output)
	suite.versions = suite.versions[:1]

	if err := suite.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "shutdown\n::endgroup::\n") {
		t.Fatalf("endgroup marker is not on its own line: %q", output.String())
	}
}

func TestSuiteReportsLogAndCleanupFailures(t *testing.T) {
	engine := &fakeEngine{
		logsHook: func(context.Context, string) ([]byte, error) {
			return nil, errors.New("logs unavailable")
		},
		removeErr: errors.New("remove failed"),
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(failureDetails(err), "logs unavailable") || !strings.Contains(failureDetails(err), "remove failed") {
		t.Fatalf("got %v, want log and cleanup errors", err)
	}
}

func TestSuiteReadsFinalResultsAfterExit(t *testing.T) {
	call := 0
	engine := &fakeEngine{
		logsHook: func(context.Context, string) ([]byte, error) {
			call++
			if call == 1 {
				return []byte(passingLogs[:30]), nil
			}
			return []byte(passingLogs), nil
		},
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]
	suite.pollAfter = func(time.Duration) <-chan time.Time {
		channel := make(chan time.Time, 1)
		channel <- time.Now()
		return channel
	}

	if err := suite.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if call != 2 {
		t.Fatalf("log snapshots = %d, want 2", call)
	}
}

func TestSuiteHonorsCancellation(t *testing.T) {
	engine := &fakeEngine{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)

	err := suite.Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("got %v, want cancellation error", err)
	}
	if len(engine.startSpecs) != 0 {
		t.Fatalf("started %d containers after cancellation", len(engine.startSpecs))
	}
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *lockedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(data)
}

func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func TestSuiteReportsFailuresOnceWithoutAnnotations(t *testing.T) {
	logs := strings.Replace(passingTest, `"status":"pass"`, `"status":"fail","traceback":"failed 100%\ntrace"`, 1) +
		strings.NewReplacer(`"passed":1`, `"passed":0`, `"failed":0`, `"failed":1`).Replace(passingSummary)
	engine := &fakeEngine{logsHook: func(context.Context, string) ([]byte, error) { return []byte(logs), nil }}
	var output bytes.Buffer
	suite := NewSuite(engine, "image", container.PullNever, "/clone", &output)
	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(failureDetails(err), "1 of 1 Lua unit tests failed") {
		t.Fatalf("expected assertion failure, got %v", err)
	}
	if len(engine.startSpecs) != len(suite.versions) {
		t.Fatal("did not continue across versions")
	}
	if count := strings.Count(output.String(), "failed 100%\n        trace\n"); count != len(suite.versions) {
		t.Fatalf("failure details printed %d times, want once per version: %s", count, &output)
	}
	for _, unwanted := range []string{"::error", "::warning", "%0A", "%25", resultPrefix} {
		if strings.Contains(output.String(), unwanted) {
			t.Fatalf("unexpected %q in output: %s", unwanted, &output)
		}
	}
	if strings.Contains(err.Error(), "\n") || strings.Contains(err.Error(), "Lua unit tests") {
		t.Fatalf("final error repeats failure details: %v", err)
	}
}

func TestSuiteDoesNotPassBeforeServerExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine := &fakeEngine{runningHook: func(context.Context, string) (bool, error) { cancel(); return true, nil }}
	suite := NewSuite(engine, "image", container.PullNever, "/clone", io.Discard)
	err := suite.Run(ctx)
	if !errors.Is(err, context.Canceled) || len(engine.stopIDs) != 1 || len(engine.removeIDs) != 1 {
		t.Fatalf("err=%v stops=%v removes=%v", err, engine.stopIDs, engine.removeIDs)
	}
}

func TestSuiteRejectsNonzeroExitEvenWithPassingSummary(t *testing.T) {
	engine := &fakeEngine{waitHook: func(context.Context, string) (int, error) { return 1, nil }}
	suite := NewSuite(engine, "image", container.PullNever, "/clone", io.Discard)
	suite.versions = suite.versions[:1]
	if err := suite.Run(context.Background()); err == nil || !strings.Contains(failureDetails(err), "status 1") {
		t.Fatalf("expected failed exit, got %v", err)
	}
}

func TestSuiteReportsInfrastructureFailures(t *testing.T) {
	for _, operation := range []string{"ensure", "inspect", "wait", "final logs", "stop", "remove"} {
		t.Run(operation, func(t *testing.T) {
			failure := errors.New(operation + " failed")
			engine := &fakeEngine{}
			switch operation {
			case "ensure":
				engine.ensureErr = failure
			case "inspect":
				engine.runningHook = func(context.Context, string) (bool, error) { return false, failure }
			case "wait":
				engine.waitHook = func(context.Context, string) (int, error) { return 0, failure }
			case "final logs":
				calls := 0
				engine.logsHook = func(context.Context, string) ([]byte, error) {
					calls++
					if calls == 2 {
						return nil, failure
					}
					return []byte(passingLogs), nil
				}
			case "stop":
				engine.logsHook = func(context.Context, string) ([]byte, error) { return nil, errors.New("read failed") }
				engine.stopErr = failure
			case "remove":
				engine.removeErr = failure
			}
			suite := NewSuite(engine, "image", container.PullNever, "/clone", io.Discard)
			suite.versions = suite.versions[:1]
			if err := suite.Run(context.Background()); !errors.Is(err, failure) {
				t.Fatalf("expected %v, got %v", failure, err)
			}
			if operation != "ensure" && len(engine.removeIDs) != 1 {
				t.Fatal("container not removed")
			}
		})
	}
}

// The CLI prints only the compact Error message; detailed causes remain available.
func failureDetails(err error) string {
	if cause := errors.Unwrap(err); cause != nil {
		return cause.Error()
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func TestSuiteFlushesPartialEngineLogOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine := &fakeEngine{
		logsHook:    func(context.Context, string) ([]byte, error) { return []byte("ERROR: final engine diagnostic"), nil },
		runningHook: func(context.Context, string) (bool, error) { cancel(); return true, nil },
	}
	var output bytes.Buffer
	suite := NewSuite(engine, "image", container.PullNever, "/clone", &output)
	err := suite.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(output.String(), "ERROR: final engine diagnostic\n::endgroup::") {
		t.Fatalf("lost final engine log: %s", &output)
	}
	if strings.Contains(err.Error(), "2 of 1") {
		t.Fatalf("cancellation counted as an extra failed version: %v", err)
	}
	if len(engine.stopIDs) != 1 || len(engine.removeIDs) != 1 {
		t.Fatal("canceled container not cleaned up")
	}
}

type logWriteFailure struct{ calls int }

func (writer *logWriteFailure) Write(data []byte) (int, error) {
	writer.calls++
	if writer.calls == 1 {
		return len(data), nil
	} // Allow the group header.
	return 0, io.ErrClosedPipe
}

func TestSuiteCleansUpWhenOutputFails(t *testing.T) {
	engine := &fakeEngine{}
	suite := NewSuite(engine, "image", container.PullNever, "/clone", &logWriteFailure{})
	err := suite.Run(context.Background())
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("err=%v", err)
	}
	if len(engine.stopIDs) != 1 || len(engine.removeIDs) != 1 {
		t.Fatal("container not cleaned up after output failure")
	}
}
