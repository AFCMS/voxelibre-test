// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/clientrun"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
	"github.com/spf13/cobra"
)

type readyEngine struct {
	mu           sync.Mutex
	ensureImages []string
	createImages []string
	policy       container.PullPolicy
	starts       int
	startSpecs   []container.ContainerSpec
	copies       int
	stops        int
	removes      int
	lintReport   string
	lintWaitCode int
}

func (engine *readyEngine) EnsureImage(_ context.Context, image string, policy container.PullPolicy) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.ensureImages = append(engine.ensureImages, image)
	engine.policy = policy
	return nil
}

func (engine *readyEngine) Start(_ context.Context, spec container.ContainerSpec) (string, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.starts++
	engine.startSpecs = append(engine.startSpecs, spec)
	return "container", nil
}

func (engine *readyEngine) Create(_ context.Context, image string) (string, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.createImages = append(engine.createImages, image)
	return "container", nil
}

func (engine *readyEngine) CopyFrom(_ context.Context, _, sourcePath, destinationPath string) error {
	engine.mu.Lock()
	engine.copies++
	lintReport := engine.lintReport
	engine.mu.Unlock()
	if sourcePath == "/tmp/log/check.json" {
		return os.WriteFile(filepath.Join(destinationPath, "check.json"), []byte(lintReport), 0o600)
	}
	if err := os.MkdirAll(filepath.Join(destinationPath, "bin"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destinationPath, "bin", "luanti"), []byte("binary"), 0o755)
}

func (*readyEngine) ReadLogs(context.Context, string) ([]byte, error) {
	return []byte("Server for gameid=\"voxelibre\" listening on [::]:30000.\n"), nil
}

func (*readyEngine) IsRunning(context.Context, string) (bool, error) {
	return true, nil
}

