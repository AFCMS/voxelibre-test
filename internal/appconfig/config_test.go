// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package appconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestConfigureDefaults(t *testing.T) {
	v := viper.New()
	Configure(v)

	assertEqual(t, KeyVoxeLibreCloneDir, v.GetString(KeyVoxeLibreCloneDir), DefaultVoxeLibreCloneDir)
	assertEqual(t, KeyContainerEngine, v.GetString(KeyContainerEngine), DefaultContainerEngine)
	assertEqual(t, KeyContainerImage, v.GetString(KeyContainerImage), DefaultContainerImage)
	assertEqual(t, KeyContainerPull, v.GetString(KeyContainerPull), DefaultContainerPull)
}

func TestConfigurationPrecedence(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configClone := makeGame(t, temporaryDirectory, "config-game")
	environmentClone := makeGame(t, temporaryDirectory, "environment-game")
	flagClone := makeGame(t, temporaryDirectory, "flag-game")
	configPath := filepath.Join(temporaryDirectory, "custom.json")
	configJSON := `{
		"voxelibre": {"clone_dir": "` + configClone + `"},
		"container": {
			"engine": "podman",
			"image": "config-image:latest",
			"pull_policy": "always"
		}
	}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VLTEST_VOXELIBRE_CLONE_DIR", environmentClone)
	t.Setenv("VLTEST_CONTAINER_ENGINE", "docker")
	t.Setenv("VLTEST_CONTAINER_IMAGE", "environment-image:latest")
	t.Setenv("VLTEST_CONTAINER_PULL_POLICY", "never")

	v := viper.New()
	Configure(v)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := AddPersistentFlags(v, flags); err != nil {
		t.Fatal(err)
	}
	if err := flags.Parse([]string{"--voxelibre-dir", flagClone, "--image", "flag-image:latest"}); err != nil {
		t.Fatal(err)
	}
	if err := LoadFile(v, configPath); err != nil {
		t.Fatal(err)
	}

	settings, err := Read(v)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, KeyVoxeLibreCloneDir, settings.VoxeLibreCloneDir, flagClone)
	assertEqual(t, KeyContainerImage, settings.ContainerImage, "flag-image:latest")
	assertEqual(t, KeyContainerEngine, settings.ContainerEngine, "docker")
	assertEqual(t, KeyContainerPull, settings.ContainerPull, "never")
}

func TestConfigFileOverridesDefaults(t *testing.T) {
	temporaryDirectory := t.TempDir()
	cloneDirectory := makeGame(t, temporaryDirectory, "game")
	configPath := filepath.Join(temporaryDirectory, "vltest.json")
	configJSON := `{
		"voxelibre": {"clone_dir": "` + cloneDirectory + `"},
		"container": {"engine": "podman", "image": "configured:tag", "pull_policy": "always"}
	}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	Configure(v)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := AddPersistentFlags(v, flags); err != nil {
		t.Fatal(err)
	}
	if err := LoadFile(v, configPath); err != nil {
		t.Fatal(err)
	}
	settings, err := Read(v)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, KeyVoxeLibreCloneDir, settings.VoxeLibreCloneDir, cloneDirectory)
	assertEqual(t, KeyContainerEngine, settings.ContainerEngine, "podman")
	assertEqual(t, KeyContainerImage, settings.ContainerImage, "configured:tag")
	assertEqual(t, KeyContainerPull, settings.ContainerPull, "always")
}

func TestReadResolvesRelativeClonePath(t *testing.T) {
	temporaryDirectory := t.TempDir()
	makeGame(t, temporaryDirectory, "relative-game")
	t.Chdir(temporaryDirectory)

	v := viper.New()
	Configure(v)
	v.Set(KeyVoxeLibreCloneDir, "relative-game")
	settings, err := Read(v)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, KeyVoxeLibreCloneDir, settings.VoxeLibreCloneDir, filepath.Join(temporaryDirectory, "relative-game"))
}

func TestReadValidation(t *testing.T) {
	temporaryDirectory := t.TempDir()
	validClone := makeGame(t, temporaryDirectory, "valid")
	missingGameConfig := filepath.Join(temporaryDirectory, "missing-game-config")
	if err := os.Mkdir(missingGameConfig, 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		cloneDir   string
		engine     string
		image      string
		pullPolicy string
	}{
		{name: "missing clone", cloneDir: filepath.Join(temporaryDirectory, "missing"), engine: "auto", image: "image", pullPolicy: "missing"},
		{name: "missing game config", cloneDir: missingGameConfig, engine: "auto", image: "image", pullPolicy: "missing"},
		{name: "invalid engine", cloneDir: validClone, engine: "containerd", image: "image", pullPolicy: "missing"},
		{name: "empty image", cloneDir: validClone, engine: "auto", image: " ", pullPolicy: "missing"},
		{name: "invalid pull policy", cloneDir: validClone, engine: "auto", image: "image", pullPolicy: "sometimes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := viper.New()
			Configure(v)
			v.Set(KeyVoxeLibreCloneDir, test.cloneDir)
			v.Set(KeyContainerEngine, test.engine)
			v.Set(KeyContainerImage, test.image)
			v.Set(KeyContainerPull, test.pullPolicy)
			if _, err := Read(v); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadFileErrors(t *testing.T) {
	temporaryDirectory := t.TempDir()
	malformedPath := filepath.Join(temporaryDirectory, "malformed.json")
	if err := os.WriteFile(malformedPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{malformedPath, filepath.Join(temporaryDirectory, "missing.json")} {
		v := viper.New()
		Configure(v)
		if err := LoadFile(v, path); err == nil {
			t.Fatalf("expected explicit config error for %s", path)
		}
	}
}

func makeGame(t *testing.T, parent, name string) string {
	t.Helper()
	directory := filepath.Join(parent, name)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "game.conf"), []byte("title = VoxeLibre\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func assertEqual(t *testing.T, name, actual, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("%s: got %q, want %q", name, actual, expected)
	}
}
