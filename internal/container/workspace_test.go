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
)

const forgejoInspection = `[{"Mounts":[
 {"Type":"volume","Name":"job-env","Source":"/var/lib/docker/volumes/job-env/_data","Destination":"/var/run/act"},
 {"Type":"bind","Source":"/var/run/docker.sock","Destination":"/var/run/docker.sock"},
 {"Type":"volume","Name":"job-workspace","Source":"/var/lib/docker/volumes/job-workspace/_data","Destination":"/workspace/VoxeLibre/VoxeLibre"}
]}]`

func TestWorkspaceStartAndCleanup(t *testing.T) {
	for _, name := range []string{"docker", "podman"} {
		for _, target := range []string{"/path/to/voxelibre", "/var/lib/minetest/games/voxelibre"} {
			t.Run(name+target, func(t *testing.T) {
				var calls [][]string
				engine := &CLIEngine{
					name: name, executable: name, workspaceContainer: "auto",
					detectWorkspace: func() (string, error) { return "job-container", nil },
					runner: runnerFunc(func(ctx context.Context, executable string, args []string, stdout, _ io.Writer) error {
						if executable != name {
							t.Fatalf("executable = %q", executable)
						}
						calls = append(calls, args)
						switch args[0] {
						case "container":
							if !reflect.DeepEqual(args, []string{"container", "inspect", "job-container"}) {
								t.Fatalf("inspect = %#v", args)
							}
							if _, ok := ctx.Deadline(); !ok {
								t.Fatal("inspection must be bounded")
							}
							_, _ = io.WriteString(stdout, forgejoInspection)
						case "run":
							_, _ = io.WriteString(stdout, "test-container")
						case "rm":
						default:
							t.Fatalf("unexpected operation: %#v", args)
						}
						return nil
					}),
				}
				spec := ContainerSpec{Image: "tools", BindMounts: []BindMount{{Source: "/workspace/VoxeLibre/VoxeLibre", Target: target, ReadOnly: true}}, AnonymousVolumes: []string{"/var/lib/minetest"}}
				for range 2 {
					id, err := engine.Start(context.Background(), spec)
					if err != nil {
						t.Fatal(err)
					}
					if err := engine.Remove(context.Background(), id); err != nil {
						t.Fatal(err)
					}
				}
				wantRun := []string{"run", "--detach", "--pull=never"}
				if name == "podman" {
					wantRun = append(wantRun, "--security-opt", "label=disable")
				}
				wantRun = append(wantRun, "--mount", "type=volume,dst=/var/lib/minetest", "--mount", "type=volume,src=job-workspace,dst="+target+",readonly", "tools")
				want := [][]string{{"container", "inspect", "job-container"}, wantRun, {"rm", "--force", "--volumes", "test-container"}, wantRun, {"rm", "--force", "--volumes", "test-container"}}
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("calls = %#v, want %#v", calls, want)
				}
				if spec.BindMounts[0].Source != "/workspace/VoxeLibre/VoxeLibre" {
					t.Fatal("caller spec was modified")
				}
			})
		}
	}
}

func TestWorkspaceTranslation(t *testing.T) {
	for _, test := range []struct {
		name        string
		mounts      []workspaceMount
		source      string
		wantBind    string
		wantVolume  string
		wantSubpath string
		wantError   string
	}{
		{name: "bind root", mounts: []workspaceMount{{Type: "bind", Source: "/host/game", Destination: "/workspace/game"}}, source: "/workspace/game", wantBind: "/host/game"},
		{name: "bind subdirectory", mounts: []workspaceMount{{Type: "bind", Source: "/host", Destination: "/workspace"}}, source: "/workspace/game", wantBind: "/host/game"},
		{name: "volume subdirectory", mounts: []workspaceMount{{Type: "volume", Name: "checkout", Destination: "/workspace"}}, source: "/workspace/game", wantVolume: "checkout", wantSubpath: "game"},
		{name: "volume already uses subpath", mounts: []workspaceMount{{Type: "volume", Name: "checkout", Destination: "/workspace", SubPath: "project"}}, source: "/workspace/game", wantVolume: "checkout", wantSubpath: "project/game"},
		{name: "most specific mount", mounts: []workspaceMount{{Type: "bind", Source: "/host", Destination: "/workspace"}, {Type: "volume", Name: "checkout", Destination: "/workspace/game"}}, source: "/workspace/game", wantVolume: "checkout"},
		{name: "path component boundary", mounts: []workspaceMount{{Type: "volume", Name: "checkout", Destination: "/workspace/game"}}, source: "/workspace/game-other", wantError: "not inside"},
		{name: "nested mount", mounts: []workspaceMount{{Type: "volume", Name: "checkout", Destination: "/workspace"}, {Type: "bind", Source: "/host/mods", Destination: "/workspace/game/mods"}}, source: "/workspace/game", wantError: "nested mount"},
		{name: "tmpfs", mounts: []workspaceMount{{Type: "tmpfs", Destination: "/workspace"}}, source: "/workspace/game", wantError: "unsupported type"},
		{name: "missing volume name", mounts: []workspaceMount{{Type: "volume", Destination: "/workspace"}}, source: "/workspace/game", wantError: "no name"},
		{name: "invalid bind source", mounts: []workspaceMount{{Type: "bind", Source: "relative", Destination: "/workspace"}}, source: "/workspace/game", wantError: "invalid source"},
		{name: "subpath escape", mounts: []workspaceMount{{Type: "volume", Name: "checkout", Destination: "/workspace", SubPath: "../outside"}}, source: "/workspace/game", wantError: "unsupported subpath"},
		{name: "writable layer", mounts: []workspaceMount{}, source: "/workspace/game", wantError: "writable layer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := &CLIEngine{workspaceLoaded: true, workspaceMounts: test.mounts}
			spec, err := engine.resolveWorkspace(context.Background(), ContainerSpec{BindMounts: []BindMount{{Source: test.source, Target: "/game", ReadOnly: true}}})
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.wantBind != "" {
				want := []BindMount{{Source: test.wantBind, Target: "/game", ReadOnly: true}}
				if !reflect.DeepEqual(spec.BindMounts, want) {
					t.Fatalf("binds = %#v", spec.BindMounts)
				}
			} else {
				want := []VolumeMount{{Source: test.wantVolume, Target: "/game", Subpath: test.wantSubpath, ReadOnly: true}}
				if !reflect.DeepEqual(spec.VolumeMounts, want) {
					t.Fatalf("volumes = %#v", spec.VolumeMounts)
				}
			}
		})
	}
}

