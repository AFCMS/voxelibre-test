// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package clientrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/buildextract"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
)

const (
	gameID                  = "voxelibre"
	legacyGamePathVariable  = "MINETEST_GAME_PATH"
	currentGamePathVariable = "LUANTI_GAME_PATH"
)

type ProcessSpec struct {
	Executable  string
	Arguments   []string
	Directory   string
	Environment []string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

type ProcessRunner interface {
	Run(ctx context.Context, spec ProcessSpec) error
}

type ProcessRunnerFunc func(context.Context, ProcessSpec) error

func (function ProcessRunnerFunc) Run(ctx context.Context, spec ProcessSpec) error {
	return function(ctx, spec)
}

type NativeOptions struct {
	Image             string
	PullPolicy        container.PullPolicy
	VoxeLibreCloneDir string
	DataDir           string
	Build             luanti.Build
	StartWorld        bool
	Arguments         []string
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
}

type NativeRunner struct {
	engine  container.BuildExporter
	process ProcessRunner
	options NativeOptions
	goos    string
	goarch  string
}

func NewNativeRunner(
	engine container.BuildExporter,
	process ProcessRunner,
	options NativeOptions,
) *NativeRunner {
	if process == nil {
		process = ExecProcessRunner{}
	}
	options.Arguments = append([]string(nil), options.Arguments...)
	return &NativeRunner{
		engine:  engine,
		process: process,
		options: options,
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
	}
}

func ValidateArguments(arguments []string, startWorld bool) error {
	if !startWorld {
		return nil
	}
	for _, argument := range arguments {
		name, _, _ := strings.Cut(argument, "=")
		switch name {
		case "--world", "--worldname", "--gameid", "--go":
			return fmt.Errorf("--start-world conflicts with Luanti argument %s", name)
		}
	}
	return nil
}

func ValidateNativePlatform() error {
	return validateNativePlatform(runtime.GOOS, runtime.GOARCH)
}

func validateNativePlatform(goos, goarch string) error {
	if goos != "linux" || goarch != "amd64" {
		return fmt.Errorf("native client startup requires Linux/x86-64; this binary is running on %s/%s", goos, goarch)
	}
	return nil
}

func (runner *NativeRunner) Run(ctx context.Context) (resultErr error) {
	if err := validateNativePlatform(runner.goos, runner.goarch); err != nil {
		return err
	}
	if runner.engine == nil {
		return errors.New("native client startup requires a build exporter")
	}
	if runner.options.Build.Kind != luanti.BuildKindClient || strings.TrimSpace(runner.options.Build.Version) == "" {
		return fmt.Errorf("invalid native client build %q", runner.options.Build.Name())
	}
	if err := ValidateArguments(runner.options.Arguments, runner.options.StartWorld); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	profileDir, cleanupProfile, err := runner.prepareProfile(ctx)
	defer func() {
		if cleanupProfile != nil {
			resultErr = errors.Join(resultErr, cleanupProfile())
		}
	}()
	if err != nil {
		return err
	}

	executable := filepath.Join(profileDir, "bin", "luanti")
	if err := validateExecutable(executable); err != nil {
		return err
	}

	gamePath, err := os.MkdirTemp("", "vltest-games-")
	if err != nil {
		return fmt.Errorf("create temporary Luanti game path: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(gamePath); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary Luanti game path: %w", err))
		}
	}()
	if err := os.Symlink(runner.options.VoxeLibreCloneDir, filepath.Join(gamePath, gameID)); err != nil {
		return fmt.Errorf("expose VoxeLibre as game ID %q: %w", gameID, err)
	}

	arguments := make([]string, 0, len(runner.options.Arguments)+5)
	if runner.options.StartWorld {
		worldPath := filepath.Join(profileDir, "worlds", "vltest")
		if err := os.MkdirAll(filepath.Dir(worldPath), 0o755); err != nil {
			return fmt.Errorf("create client worlds directory: %w", err)
		}
		arguments = append(arguments, "--world", worldPath, "--gameid", gameID, "--go")
	}
	arguments = append(arguments, runner.options.Arguments...)

	environment := setEnvironmentVariable(os.Environ(), currentGamePathVariable, gamePath)
	environment = setEnvironmentVariable(environment, legacyGamePathVariable, gamePath)
	spec := ProcessSpec{
		Executable:  executable,
		Arguments:   arguments,
		Directory:   profileDir,
		Environment: environment,
		Stdin:       runner.options.Stdin,
		Stdout:      runner.options.Stdout,
		Stderr:      runner.options.Stderr,
	}
	if err := runner.process.Run(ctx, spec); err != nil {
		return fmt.Errorf("run Luanti %s client: %w", runner.options.Build.Version, err)
	}
	return nil
}

func (runner *NativeRunner) prepareProfile(ctx context.Context) (string, func() error, error) {
	if strings.TrimSpace(runner.options.DataDir) == "" {
		root, err := os.MkdirTemp("", "vltest-client-")
		if err != nil {
			return "", nil, fmt.Errorf("create temporary client profile directory: %w", err)
		}
		cleanup := func() error {
			if err := os.RemoveAll(root); err != nil {
				return fmt.Errorf("remove temporary client profile directory: %w", err)
			}
			return nil
		}
		if err := runner.extractProfile(ctx, root); err != nil {
			return "", cleanup, err
		}
		return filepath.Join(root, runner.options.Build.Name()), cleanup, nil
	}

	dataDir, err := filepath.Abs(runner.options.DataDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve client data directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("create client data directory %q: %w", dataDir, err)
	}
	dataDir, err = filepath.EvalSymlinks(dataDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve client data directory %q: %w", dataDir, err)
	}

	lockPath := filepath.Join(dataDir, "."+runner.options.Build.Name()+".lock")
	release, err := acquireProfileLock(lockPath)
	if err != nil {
		return "", nil, fmt.Errorf("lock client profile %q: %w", filepath.Join(dataDir, runner.options.Build.Name()), err)
	}
	cleanup := func() error {
		if err := release(); err != nil {
			return fmt.Errorf("release client profile lock: %w", err)
		}
		return nil
	}

	profileDir := filepath.Join(dataDir, runner.options.Build.Name())
	info, err := os.Stat(profileDir)
	switch {
	case err == nil && !info.IsDir():
		return "", cleanup, fmt.Errorf("client profile path %q is not a directory", profileDir)
	case err == nil:
		return profileDir, cleanup, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", cleanup, fmt.Errorf("inspect client profile %q: %w", profileDir, err)
	}

	if err := runner.extractProfile(ctx, dataDir); err != nil {
		return "", cleanup, err
	}
	return profileDir, cleanup, nil
}

func (runner *NativeRunner) extractProfile(ctx context.Context, outputDir string) error {
	extractor := buildextract.NewRunner(
		runner.engine,
		buildextract.ImageReferences{Client: runner.options.Image},
		runner.options.PullPolicy,
		outputDir,
		[]luanti.Build{runner.options.Build},
		runner.options.Stdout,
	)
	if err := extractor.Run(ctx); err != nil {
		return fmt.Errorf("extract native client profile: %w", err)
	}
	return nil
}

func validateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect native Luanti executable %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("native Luanti executable %q is not a regular file", path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("native Luanti executable %q is not executable", path)
	}
	return nil
}

func setEnvironmentVariable(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
