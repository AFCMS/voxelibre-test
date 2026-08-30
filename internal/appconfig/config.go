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
	KeyContainerToolsImage  = "container.tools_image"
	KeyContainerPull        = "container.pull_policy"
	KeyClientDataDir        = "client.data_dir"
	KeyExtractOutputDir     = "extract_builds.output_dir"
	KeyLintCheckLevel       = "lint.check_level"

	DefaultVoxeLibreCloneDir    = "./VoxeLibre"
	DefaultContainerEngine      = "auto"
	DefaultContainerServerImage = "git.minetest.land/voxelibre/voxelibre-test/luanti-server:master"
	DefaultContainerClientImage = "git.minetest.land/voxelibre/voxelibre-test/luanti-client:master"
	DefaultContainerToolsImage  = "git.minetest.land/voxelibre/voxelibre-test/tools:master"
	DefaultContainerPull        = "missing"
	DefaultClientDataDir        = ""
	DefaultExtractOutputDir     = "./builds"
	DefaultLintCheckLevel       = "warning"
)

const (
	FlagVoxeLibreDir  = "voxelibre-dir"
	FlagEngine        = "container-engine"
	FlagServerImage   = "server-image"
	FlagClientImage   = "client-image"
	FlagToolsImage    = "tools-image"
	FlagPullPolicy    = "pull-policy"
	FlagClientDataDir = "data-dir"
	FlagOutputDir     = "output-dir"
	FlagCheckLevel    = "check-level"
	defaultConfigFile = "vltest.json"
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

type ClientSettings struct {
	VoxeLibreCloneDir string
	DataDir           string
	Container         ContainerSettings
	Image             string
}

type LintSettings struct {
	VoxeLibreCloneDir string
	Container         ContainerSettings
	Image             string
	CheckLevel        string
}

func Configure(v *viper.Viper) {
	v.SetDefault(KeyVoxeLibreCloneDir, DefaultVoxeLibreCloneDir)
	v.SetDefault(KeyContainerEngine, DefaultContainerEngine)
	v.SetDefault(KeyContainerServerImage, DefaultContainerServerImage)
	v.SetDefault(KeyContainerClientImage, DefaultContainerClientImage)
	v.SetDefault(KeyContainerToolsImage, DefaultContainerToolsImage)
	v.SetDefault(KeyContainerPull, DefaultContainerPull)
	v.SetDefault(KeyClientDataDir, DefaultClientDataDir)
	v.SetDefault(KeyExtractOutputDir, DefaultExtractOutputDir)
	v.SetDefault(KeyLintCheckLevel, DefaultLintCheckLevel)

	v.SetEnvPrefix("VLTEST")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
}

func AddClientFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.String(FlagClientDataDir, DefaultClientDataDir, "persistent directory for version-isolated client profiles")
	if err := v.BindPFlag(KeyClientDataDir, flags.Lookup(FlagClientDataDir)); err != nil {
		return fmt.Errorf("bind --%s to %s: %w", FlagClientDataDir, KeyClientDataDir, err)
	}
	return nil
}

func AddPersistentFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.String(FlagVoxeLibreDir, DefaultVoxeLibreCloneDir, "path to the VoxeLibre clone")
	flags.String(FlagEngine, DefaultContainerEngine, "container engine to use (auto, docker, or podman)")
	flags.String(FlagServerImage, DefaultContainerServerImage, "local or remote Luanti server builds image")
	flags.String(FlagClientImage, DefaultContainerClientImage, "local or remote Luanti client builds image")
	flags.String(FlagToolsImage, DefaultContainerToolsImage, "local or remote linting tools image")
	flags.String(FlagPullPolicy, DefaultContainerPull, "image pull policy (always, missing, or never)")

	bindings := map[string]string{
		KeyVoxeLibreCloneDir:    FlagVoxeLibreDir,
		KeyContainerEngine:      FlagEngine,
		KeyContainerServerImage: FlagServerImage,
		KeyContainerClientImage: FlagClientImage,
		KeyContainerToolsImage:  FlagToolsImage,
		KeyContainerPull:        FlagPullPolicy,
	}
	for key, flagName := range bindings {
		if err := v.BindPFlag(key, flags.Lookup(flagName)); err != nil {
			return fmt.Errorf("bind --%s to %s: %w", flagName, key, err)
		}
	}

	return nil
}

func AddLintFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	flags.String(
		FlagCheckLevel,
		DefaultLintCheckLevel,
		"minimum LuaLS diagnostic level (error, warning, information, or hint)",
	)
	if err := v.BindPFlag(KeyLintCheckLevel, flags.Lookup(FlagCheckLevel)); err != nil {
		return fmt.Errorf("bind --%s to %s: %w", FlagCheckLevel, KeyLintCheckLevel, err)
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
		return loadConfigFile(v, explicitPath)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	userConfigDirectory, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("get user config directory: %w", err)
	}

	for _, directory := range []string{workingDirectory, userConfigDirectory} {
		path := filepath.Join(directory, defaultConfigFile)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect config file %q: %w", path, err)
		}
		return loadConfigFile(v, path)
	}

	return nil
}

func loadConfigFile(v *viper.Viper, path string) error {
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file %q does not exist", path)
		}
		var parseError viper.ConfigParseError
		if errors.As(err, &parseError) {
			return fmt.Errorf("parse config file %q: %w", path, parseError.Unwrap())
		}
		return fmt.Errorf("read config file %q: %w", path, err)
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
		Container: containerSettings,
		Image:     image,
	}
	settings.VoxeLibreCloneDir, err = ReadVoxeLibreCloneDir(v)
	if err != nil {
		return ServerSettings{}, err
	}
	return settings, nil
}

func ReadClient(v *viper.Viper) (ClientSettings, error) {
	containerSettings, err := ReadContainer(v)
	if err != nil {
		return ClientSettings{}, err
	}
	image, err := ReadClientImage(v)
	if err != nil {
		return ClientSettings{}, err
	}
	cloneDir, err := ReadVoxeLibreCloneDir(v)
	if err != nil {
		return ClientSettings{}, err
	}

	dataDir := strings.TrimSpace(v.GetString(KeyClientDataDir))
	if dataDir != "" {
		dataDir, err = filepath.Abs(dataDir)
		if err != nil {
			return ClientSettings{}, fmt.Errorf("resolve client data directory: %w", err)
		}
	}

	return ClientSettings{
		VoxeLibreCloneDir: cloneDir,
		DataDir:           dataDir,
		Container:         containerSettings,
		Image:             image,
	}, nil
}

func ReadLint(v *viper.Viper) (LintSettings, error) {
	containerSettings, err := ReadContainer(v)
	if err != nil {
		return LintSettings{}, err
	}
	image, err := ReadToolsImage(v)
	if err != nil {
		return LintSettings{}, err
	}
	cloneDirectory, err := ReadVoxeLibreCloneDir(v)
	if err != nil {
		return LintSettings{}, err
	}
	checkLevel := strings.ToLower(strings.TrimSpace(v.GetString(KeyLintCheckLevel)))
	switch checkLevel {
	case "error", "warning", "information", "hint":
	default:
		return LintSettings{}, fmt.Errorf(
			"unsupported LuaLS check level %q: expected error, warning, information, or hint",
			checkLevel,
		)
	}
	return LintSettings{
		VoxeLibreCloneDir: cloneDirectory,
		Container:         containerSettings,
		Image:             image,
		CheckLevel:        checkLevel,
	}, nil
}

func ReadVoxeLibreCloneDir(v *viper.Viper) (string, error) {
	cloneDir := strings.TrimSpace(v.GetString(KeyVoxeLibreCloneDir))
	if cloneDir == "" {
		return "", errors.New("voxelibre clone directory must not be empty")
	}
	absoluteCloneDir, err := filepath.Abs(cloneDir)
	if err != nil {
		return "", fmt.Errorf("resolve VoxeLibre clone directory: %w", err)
	}
	resolvedCloneDir, err := filepath.EvalSymlinks(absoluteCloneDir)
	if err != nil {
		return "", fmt.Errorf("resolve VoxeLibre clone directory %q: %w", absoluteCloneDir, err)
	}
	cloneInfo, err := os.Stat(resolvedCloneDir)
	if err != nil {
		return "", fmt.Errorf("inspect VoxeLibre clone directory %q: %w", resolvedCloneDir, err)
	}
	if !cloneInfo.IsDir() {
		return "", fmt.Errorf("VoxeLibre clone path %q is not a directory", resolvedCloneDir)
	}
	gameConfigPath := filepath.Join(resolvedCloneDir, "game.conf")
	gameConfigInfo, err := os.Stat(gameConfigPath)
	if err != nil {
		return "", fmt.Errorf("VoxeLibre clone must contain game.conf: %w", err)
	}
	if !gameConfigInfo.Mode().IsRegular() {
		return "", fmt.Errorf("VoxeLibre game config %q is not a regular file", gameConfigPath)
	}
	return resolvedCloneDir, nil
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

func ReadToolsImage(v *viper.Viper) (string, error) {
	return readImage(v, KeyContainerToolsImage, "tools")
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