func TestWorkspaceDiscoveryModes(t *testing.T) {
	for _, mode := range []string{"auto", "none", "custom-job"} {
		t.Run(mode, func(t *testing.T) {
			inspects := 0
			engine := &CLIEngine{workspaceContainer: mode, detectWorkspace: func() (string, error) {
				if mode != "auto" {
					t.Fatal("explicit mode tried discovery")
				}
				return "", nil // Ordinary host execution.
			}, runner: runnerFunc(func(_ context.Context, _ string, args []string, stdout, _ io.Writer) error {
				inspects++
				if args[2] != "custom-job" {
					t.Fatalf("inspect = %#v", args)
				}
				_, _ = io.WriteString(stdout, forgejoInspection)
				return nil
			})}
			spec := ContainerSpec{BindMounts: []BindMount{{Source: "/workspace/VoxeLibre/VoxeLibre", Target: "/game", ReadOnly: true}}}
			got, err := engine.resolveWorkspace(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "custom-job" {
				if inspects != 1 || len(got.VolumeMounts) != 1 {
					t.Fatalf("inspection count %d, spec %#v", inspects, got)
				}
			} else if inspects != 0 || !reflect.DeepEqual(spec, got) {
				t.Fatalf("host mount changed: %#v", got)
			}
		})
	}
}

func TestWorkspaceInspectionErrorsPreventStart(t *testing.T) {
	for _, test := range []struct {
		name, output, want string
		err                error
	}{
		{"unavailable", "", "--workspace-container", errors.New("no such container")},
		{"canceled", "", "context canceled", context.Canceled},
		{"bad JSON", "oops", "decode workspace", nil},
		{"no result", "[]", "expected one", nil},
		{"bad destination", `[{"Mounts":[{"Destination":"relative"}]}]`, "invalid destination", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := &CLIEngine{workspaceContainer: "job", runner: runnerFunc(func(_ context.Context, _ string, args []string, stdout, _ io.Writer) error {
				if args[0] != "container" {
					t.Fatal("started container after discovery failure")
				}
				_, _ = io.WriteString(stdout, test.output)
				return test.err
			})}
			_, err := engine.Start(context.Background(), ContainerSpec{BindMounts: []BindMount{{Source: "/game", Target: "/game"}}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
			if test.err != nil && !errors.Is(err, test.err) {
				t.Fatalf("lost underlying error: %v", err)
			}
			if engine.workspaceLoaded {
				t.Fatal("failed inspection was cached")
			}
		})
	}
}

func TestWorkspaceDockerOriginalSubpath(t *testing.T) {
	engine := &CLIEngine{workspaceContainer: "job", runner: runnerFunc(func(_ context.Context, _ string, _ []string, stdout, _ io.Writer) error {
		_, _ = io.WriteString(stdout, `[{"Mounts":[{"Type":"volume","Name":"checkout","Destination":"/workspace"}],"HostConfig":{"Mounts":[{"Target":"/workspace","VolumeOptions":{"Subpath":"project"}}]}}]`)
		return nil
	})}
	spec, err := engine.resolveWorkspace(context.Background(), ContainerSpec{BindMounts: []BindMount{{Source: "/workspace/game", Target: "/game", ReadOnly: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if spec.VolumeMounts[0].Subpath != "project/game" {
		t.Fatalf("volume = %#v", spec.VolumeMounts)
	}
}

func TestVolumeSubpathArguments(t *testing.T) {
	for _, name := range []string{"docker", "podman"} {
		t.Run(name, func(t *testing.T) {
			engine := &CLIEngine{name: name}
			args := engine.runArguments(ContainerSpec{Image: "tools", VolumeMounts: []VolumeMount{{Source: "checkout", Target: "/game", Subpath: "project/game", ReadOnly: true}}})
			key := "volume-subpath"
			if name == "podman" {
				key = "subpath"
			}
			want := "type=volume,src=checkout,dst=/game," + key + "=project/game,readonly"
			if !strings.Contains(strings.Join(args, " "), want) {
				t.Fatalf("args = %#v", args)
			}
		})
	}
}

func TestExtractionSkipsWorkspaceDiscovery(t *testing.T) {
	engine := &CLIEngine{workspaceContainer: "auto", detectWorkspace: func() (string, error) { t.Fatal("unnecessary workspace discovery"); return "", nil }}
	if _, err := engine.resolveWorkspace(context.Background(), ContainerSpec{Image: "builds"}); err != nil {
		t.Fatal(err)
	}
}
