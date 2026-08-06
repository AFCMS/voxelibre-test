// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
)

type readyEngine struct {
	mu          sync.Mutex
	ensureImage string
	policy      container.PullPolicy
	starts      int
	stops       int
	removes     int
}

func (engine *readyEngine) EnsureImage(_ context.Context, image string, policy container.PullPolicy) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.ensureImage = image
	engine.policy = policy
	return nil
}

func (engine *readyEngine) Start(context.Context, container.ContainerSpec) (string, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.starts++
	return "container", nil
}

func (*readyEngine) ReadLogs(context.Context, string) ([]byte, error) {
	return []byte("Server for gameid=\"voxelibre\" listening on [::]:30000.\n"), nil
}

func (*readyEngine) IsRunning(context.Context, string) (bool, error) {
	return true, nil
}

func (*readyEngine) Wait(ctx context.Context, _ string) (int, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

func (engine *readyEngine) Stop(context.Context, string, time.Duration) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.stops++
	return nil
}

func (engine *readyEngine) Remove(context.Context, string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.removes++
	return nil
}

func TestRootCommandWiresFlagsThroughViper(t *testing.T) {
	temporaryDirectory := t.TempDir()
	cloneDirectory := filepath.Join(temporaryDirectory, "VoxeLibre")
	if err := os.Mkdir(cloneDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloneDirectory, "game.conf"), []byte("title = VoxeLibre\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(temporaryDirectory, "empty.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	engine := &readyEngine{}
	selectedEngine := ""
	root := newRootCommand(dependencies{
		newEngine: func(_ context.Context, preference string) (container.Engine, error) {
			selectedEngine = preference
			return engine, nil
		},
	})
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{
		"--config", configPath,
		"--voxelibre-dir", cloneDirectory,
		"--container-engine", "podman",
		"--image", "local-image:test",
		"--pull-policy", "never",
		"server", "unittests",
	})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if selectedEngine != "podman" {
		t.Fatalf("selected engine = %q, want podman", selectedEngine)
	}
	if engine.ensureImage != "local-image:test" || engine.policy != container.PullNever {
		t.Fatalf("image setup = %q/%q", engine.ensureImage, engine.policy)
	}
	if engine.starts != 3 || engine.stops != 3 || engine.removes != 3 {
		t.Fatalf("lifecycle counts: starts=%d stops=%d removes=%d", engine.starts, engine.stops, engine.removes)
	}
	if !strings.Contains(output.String(), "PASS  all 3 server startup tests") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestServerUnitTestsRejectsArguments(t *testing.T) {
	root := newRootCommand(defaultDependencies())
	root.SetArgs([]string{"server", "unittests", "unexpected"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected positional argument error")
	}
}
