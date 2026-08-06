// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package container

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

type runnerFunc func(context.Context, string, []string, io.Writer, io.Writer) error

func (function runnerFunc) Run(
	ctx context.Context,
	executable string,
	arguments []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return function(ctx, executable, arguments, stdout, stderr)
}

func TestParsePullPolicy(t *testing.T) {
	for _, value := range []string{"always", "missing", "never", " MISSING "} {
		if _, err := ParsePullPolicy(value); err != nil {
			t.Fatalf("ParsePullPolicy(%q): %v", value, err)
		}
	}
	if _, err := ParsePullPolicy("sometimes"); err == nil {
		t.Fatal("expected invalid policy error")
	}
}

func TestRunArguments(t *testing.T) {
	spec := ContainerSpec{
		Image:      "example:local",
		Entrypoint: "/bin/server",
		Arguments:  []string{"--gameid", "voxelibre"},
		AnonymousVolumes: []string{
			"/var/lib/minetest",
		},
		BindMounts: []BindMount{
			{Source: "/a path/VoxeLibre", Target: "/games/voxelibre", ReadOnly: true},
		},
	}

	docker := &CLIEngine{name: "docker"}
	wantDocker := []string{
		"run", "--detach", "--pull=never",
		"--mount", "type=volume,dst=/var/lib/minetest",
		"--mount", "type=bind,src=/a path/VoxeLibre,dst=/games/voxelibre,readonly",
		"--entrypoint", "/bin/server",
		"example:local", "--gameid", "voxelibre",
	}
	if got := docker.runArguments(spec); !reflect.DeepEqual(got, wantDocker) {
		t.Fatalf("docker arguments:\n got: %#v\nwant: %#v", got, wantDocker)
	}

	podman := &CLIEngine{name: "podman"}
	wantPodman := append([]string{"run", "--detach", "--pull=never", "--security-opt", "label=disable"}, wantDocker[3:]...)
	if got := podman.runArguments(spec); !reflect.DeepEqual(got, wantPodman) {
		t.Fatalf("podman arguments:\n got: %#v\nwant: %#v", got, wantPodman)
	}
}

func TestEnsureImagePolicies(t *testing.T) {
	tests := []struct {
		name          string
		policy        PullPolicy
		imagePresent  bool
		wantCalls     []string
		wantErrorText string
	}{
		{name: "always", policy: PullAlways, wantCalls: []string{"pull"}},
		{name: "missing present", policy: PullMissing, imagePresent: true, wantCalls: []string{"image inspect"}},
		{name: "missing absent", policy: PullMissing, wantCalls: []string{"image inspect", "pull"}},
		{name: "never present", policy: PullNever, imagePresent: true, wantCalls: []string{"image inspect"}},
		{name: "never absent", policy: PullNever, wantCalls: []string{"image inspect"}, wantErrorText: "not available locally"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			runner := runnerFunc(func(_ context.Context, _ string, arguments []string, _, _ io.Writer) error {
				operation := arguments[0]
				if operation == "image" {
					operation = "image inspect"
				}
				calls = append(calls, operation)
				if arguments[0] == "image" && !test.imagePresent {
					return errors.New("not found")
				}
				return nil
			})
			engine := &CLIEngine{name: "docker", executable: "docker", runner: runner}
			err := engine.EnsureImage(context.Background(), "example:tag", test.policy)
			if test.wantErrorText == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErrorText != "" && (err == nil || !strings.Contains(err.Error(), test.wantErrorText)) {
				t.Fatalf("got error %v, want text %q", err, test.wantErrorText)
			}
			if !reflect.DeepEqual(calls, test.wantCalls) {
				t.Fatalf("calls: got %#v, want %#v", calls, test.wantCalls)
			}
		})
	}
}

func TestSelectCLIEngineFallsBackToPodman(t *testing.T) {
	runner := runnerFunc(func(_ context.Context, executable string, arguments []string, _, stderr io.Writer) error {
		if executable == "/usr/bin/docker" && reflect.DeepEqual(arguments, []string{"info"}) {
			_, _ = io.WriteString(stderr, "daemon unavailable")
			return errors.New("exit status 1")
		}
		return nil
	})
	finder := func(name string) (string, error) { return "/usr/bin/" + name, nil }

	engine, err := selectCLIEngine(context.Background(), "auto", runner, finder)
	if err != nil {
		t.Fatal(err)
	}
	if engine.name != "podman" {
		t.Fatalf("selected %q, want podman", engine.name)
	}
}

func TestSelectCLIEngineReportsAllFailures(t *testing.T) {
	runner := runnerFunc(func(_ context.Context, executable string, _ []string, _, _ io.Writer) error {
		return errors.New(executable + " unavailable")
	})
	finder := func(name string) (string, error) { return name, nil }

	_, err := selectCLIEngine(context.Background(), "auto", runner, finder)
	if err == nil || !strings.Contains(err.Error(), "docker") || !strings.Contains(err.Error(), "podman") {
		t.Fatalf("expected both engine failures, got %v", err)
	}
}

func TestCLILifecycleCommands(t *testing.T) {
	var calls [][]string
	runner := runnerFunc(func(_ context.Context, _ string, arguments []string, stdout, _ io.Writer) error {
		calls = append(calls, append([]string(nil), arguments...))
		switch arguments[0] {
		case "run":
			_, _ = io.WriteString(stdout, "container-id\n")
		case "logs":
			_, _ = io.WriteString(stdout, "ready\n")
		case "inspect":
			_, _ = io.WriteString(stdout, "true\n")
		case "wait":
			_, _ = io.WriteString(stdout, "0\n")
		}
		return nil
	})
	engine := &CLIEngine{name: "docker", executable: "docker", runner: runner}

	id, err := engine.Start(context.Background(), ContainerSpec{Image: "image"})
	if err != nil || id != "container-id" {
		t.Fatalf("Start() = %q, %v", id, err)
	}
	if logs, err := engine.ReadLogs(context.Background(), id); err != nil || string(logs) != "ready\n" {
		t.Fatalf("ReadLogs() = %q, %v", logs, err)
	}
	if running, err := engine.IsRunning(context.Background(), id); err != nil || !running {
		t.Fatalf("IsRunning() = %t, %v", running, err)
	}
	if code, err := engine.Wait(context.Background(), id); err != nil || code != 0 {
		t.Fatalf("Wait() = %d, %v", code, err)
	}
	if err := engine.Stop(context.Background(), id, 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := engine.Remove(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	wantLastTwo := [][]string{
		{"stop", "--time", "2", "container-id"},
		{"rm", "--force", "--volumes", "container-id"},
	}
	if !reflect.DeepEqual(calls[len(calls)-2:], wantLastTwo) {
		t.Fatalf("cleanup calls: got %#v, want %#v", calls[len(calls)-2:], wantLastTwo)
	}
}
