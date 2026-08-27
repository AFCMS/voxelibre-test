// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package clientrun

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
)

type fakeExporter struct {
	mu sync.Mutex

	ensureImages []string
	pullPolicies []container.PullPolicy
	createImages []string
	copies       int
	removed      int
	copyErr      error
}

func (engine *fakeExporter) EnsureImage(_ context.Context, image string, policy container.PullPolicy) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.ensureImages = append(engine.ensureImages, image)
	engine.pullPolicies = append(engine.pullPolicies, policy)
	return nil
}

func (engine *fakeExporter) Create(_ context.Context, image string) (string, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.createImages = append(engine.createImages, image)
	return "container", nil
}

func (engine *fakeExporter) CopyFrom(_ context.Context, _, _, destinationPath string) error {
	engine.mu.Lock()
	engine.copies++
	err := engine.copyErr
	engine.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(destinationPath, "bin"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destinationPath, "bin", "luanti"), []byte("binary"), 0o755)
}

func (engine *fakeExporter) Remove(context.Context, string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.removed++
	return nil
}

func TestNativeRunnerEphemeralProfileAndGamePathAreRemoved(t *testing.T) {
	requireNativePlatform(t)
	cloneDir := makeGame(t)
	engine := &fakeExporter{}
	input := strings.NewReader("input")
	var output bytes.Buffer
	var stderr bytes.Buffer
	var profileRoot string
	var gamePath string
	arguments := []string{"--name", "Test Player", "--verbose"}

	process := ProcessRunnerFunc(func(_ context.Context, spec ProcessSpec) error {
		profileRoot = filepath.Dir(spec.Directory)
		gamePath = environmentValue(t, spec.Environment, currentGamePathVariable)
		if legacy := environmentValue(t, spec.Environment, legacyGamePathVariable); legacy != gamePath {
			t.Fatalf("legacy game path = %q, want %q", legacy, gamePath)
		}
		linkTarget, err := os.Readlink(filepath.Join(gamePath, gameID))
		if err != nil {
			t.Fatal(err)
		}
		if linkTarget != cloneDir {
			t.Fatalf("game symlink target = %q, want %q", linkTarget, cloneDir)
		}
		if spec.Executable != filepath.Join(spec.Directory, "bin", "luanti") {
			t.Fatalf("executable = %q", spec.Executable)
		}
		if !reflect.DeepEqual(spec.Arguments, arguments) {
			t.Fatalf("arguments = %#v, want %#v", spec.Arguments, arguments)
		}
		if spec.Stdin != input || spec.Stdout != &output || spec.Stderr != &stderr {
			t.Fatal("process did not inherit configured standard streams")
		}
		return nil
	})

	runner := NewNativeRunner(engine, process, nativeOptions(cloneDir, "", false, arguments, input, &output, &stderr))
	if err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{profileRoot, gamePath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary path %q still exists: %v", path, err)
		}
	}
	if !reflect.DeepEqual(engine.ensureImages, []string{"client:local"}) ||
		!reflect.DeepEqual(engine.pullPolicies, []container.PullPolicy{container.PullNever}) ||
		len(engine.createImages) != 1 || engine.copies != 1 || engine.removed != 1 {
		t.Fatalf("extraction lifecycle = ensure:%#v create:%#v copy:%d remove:%d", engine.ensureImages, engine.createImages, engine.copies, engine.removed)
	}
	if !strings.Contains(output.String(), "EXTRACT luanti-5.17.0-client") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestNativeRunnerPersistentProfileIsReusedAndRetainsWorld(t *testing.T) {
	requireNativePlatform(t)
	cloneDir := makeGame(t)
	dataDir := t.TempDir()
	engine := &fakeExporter{}
	var specs []ProcessSpec
	process := ProcessRunnerFunc(func(_ context.Context, spec ProcessSpec) error {
		spec.Arguments = append([]string(nil), spec.Arguments...)
		spec.Environment = append([]string(nil), spec.Environment...)
		specs = append(specs, spec)
		worldDir := filepath.Join(spec.Directory, "worlds", "vltest")
		if err := os.MkdirAll(worldDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte("gameid = voxelibre\n"), 0o600)
	})
	options := nativeOptions(cloneDir, dataDir, true, []string{"--name=Player"}, nil, io.Discard, io.Discard)

	for range 2 {
		runner := NewNativeRunner(engine, process, options)
		if err := runner.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	profileDir := filepath.Join(dataDir, options.Build.Name())
	worldPath := filepath.Join(profileDir, "worlds", "vltest")
	wantArguments := []string{"--world", worldPath, "--gameid", gameID, "--go", "--name=Player"}
	if len(specs) != 2 {
		t.Fatalf("process runs = %d, want 2", len(specs))
	}
	for _, spec := range specs {
		if spec.Directory != profileDir || !reflect.DeepEqual(spec.Arguments, wantArguments) {
			t.Fatalf("process spec directory/arguments = %q/%#v", spec.Directory, spec.Arguments)
		}
	}
	if _, err := os.Stat(filepath.Join(worldPath, "world.mt")); err != nil {
		t.Fatalf("persistent world was not retained: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "."+options.Build.Name()+".lock")); err != nil {
		t.Fatalf("profile lock file was not retained: %v", err)
	}
	if len(engine.ensureImages) != 1 || len(engine.createImages) != 1 || engine.copies != 1 || engine.removed != 1 {
		t.Fatalf("persistent profile was extracted more than once: ensure:%d create:%d copy:%d remove:%d", len(engine.ensureImages), len(engine.createImages), engine.copies, engine.removed)
	}
}

func TestNativeRunnerRejectsConcurrentPersistentProfileUse(t *testing.T) {
	requireNativePlatform(t)
	cloneDir := makeGame(t)
	dataDir := t.TempDir()
	options := nativeOptions(cloneDir, dataDir, false, nil, nil, io.Discard, io.Discard)
	profileDir := filepath.Join(dataDir, options.Build.Name())
	makeExecutable(t, profileDir)

	entered := make(chan struct{})
	release := make(chan struct{})
	firstProcess := ProcessRunnerFunc(func(context.Context, ProcessSpec) error {
		close(entered)
		<-release
		return nil
	})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- NewNativeRunner(&fakeExporter{}, firstProcess, options).Run(context.Background())
	}()
	<-entered

	secondStarted := false
	secondProcess := ProcessRunnerFunc(func(context.Context, ProcessSpec) error {
		secondStarted = true
		return nil
	})
	err := NewNativeRunner(&fakeExporter{}, secondProcess, options).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("second launch error = %v, want profile-in-use error", err)
	}
	if secondStarted {
		t.Fatal("second client process started while the profile was locked")
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestNativeRunnerCleansEphemeralProfileAfterProcessFailure(t *testing.T) {
	requireNativePlatform(t)
	cloneDir := makeGame(t)
	var profileRoot string
	var gamePath string
	process := ProcessRunnerFunc(func(_ context.Context, spec ProcessSpec) error {
		profileRoot = filepath.Dir(spec.Directory)
		gamePath = environmentValue(t, spec.Environment, currentGamePathVariable)
		return errors.New("display unavailable")
	})
	runner := NewNativeRunner(&fakeExporter{}, process, nativeOptions(cloneDir, "", false, nil, nil, io.Discard, io.Discard))

	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "display unavailable") {
		t.Fatalf("error = %v", err)
	}
	for _, path := range []string{profileRoot, gamePath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary path %q still exists after failure: %v", path, err)
		}
	}
}