func (engine *readyEngine) Wait(ctx context.Context, _ string) (int, error) {
	engine.mu.Lock()
	lintReport := engine.lintReport
	waitCode := engine.lintWaitCode
	engine.mu.Unlock()
	if lintReport != "" {
		return waitCode, nil
	}
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
	previousDependencies := commandDependencies
	commandDependencies = dependencies{
		newEngine: func(_ context.Context, preference string) (container.Runtime, error) {
			selectedEngine = preference
			return engine, nil
		},
	}
	t.Cleanup(func() {
		commandDependencies = previousDependencies
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{
		"--config", configPath,
		"--voxelibre-dir", cloneDirectory,
		"--container-engine", "podman",
		"--server-image", "local-server:test",
		"--pull-policy", "never",
		"server", "unittests",
	})

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if selectedEngine != "podman" {
		t.Fatalf("selected engine = %q, want podman", selectedEngine)
	}
	if len(engine.ensureImages) != 1 || engine.ensureImages[0] != "local-server:test" || engine.policy != container.PullNever {
		t.Fatalf("image setup = %#v/%q", engine.ensureImages, engine.policy)
	}
	wantTests := len(luanti.SupportedServerVersions())
	if engine.starts != wantTests || engine.stops != wantTests || engine.removes != wantTests {
		t.Fatalf("lifecycle counts: starts=%d stops=%d removes=%d", engine.starts, engine.stops, engine.removes)
	}
	if !strings.Contains(output.String(), fmt.Sprintf("PASS  all %d server startup tests", wantTests)) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestServerUnitTestsRejectsArguments(t *testing.T) {
	rootCmd.SetArgs([]string{"server", "unittests", "unexpected"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected positional argument error")
	}
}

func TestExtractBuildsRegistrationAndFlags(t *testing.T) {
	if extractBuildsCmd.Parent() != rootCmd {
		t.Fatal("extract-builds command is not registered on rootCmd")
	}
	for _, flagName := range []string{"version", "all", "kind", "output-dir"} {
		if extractBuildsCmd.Flags().Lookup(flagName) == nil {
			t.Fatalf("extract-builds --%s flag is not registered", flagName)
		}
	}
	if flag := extractBuildsCmd.Flags().Lookup("toggle"); flag != nil {
		t.Fatal("placeholder --toggle flag is still registered")
	}
	if flag := extractBuildsCmd.Flags().Lookup("kind"); flag.DefValue != "all" {
		t.Fatalf("kind default = %q, want all", flag.DefValue)
	}
	if flag := extractBuildsCmd.Flags().Lookup("output-dir"); flag.DefValue != "./builds" {
		t.Fatalf("output-dir default = %q, want ./builds", flag.DefValue)
	}
	for _, flagName := range []string{"server-image", "client-image", "tools-image"} {
		if rootCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Fatalf("root --%s flag is not registered", flagName)
		}
	}
	if rootCmd.PersistentFlags().Lookup("image") != nil {
		t.Fatal("legacy root --image flag is still registered")
	}
}

func TestLintRegistrationAndFlags(t *testing.T) {
	if lintCmd.Parent() != rootCmd {
		t.Fatal("lint command is not registered on rootCmd")
	}
	flag := lintCmd.Flags().Lookup("check-level")
	if flag == nil {
		t.Fatal("lint --check-level flag is not registered")
	}
	if flag.DefValue != "warning" {
		t.Fatalf("check-level default = %q, want warning", flag.DefValue)
	}
}

func TestLintRejectsArguments(t *testing.T) {
	rootCmd.SetArgs([]string{"lint", "unexpected"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected positional argument error")
	}
}

func TestLintCommandUsesConfiguredToolsImageAndLevel(t *testing.T) {
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

	engine := &readyEngine{
		lintReport: `{"file:///path/to/voxelibre/init.lua":[{
  "code":"undefined-global","message":"Undefined global.","severity":2,
  "range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}
}]}`,
		lintWaitCode: 1,
	}
	selectedEngine := ""
	previousDependencies := commandDependencies
	commandDependencies = dependencies{
		newEngine: func(_ context.Context, preference string) (container.Runtime, error) {
			selectedEngine = preference
			return engine, nil
		},
	}
	t.Cleanup(func() {
		commandDependencies = previousDependencies
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{
		"--config", configPath,
		"--voxelibre-dir", cloneDirectory,
		"--container-engine", "podman",
		"--tools-image", "tools:local",
		"--pull-policy", "never",
		"lint", "--check-level", "information",
	})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if selectedEngine != "podman" {
		t.Fatalf("selected engine = %q", selectedEngine)
	}
	if len(engine.ensureImages) != 1 || engine.ensureImages[0] != "tools:local" || engine.policy != container.PullNever {
		t.Fatalf("image setup = %#v/%q", engine.ensureImages, engine.policy)
	}
	if len(engine.startSpecs) != 1 || !strings.Contains(strings.Join(engine.startSpecs[0].Arguments, " "), "--checklevel Information") {
		t.Fatalf("start specs = %#v", engine.startSpecs)
	}
	if !strings.Contains(output.String(), "::warning ") || !strings.Contains(output.String(), "LuaLS: 0 errors, 1 warnings") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestExtractBuildsCommandExportsWithoutVoxeLibreClone(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configPath := filepath.Join(temporaryDirectory, "empty.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(temporaryDirectory, "exports")
	engine := &readyEngine{}
	previousDependencies := commandDependencies
	commandDependencies = dependencies{
		newEngine: func(context.Context, string) (container.Runtime, error) {
			return engine, nil
		},
	}
	t.Cleanup(func() {
		commandDependencies = previousDependencies
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{
		"--config", configPath,
		"--voxelibre-dir", filepath.Join(temporaryDirectory, "does-not-exist"),
		"--container-engine", "docker",
		"--client-image", "client-image:local",
		"--pull-policy", "never",
		"extract-builds",
		"--version", "5.16.1",
		"--kind", "client",
		"--output-dir", outputDir,
	})

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	extractedPath := filepath.Join(outputDir, "luanti-5.16.1-client", "bin")
	if info, err := os.Stat(extractedPath); err != nil || !info.IsDir() {
		t.Fatalf("extracted path %q: %v", extractedPath, err)
	}
	if len(engine.ensureImages) != 1 || engine.ensureImages[0] != "client-image:local" {
		t.Fatalf("ensured images = %#v, want only client image", engine.ensureImages)
	}
	if len(engine.createImages) != 1 || engine.createImages[0] != "client-image:local" || engine.copies != 1 || engine.removes != 1 {
		t.Fatalf("extraction lifecycle = create:%d copy:%d remove:%d", len(engine.createImages), engine.copies, engine.removes)
	}
	if !strings.Contains(output.String(), "WROTE   "+filepath.Join(outputDir, "luanti-5.16.1-client")) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestClientNativeRegistrationAndFlags(t *testing.T) {
	if clientCmd.Parent() != rootCmd {
		t.Fatal("client command is not registered on rootCmd")
	}
	if clientNativeCmd.Parent() != clientCmd {
		t.Fatal("native command is not registered on clientCmd")
	}
	for _, flagName := range []string{"version", "start-world"} {
		if clientNativeCmd.Flags().Lookup(flagName) == nil {
			t.Fatalf("client native --%s flag is not registered", flagName)
		}
	}
	if clientCmd.PersistentFlags().Lookup("data-dir") == nil {
		t.Fatal("client --data-dir flag is not registered")
	}
	if flag := clientCmd.PersistentFlags().Lookup("data-dir"); flag.DefValue != "" {
		t.Fatalf("data-dir default = %q, want empty", flag.DefValue)
	}
}

func TestClientNativeCommandLaunchesSelectedBuild(t *testing.T) {
	temporaryDirectory := t.TempDir()
	cloneDirectory := filepath.Join(temporaryDirectory, "checkout")
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
	dataDir := filepath.Join(temporaryDirectory, "profiles")

	engine := &readyEngine{}
	selectedEngine := ""
	var processSpec clientrun.ProcessSpec
	previousDependencies := commandDependencies
	commandDependencies = dependencies{
		newEngine: func(_ context.Context, preference string) (container.Runtime, error) {
			selectedEngine = preference
			return engine, nil
		},
		runProcess: clientrun.ProcessRunnerFunc(func(_ context.Context, spec clientrun.ProcessSpec) error {
			processSpec = spec
			gamePath := environmentEntry(spec.Environment, "LUANTI_GAME_PATH")
			linkTarget, err := os.Readlink(filepath.Join(gamePath, "voxelibre"))
			if err != nil {
				return err
			}
			if linkTarget != cloneDirectory {
				return fmt.Errorf("game link target = %q, want %q", linkTarget, cloneDirectory)
			}
			return nil
		}),
	}
	t.Cleanup(func() {
		commandDependencies = previousDependencies
		rootCmd.SetArgs(nil)
		rootCmd.SetIn(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{
		"--config", configPath,
		"--voxelibre-dir", cloneDirectory,
		"--container-engine", "podman",
		"--client-image", "client-image:local",
		"--pull-policy", "never",
		"client", "native",
		"--version", "5.16.1",
		"--data-dir", dataDir,
		"--start-world",
		"--", "--name", "Test Player",
	})

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if selectedEngine != "podman" {
		t.Fatalf("selected engine = %q, want podman", selectedEngine)
	}
	if len(engine.ensureImages) != 1 || engine.ensureImages[0] != "client-image:local" || engine.policy != container.PullNever {
		t.Fatalf("image setup = %#v/%q", engine.ensureImages, engine.policy)
	}
	profileDir := filepath.Join(dataDir, "luanti-5.16.1-client")
	wantArguments := []string{
		"--world", filepath.Join(profileDir, "worlds", "vltest"),
		"--gameid", "voxelibre", "--go",
		"--name", "Test Player",
	}
	if processSpec.Directory != profileDir || processSpec.Executable != filepath.Join(profileDir, "bin", "luanti") {
		t.Fatalf("process directory/executable = %q/%q", processSpec.Directory, processSpec.Executable)
	}
	if strings.Join(processSpec.Arguments, "\x00") != strings.Join(wantArguments, "\x00") {
		t.Fatalf("process arguments = %#v, want %#v", processSpec.Arguments, wantArguments)
	}
	if legacy := environmentEntry(processSpec.Environment, "MINETEST_GAME_PATH"); legacy == "" {
		t.Fatal("legacy game path environment variable is missing")
	}
	if !strings.Contains(output.String(), "WROTE   "+profileDir) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestClientNativeRequiresArgumentDelimiter(t *testing.T) {
	command := &cobra.Command{}
	if err := command.Flags().Parse([]string{"verbose"}); err != nil {
		t.Fatal(err)
	}
	err := clientNativeCmd.Args(command, command.Flags().Args())
	if err == nil || !strings.Contains(err.Error(), "must follow --") {
		t.Fatalf("delimiter error = %v", err)
	}
}

func environmentEntry(environment []string, key string) string {
	prefix := key + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
