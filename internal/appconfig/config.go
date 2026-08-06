// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package appconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	KeyVoxeLibreCloneDir = "voxelibre.clone_dir"
	KeyContainerEngine   = "container.engine"
	KeyContainerImage    = "container.image"
	KeyContainerPull     = "container.pull_policy"

	DefaultVoxeLibreCloneDir = "./VoxeLibre"
	DefaultContainerEngine   = "auto"
	DefaultContainerImage    = "git.minetest.land/voxelibre/voxelibre-test:latest"
	DefaultContainerPull     = "missing"
)

const (
	FlagVoxeLibreDir  = "voxelibre-dir"
	FlagEngine        = "container-engine"
	FlagImage         = "image"
	FlagPullPolicy    = "pull-policy"
	defaultConfigName = "vltest"
)

type Settings struct {
	VoxeLibreCloneDir string
	ContainerEngine   string
	ContainerImage    string
	ContainerPull     string
}

func Configure(v *viper.Viper) {
	v.SetDefault(KeyVoxeLibreCloneDir, DefaultVoxeLibreCloneDir)
	v.SetDefault(KeyContainerEngine, DefaultContainerEngine)
	v.SetDefault(KeyContainerImage, DefaultContainerImage)
	v.SetDefault(KeyContainerPull, DefaultContainerPull)

	v.SetEnvPrefix("VLTEST")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
}

func AddPersistentFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.String(FlagVoxeLibreDir, DefaultVoxeLibreCloneDir, "path to the VoxeLibre clone on the container host")
	flags.String(FlagEngine, DefaultContainerEngine, "container engine to use (auto, docker, or podman)")
	flags.String(FlagImage, DefaultContainerImage, "local or remote Luanti test image")
	flags.String(FlagPullPolicy, DefaultContainerPull, "image pull policy (always, missing, or never)")

	bindings := map[string]string{
		KeyVoxeLibreCloneDir: FlagVoxeLibreDir,
		KeyContainerEngine:   FlagEngine,
		KeyContainerImage:    FlagImage,
		KeyContainerPull:     FlagPullPolicy,
	}
	for key, flagName := range bindings {
		if err := v.BindPFlag(key, flags.Lookup(flagName)); err != nil {
			return fmt.Errorf("bind --%s to %s: %w", flagName, key, err)
		}
	}

	return nil
}

func LoadFile(v *viper.Viper, explicitPath string) error {
	if explicitPath != "" {
		v.SetConfigFile(explicitPath)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("read config file %q: %w", explicitPath, err)
		}
		return nil
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	userConfigDirectory, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("get user config directory: %w", err)
	}

	v.SetConfigName(defaultConfigName)
	v.SetConfigType("json")
	v.AddConfigPath(workingDirectory)
	v.AddConfigPath(userConfigDirectory)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	return nil
}

func Read(v *viper.Viper) (Settings, error) {
	settings := Settings{
		VoxeLibreCloneDir: strings.TrimSpace(v.GetString(KeyVoxeLibreCloneDir)),
		ContainerEngine:   strings.ToLower(strings.TrimSpace(v.GetString(KeyContainerEngine))),
		ContainerImage:    strings.TrimSpace(v.GetString(KeyContainerImage)),
		ContainerPull:     strings.ToLower(strings.TrimSpace(v.GetString(KeyContainerPull))),
	}

	if settings.VoxeLibreCloneDir == "" {
		return Settings{}, errors.New("voxelibre clone directory must not be empty")
	}
	absoluteCloneDir, err := filepath.Abs(settings.VoxeLibreCloneDir)
	if err != nil {
		return Settings{}, fmt.Errorf("resolve VoxeLibre clone directory: %w", err)
	}
	resolvedCloneDir, err := filepath.EvalSymlinks(absoluteCloneDir)
	if err != nil {
		return Settings{}, fmt.Errorf("resolve VoxeLibre clone directory %q: %w", absoluteCloneDir, err)
	}
	cloneInfo, err := os.Stat(resolvedCloneDir)
	if err != nil {
		return Settings{}, fmt.Errorf("inspect VoxeLibre clone directory %q: %w", resolvedCloneDir, err)
	}
	if !cloneInfo.IsDir() {
		return Settings{}, fmt.Errorf("VoxeLibre clone path %q is not a directory", resolvedCloneDir)
	}
	gameConfigPath := filepath.Join(resolvedCloneDir, "game.conf")
	gameConfigInfo, err := os.Stat(gameConfigPath)
	if err != nil {
		return Settings{}, fmt.Errorf("VoxeLibre clone must contain game.conf: %w", err)
	}
	if !gameConfigInfo.Mode().IsRegular() {
		return Settings{}, fmt.Errorf("VoxeLibre game config %q is not a regular file", gameConfigPath)
	}
	settings.VoxeLibreCloneDir = resolvedCloneDir

	switch settings.ContainerEngine {
	case "auto", "docker", "podman":
	default:
		return Settings{}, fmt.Errorf("unsupported container engine %q: expected auto, docker, or podman", settings.ContainerEngine)
	}
	if settings.ContainerImage == "" {
		return Settings{}, errors.New("container image must not be empty")
	}
	switch settings.ContainerPull {
	case "always", "missing", "never":
	default:
		return Settings{}, fmt.Errorf("unsupported pull policy %q: expected always, missing, or never", settings.ContainerPull)
	}

	return settings, nil
}