func TestNativeRunnerCleansEphemeralProfileAfterCancellation(t *testing.T) {
	requireNativePlatform(t)
	cloneDir := makeGame(t)
	ctx, cancel := context.WithCancel(context.Background())
	var profileRoot string
	var gamePath string
	process := ProcessRunnerFunc(func(_ context.Context, spec ProcessSpec) error {
		profileRoot = filepath.Dir(spec.Directory)
		gamePath = environmentValue(t, spec.Environment, currentGamePathVariable)
		cancel()
		return ctx.Err()
	})
	runner := NewNativeRunner(&fakeExporter{}, process, nativeOptions(cloneDir, "", false, nil, nil, io.Discard, io.Discard))

	err := runner.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	for _, path := range []string{profileRoot, gamePath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary path %q still exists after cancellation: %v", path, err)
		}
	}
}

func TestValidateArgumentsRejectsStartWorldConflicts(t *testing.T) {
	for _, argument := range []string{"--world", "--world=/tmp/world", "--worldname=test", "--gameid", "--go"} {
		t.Run(argument, func(t *testing.T) {
			err := ValidateArguments([]string{"--name", "Player", argument}, true)
			if err == nil || !strings.Contains(err.Error(), argumentName(argument)) {
				t.Fatalf("ValidateArguments() = %v", err)
			}
		})
	}
	if err := ValidateArguments([]string{"--world", "/tmp/world"}, false); err != nil {
		t.Fatalf("arguments should be allowed without --start-world: %v", err)
	}
	if err := ValidateArguments([]string{"--name", "Player", "--verbose"}, true); err != nil {
		t.Fatalf("non-conflicting arguments were rejected: %v", err)
	}
}

func TestNativeRunnerRejectsUnsupportedPlatformBeforeExtraction(t *testing.T) {
	engine := &fakeExporter{}
	runner := NewNativeRunner(engine, nil, nativeOptions(makeGame(t), "", false, nil, nil, io.Discard, io.Discard))
	runner.goos = "darwin"
	runner.goarch = "arm64"

	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Linux/x86-64") {
		t.Fatalf("error = %v", err)
	}
	if len(engine.ensureImages) != 0 {
		t.Fatal("unsupported platform performed extraction work")
	}
}

func nativeOptions(
	cloneDir string,
	dataDir string,
	startWorld bool,
	arguments []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) NativeOptions {
	return NativeOptions{
		Image:             "client:local",
		PullPolicy:        container.PullNever,
		VoxeLibreCloneDir: cloneDir,
		DataDir:           dataDir,
		Build:             luanti.Build{Version: "5.17.0", Kind: luanti.BuildKindClient},
		StartWorld:        startWorld,
		Arguments:         arguments,
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stderr,
	}
}

func makeGame(t *testing.T) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "checkout with arbitrary name")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "game.conf"), []byte("title = VoxeLibre\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func makeExecutable(t *testing.T, profileDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(profileDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "bin", "luanti"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func environmentValue(t *testing.T, environment []string, key string) string {
	t.Helper()
	prefix := key + "="
	var values []string
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			values = append(values, strings.TrimPrefix(entry, prefix))
		}
	}
	if len(values) != 1 {
		t.Fatalf("environment contains %d values for %s: %#v", len(values), key, values)
	}
	return values[0]
}

func argumentName(argument string) string {
	name, _, _ := strings.Cut(argument, "=")
	return name
}

func requireNativePlatform(t *testing.T) {
	t.Helper()
	if err := ValidateNativePlatform(); err != nil {
		t.Skip(err)
	}
}
