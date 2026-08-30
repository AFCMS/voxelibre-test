// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package lint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
)

type fakeRuntime struct {
	ensuredImage string
	pullPolicy   container.PullPolicy
	startSpec    container.ContainerSpec
	waitCode     int
	logs         []byte
	report       string
	removed      bool

	ensureErr error
	startErr  error
	waitErr   error
	logsErr   error
	copyErr   error
	removeErr error
}

func (runtime *fakeRuntime) EnsureImage(_ context.Context, image string, policy container.PullPolicy) error {
	runtime.ensuredImage = image
	runtime.pullPolicy = policy
	return runtime.ensureErr
}

func (runtime *fakeRuntime) Start(_ context.Context, spec container.ContainerSpec) (string, error) {
	runtime.startSpec = spec
	return "lint-container", runtime.startErr
}

func (runtime *fakeRuntime) ReadLogs(context.Context, string) ([]byte, error) {
	return runtime.logs, runtime.logsErr
}

func (runtime *fakeRuntime) Wait(context.Context, string) (int, error) {
	return runtime.waitCode, runtime.waitErr
}

func (runtime *fakeRuntime) CopyFrom(_ context.Context, _, source, destination string) error {
	if runtime.copyErr != nil {
		return runtime.copyErr
	}
	if source != reportPath {
		return errors.New("unexpected report source")
	}
	return os.WriteFile(filepath.Join(destination, "check.json"), []byte(runtime.report), 0o600)
}

func (runtime *fakeRuntime) Remove(context.Context, string) error {
	runtime.removed = true
	return runtime.removeErr
}

func TestRunnerRunsLuaLSAndAllowsWarnings(t *testing.T) {
	runtime := &fakeRuntime{
		waitCode: 1,
		logs:     []byte("Diagnosis complete, 1 problem found"),
		report: `{
  "file:///path/to/voxelibre/mods/example/init.lua": [{
    "code": "undefined-global",
    "message": "Undefined global ` + "`example`" + `.",
    "range": {"start":{"line":4,"character":2},"end":{"line":4,"character":9}},
    "severity": 2,
    "source": "Lua Diagnostics."
  }]
}`,
	}
	var output bytes.Buffer
	runner := NewRunner(
		runtime,
		"tools:local",
		container.PullNever,
		"/workspace/VoxeLibre",
		luals.CheckLevelWarning,
		&output,
	)
	runner.workingDirectory = func() (string, error) { return "/workspace", nil }

	if err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runtime.ensuredImage != "tools:local" || runtime.pullPolicy != container.PullNever {
		t.Fatalf("image setup = %q/%q", runtime.ensuredImage, runtime.pullPolicy)
	}
	wantArguments := []string{
		"--check", "/path/to/voxelibre",
		"--check_format", "json",
		"--logpath", "./log",
		"--metapath", "./meta",
		"--checklevel", "Warning",
	}
	if runtime.startSpec.Image != "tools:local" || runtime.startSpec.Entrypoint != luaLSEntrypoint ||
		!reflect.DeepEqual(runtime.startSpec.Arguments, wantArguments) {
		t.Fatalf("start spec = %#v", runtime.startSpec)
	}
	wantMounts := []container.BindMount{{
		Source: "/workspace/VoxeLibre", Target: "/path/to/voxelibre", ReadOnly: true,
	}}
	if !reflect.DeepEqual(runtime.startSpec.BindMounts, wantMounts) {
		t.Fatalf("mounts = %#v", runtime.startSpec.BindMounts)
	}
	if !runtime.removed {
		t.Fatal("container was not removed")
	}
	for _, text := range []string{
		"::group::LuaLS logs\n",
		"Diagnosis complete, 1 problem found\n::endgroup::\n",
		"::warning file=VoxeLibre/mods/example/init.lua,line=5,endLine=5,col=3,endColumn=10,title=LuaLS Warning%3A undefined-global::Undefined global `example`.",
		"LuaLS: 0 errors, 1 warnings, 0 information, 0 hints",
	} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("output does not contain %q: %q", text, output.String())
		}
	}
}

func TestRunnerFailsOnlyForErrorDiagnostics(t *testing.T) {
	runtime := &fakeRuntime{
		waitCode: 1,
		report: `{"file:///path/to/voxelibre/a.lua":[{
  "code":"syntax-error","message":"Unexpected token.","severity":1,
  "range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}
}]}`,
	}
	var output bytes.Buffer
	runner := NewRunner(runtime, "tools", container.PullMissing, "/workspace/VoxeLibre", luals.CheckLevelError, &output)
	runner.workingDirectory = func() (string, error) { return "/workspace", nil }

	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "reported 1 error diagnostics") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(output.String(), "::error file=VoxeLibre/a.lua") {
		t.Fatalf("output = %q", output.String())
	}
	if !runtime.removed {
		t.Fatal("container was not removed")
	}
}

func TestRunnerAcceptsEmptyReport(t *testing.T) {
	runtime := &fakeRuntime{report: `{}`}
	var output bytes.Buffer
	runner := NewRunner(runtime, "tools", container.PullMissing, "/workspace/VoxeLibre", luals.CheckLevelWarning, &output)
	runner.workingDirectory = func() (string, error) { return "/workspace", nil }
	if err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "LuaLS: 0 errors, 0 warnings, 0 information, 0 hints") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunnerReportsOperationalAndCleanupFailures(t *testing.T) {
	t.Run("unexpected exit", func(t *testing.T) {
		runtime := &fakeRuntime{waitCode: 2}
		runner := NewRunner(runtime, "tools", container.PullMissing, "/clone", luals.CheckLevelWarning, nil)
		err := runner.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "status 2") {
			t.Fatalf("error = %v", err)
		}
		if !runtime.removed {
			t.Fatal("container was not removed")
		}
	})

	t.Run("malformed report and cleanup", func(t *testing.T) {
		runtime := &fakeRuntime{report: `null`, removeErr: errors.New("remove failed")}
		runner := NewRunner(runtime, "tools", container.PullMissing, "/clone", luals.CheckLevelWarning, nil)
		err := runner.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "expected a JSON object") || !strings.Contains(err.Error(), "remove failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("copy report", func(t *testing.T) {
		runtime := &fakeRuntime{copyErr: errors.New("copy failed")}
		runner := NewRunner(runtime, "tools", container.PullMissing, "/clone", luals.CheckLevelWarning, nil)
		err := runner.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "copy failed") {
			t.Fatalf("error = %v", err)
		}
		if !runtime.removed {
			t.Fatal("container was not removed")
		}
	})
}

func TestRunnerHonorsCancellationWhileWaiting(t *testing.T) {
	runtime := &fakeRuntime{waitErr: context.Canceled}
	runner := NewRunner(runtime, "tools", container.PullMissing, "/clone", luals.CheckLevelWarning, nil)
	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("error = %v", err)
	}
	if !runtime.removed {
		t.Fatal("container was not removed")
	}
}
