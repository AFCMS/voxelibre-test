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
	KeyVoxeLibreCloneDir    = "voxelibre.clone_dir"
	KeyContainerEngine      = "container.engine"
	KeyContainerServerImage = "container.server_image"
	KeyContainerClientImage = "container.client_image"
	KeyContainerPull        = "container.pull_policy"
	KeyExtractOutputDir     = "extract_builds.output_dir"

	DefaultVoxeLibreCloneDir    = "./VoxeLibre"
	DefaultContainerEngine      = "auto"
	DefaultContainerServerImage = "git.minetest.land/voxelibre/voxelibre-test/luanti-server:latest"
	DefaultContainerClientImage = "git.minetest.land/voxelibre/voxelibre-test/luanti-client:latest"
	DefaultContainerPull        = "missing"
	DefaultExtractOutputDir     = "./builds"
)

const (
	FlagVoxeLibreDir  = "voxelibre-dir"
	FlagEngine        = "container-engine"
	FlagServerImage   = "server-image"
	FlagClientImage   = "client-image"
	FlagPullPolicy    = "pull-policy"
	FlagOutputDir     = "output-dir"
	defaultConfigName = "vltest"
)

type ContainerSettings struct {
	Engine     string
	PullPolicy string
}

type ExtractBuildsSettings struct {
	OutputDir string
}

type ServerSettings struct {
	VoxeLibreCloneDir string
	Container         ContainerSettings
	Image             string
}

func Configure(v *viper.Viper) {
	v.SetDefault(KeyVoxeLibreCloneDir, DefaultVoxeLibreCloneDir)
	v.SetDefault(KeyContainerEngine, DefaultContainerEngine)
	v.SetDefault(KeyContainerServerImage, DefaultContainerServerImage)
	v.SetDefault(KeyContainerClientImage, DefaultContainerClientImage)
	v.SetDefault(KeyContainerPull, DefaultContainerPull)
	v.SetDefault(KeyExtractOutputDir, DefaultExtractOutputDir)

	v.SetEnvPrefix("VLTEST")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
}

func AddPersistentFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.String(FlagVoxeLibreDir, DefaultVoxeLibreCloneDir, "path to the VoxeLibre clone on the container host")
	flags.String(FlagEngine, DefaultContainerEngine, "container engine to use (auto, docker, or podman)")
	flags.String(FlagServerImage, DefaultContainerServerImage, "local or remote Luanti server builds image")
	flags.String(FlagClientImage, DefaultContainerClientImage, "local or remote Luanti client builds image")
	flags.String(FlagPullPolicy, DefaultContainerPull, "image pull policy (always, missing, or never)")

	bindings := map[string]string{
		KeyVoxeLibreCloneDir:    FlagVoxeLibreDir,
		KeyContainerEngine:      FlagEngine,
		KeyContainerServerImage: FlagServerImage,
		KeyContainerClientImage: FlagClientImage,
		KeyContainerPull:        FlagPullPolicy,
	}
	for key, flagName := range bindings {
		if err := v.BindPFlag(key, flags.Lookup(flagName)); err != nil {
			return fmt.Errorf("bind --%s to %s: %w", flagName, key, err)
		}
	}

	return nil
}

func AddExtractBuildFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.String(FlagOutputDir, DefaultExtractOutputDir, "directory in which extracted build directories are created")
	if err := v.BindPFlag(KeyExtractOutputDir, flags.Lookup(FlagOutputDir)); err != nil {
		return fmt.Errorf("bind --%s to %s: %w", FlagOutputDir, KeyExtractOutputDir, err)
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

func ReadServer(v *viper.Viper) (ServerSettings, error) {
	containerSettings, err := ReadContainer(v)
	if err != nil {
		return ServerSettings{}, err
	}
	image, err := ReadServerImage(v)
	if err != nil {
		return ServerSettings{}, err
	}
	settings := ServerSettings{
		VoxeLibreCloneDir: strings.TrimSpace(v.GetString(KeyVoxeLibreCloneDir)),
		Container:         containerSettings,
		Image:             image,
	}

	if settings.VoxeLibreCloneDir == "" {
		return ServerSettings{}, errors.New("voxelibre clone directory must not be empty")
	}
	absoluteCloneDir, err := filepath.Abs(settings.VoxeLibreCloneDir)
	if err != nil {
		return ServerSettings{}, fmt.Errorf("resolve VoxeLibre clone directory: %w", err)
	}
	resolvedCloneDir, err := filepath.EvalSymlinks(absoluteCloneDir)
	if err != nil {
		return ServerSettings{}, fmt.Errorf("resolve VoxeLibre clone directory %q: %w", absoluteCloneDir, err)
	}
	cloneInfo, err := os.Stat(resolvedCloneDir)
	if err != nil {
		return ServerSettings{}, fmt.Errorf("inspect VoxeLibre clone directory %q: %w", resolvedCloneDir, err)
	}
	if !cloneInfo.IsDir() {
		return ServerSettings{}, fmt.Errorf("VoxeLibre clone path %q is not a directory", resolvedCloneDir)
	}
	gameConfigPath := filepath.Join(resolvedCloneDir, "game.conf")
	gameConfigInfo, err := os.Stat(gameConfigPath)
	if err != nil {
		return ServerSettings{}, fmt.Errorf("VoxeLibre clone must contain game.conf: %w", err)
	}
	if !gameConfigInfo.Mode().IsRegular() {
		return ServerSettings{}, fmt.Errorf("VoxeLibre game config %q is not a regular file", gameConfigPath)
	}
	settings.VoxeLibreCloneDir = resolvedCloneDir
	return settings, nil
}

func ReadContainer(v *viper.Viper) (ContainerSettings, error) {
	settings := ContainerSettings{
		Engine:     strings.ToLower(strings.TrimSpace(v.GetString(KeyContainerEngine))),
		PullPolicy: strings.ToLower(strings.TrimSpace(v.GetString(KeyContainerPull))),
	}

	switch settings.Engine {
	case "auto", "docker", "podman":
	default:
		return ContainerSettings{}, fmt.Errorf("unsupported container engine %q: expected auto, docker, or podman", settings.Engine)
	}
	switch settings.PullPolicy {
	case "always", "missing", "never":
	default:
		return ContainerSettings{}, fmt.Errorf("unsupported pull policy %q: expected always, missing, or never", settings.PullPolicy)
	}
	return settings, nil
}

func ReadServerImage(v *viper.Viper) (string, error) {
	return readImage(v, KeyContainerServerImage, "server")
}

func ReadClientImage(v *viper.Viper) (string, error) {
	return readImage(v, KeyContainerClientImage, "client")
}

func readImage(v *viper.Viper, key, kind string) (string, error) {
	image := strings.TrimSpace(v.GetString(key))
	if image == "" {
		return "", fmt.Errorf("container %s image must not be empty", kind)
	}
	return image, nil
}

func ReadExtractBuilds(v *viper.Viper) (ExtractBuildsSettings, error) {
	outputDir := strings.TrimSpace(v.GetString(KeyExtractOutputDir))
	if outputDir == "" {
		return ExtractBuildsSettings{}, errors.New("extract builds output directory must not be empty")
	}
	absoluteOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return ExtractBuildsSettings{}, fmt.Errorf("resolve extract builds output directory: %w", err)
	}
	return ExtractBuildsSettings{OutputDir: absoluteOutputDir}, nil
}
