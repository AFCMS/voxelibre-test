// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
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
	return []byte("Server for gameid=\"voxelibre\" listening on [::]:30000.\n"), nil
}

func (engine *fakeEngine) IsRunning(ctx context.Context, id string) (bool, error) {
	if engine.runningHook != nil {
		return engine.runningHook(ctx, id)
	}
	return true, nil
}

func (engine *fakeEngine) Wait(ctx context.Context, id string) (int, error) {
	if engine.waitHook != nil {
		return engine.waitHook(ctx, id)
	}
	<-ctx.Done()
	return 0, ctx.Err()
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

	if err := suite.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if engine.ensureCalls != 1 {
		t.Fatalf("EnsureImage calls = %d, want 1", engine.ensureCalls)
	}
	if len(engine.startSpecs) != 3 || len(engine.stopIDs) != 3 || len(engine.removeIDs) != 3 {
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
		for _, expected := range []string{"--gameid voxelibre", "--logfile /dev/null", "--world /var/lib/minetest/worlds/startup-test"} {
			if !strings.Contains(arguments, expected) {
				t.Fatalf("arguments %q do not contain %q", arguments, expected)
			}
		}
	}
	if !strings.Contains(output.String(), "PASS  all 3 server startup tests") {
		t.Fatalf("output = %q", output.String())
	}
	if count := strings.Count(output.String(), "::group::Luanti "); count != 3 {
		t.Fatalf("log group starts = %d, want 3; output = %q", count, output.String())
	}
	if count := strings.Count(output.String(), "::endgroup::"); count != 3 {
		t.Fatalf("log group ends = %d, want 3; output = %q", count, output.String())
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
	if err == nil || !strings.Contains(err.Error(), "exited with status 42") {
		t.Fatalf("got %v, want early-exit error", err)
	}
	if len(engine.stopIDs) != 0 || len(engine.removeIDs) != 1 {
		t.Fatalf("cleanup: stops=%d removes=%d", len(engine.stopIDs), len(engine.removeIDs))
	}
}

func TestSuiteUsesHardcodedStartupTimeout(t *testing.T) {
	engine := &fakeEngine{
		logsHook: func(context.Context, string) ([]byte, error) { return nil, nil },
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]
	suite.after = func(duration time.Duration) <-chan time.Time {
		if duration != 15*time.Second {
			t.Fatalf("timeout = %s, want 15s", duration)
		}
		channel := make(chan time.Time, 1)
		channel <- time.Now()
		return channel
	}

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "within 15s") {
		t.Fatalf("got %v, want timeout error", err)
	}
	if len(engine.stopIDs) != 1 || len(engine.removeIDs) != 1 {
		t.Fatalf("cleanup: stops=%d removes=%d", len(engine.stopIDs), len(engine.removeIDs))
	}
}

func TestSuiteStartupTimeoutIncludesContainerStart(t *testing.T) {
	engine := &fakeEngine{
		startHook: func(ctx context.Context, _ container.ContainerSpec, _ int) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", io.Discard)
	suite.versions = suite.versions[:1]
	suite.after = func(duration time.Duration) <-chan time.Time {
		if duration != 15*time.Second {
			t.Fatalf("timeout = %s, want 15s", duration)
		}
		channel := make(chan time.Time, 1)
		channel <- time.Now()
		return channel
	}

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "within 15s") {
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

	err := suite.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 of 3 completed server startup tests failed") {
		t.Fatalf("got %v, want aggregated failure", err)
	}
	if len(engine.startSpecs) != 3 {
		t.Fatalf("started %d versions, want 3", len(engine.startSpecs))
	}
	annotation := "::error title=Luanti 5.15.2 startup test failed::start container: start failed\n"
	if count := strings.Count(output.String(), annotation); count != 1 {
		t.Fatalf("failure annotations = %d, want 1; output = %q", count, output.String())
	}
	if count := strings.Count(output.String(), "::endgroup::"); count != 3 {
		t.Fatalf("log group ends = %d, want 3; output = %q", count, output.String())
	}
}

func TestWorkflowLogGroupClosesOnNewLine(t *testing.T) {
	engine := &fakeEngine{
		logsHook: func(context.Context, string) ([]byte, error) {
			return []byte(readinessText), nil
		},
	}
	output := &lockedBuffer{}
	suite := NewSuite(engine, "image", container.PullMissing, "/clone", output)
	suite.versions = suite.versions[:1]

	if err := suite.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), readinessText+"\n::endgroup::\n") {
		t.Fatalf("endgroup marker is not on its own line: %q", output.String())
	}
}

func TestWorkflowErrorEscapesCommandValues(t *testing.T) {
	var output bytes.Buffer
	writeWorkflowError(&output, "Luanti: 5.16.1, client", "failed 100%\r\nretry")

	want := "::error title=Luanti%3A 5.16.1%2C client::failed 100%25%0D%0Aretry\n"
	if output.String() != want {
		t.Fatalf("workflow error = %q, want %q", output.String(), want)
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
	if err == nil || !strings.Contains(err.Error(), "logs unavailable") || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("got %v, want log and cleanup errors", err)
	}
}

func TestSuiteFindsReadinessAcrossLogSnapshots(t *testing.T) {
	call := 0
	engine := &fakeEngine{
		logsHook: func(context.Context, string) ([]byte, error) {
			call++
			if call == 1 {
				return []byte("Server for gameid=\"voxe"), nil
			}
			return []byte("Server for gameid=\"voxelibre\" listening on [::]:30000.\n"), nil
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
