// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestReadResolvesRelativeClonePath(t *testing.T) {
	temporaryDirectory := t.TempDir()
	makeGame(t, temporaryDirectory, "relative-game")
	t.Chdir(temporaryDirectory)

	v := viper.New()
	Configure(v)
	v.Set(KeyVoxeLibreCloneDir, "relative-game")
	settings, err := ReadServer(v)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, KeyVoxeLibreCloneDir, settings.VoxeLibreCloneDir, filepath.Join(temporaryDirectory, "relative-game"))
}

func TestReadClientResolvesCloneAndOptionalDataDirectory(t *testing.T) {
	temporaryDirectory := t.TempDir()
	makeGame(t, temporaryDirectory, "game")
	t.Chdir(temporaryDirectory)

	v := viper.New()
	Configure(v)
	v.Set(KeyVoxeLibreCloneDir, "game")
	v.Set(KeyClientDataDir, "profiles")
	settings, err := ReadClient(v)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, KeyVoxeLibreCloneDir, settings.VoxeLibreCloneDir, filepath.Join(temporaryDirectory, "game"))
	assertEqual(t, KeyClientDataDir, settings.DataDir, filepath.Join(temporaryDirectory, "profiles"))
	assertEqual(t, KeyContainerClientImage, settings.Image, DefaultContainerClientImage)

	v.Set(KeyClientDataDir, "  ")
	settings, err = ReadClient(v)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, KeyClientDataDir, settings.DataDir, "")
}

func TestConfigurationSchemaDefinesOptionalClientDataDirectory(t *testing.T) {
	schema := readConfigurationSchema(t)
	client, exists := schema.Properties["client"]
	if !exists {
		t.Fatal("schema does not define client settings")
	}
	dataDir, exists := client.Properties["data_dir"]
	if !exists || dataDir.Type != "string" || dataDir.Default != "" {
		t.Fatalf("client.data_dir schema = %#v", dataDir)
	}
}

func TestConfigurationSchemaUsesPublishedBranchImageDefaults(t *testing.T) {
	schema := readConfigurationSchema(t)
	container, exists := schema.Properties["container"]
	if !exists {
		t.Fatal("schema does not define container settings")
	}

	for propertyName, expected := range map[string]string{
		"server_image": DefaultContainerServerImage,
		"client_image": DefaultContainerClientImage,
	} {
		property, exists := container.Properties[propertyName]
		if !exists {
			t.Fatalf("schema does not define container.%s", propertyName)
		}
		actual, ok := property.Default.(string)
		if !ok {
			t.Fatalf("container.%s default = %#v, want a string", propertyName, property.Default)
		}
		assertEqual(t, "container."+propertyName+" schema default", actual, expected)
	}
}

type schemaProperty struct {
	Type       string                    `json:"type"`
	Default    any                       `json:"default"`
	Properties map[string]schemaProperty `json:"properties"`
}

type configurationSchema struct {
	Properties map[string]schemaProperty `json:"properties"`
}

func readConfigurationSchema(t *testing.T) configurationSchema {
	t.Helper()
	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "vltest.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema configurationSchema
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestReadValidation(t *testing.T) {
	temporaryDirectory := t.TempDir()
	validClone := makeGame(t, temporaryDirectory, "valid")
	missingGameConfig := filepath.Join(temporaryDirectory, "missing-game-config")
	if err := os.Mkdir(missingGameConfig, 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		cloneDir    string
		engine      string
		serverImage string
		pullPolicy  string
	}{
		{name: "missing clone", cloneDir: filepath.Join(temporaryDirectory, "missing"), engine: "auto", serverImage: "image", pullPolicy: "missing"},
		{name: "missing game config", cloneDir: missingGameConfig, engine: "auto", serverImage: "image", pullPolicy: "missing"},
		{name: "invalid engine", cloneDir: validClone, engine: "containerd", serverImage: "image", pullPolicy: "missing"},
		{name: "empty server image", cloneDir: validClone, engine: "auto", serverImage: " ", pullPolicy: "missing"},
		{name: "invalid pull policy", cloneDir: validClone, engine: "auto", serverImage: "image", pullPolicy: "sometimes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := viper.New()
			Configure(v)
			v.Set(KeyVoxeLibreCloneDir, test.cloneDir)
			v.Set(KeyContainerEngine, test.engine)
			v.Set(KeyContainerServerImage, test.serverImage)
			v.Set(KeyContainerPull, test.pullPolicy)
			if _, err := ReadServer(v); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestImageValidationOnlyRequiresSelectedImage(t *testing.T) {
	cloneDirectory := makeGame(t, t.TempDir(), "game")

	t.Run("server ignores empty client image", func(t *testing.T) {
		v := viper.New()
		Configure(v)
		v.Set(KeyVoxeLibreCloneDir, cloneDirectory)
		v.Set(KeyContainerClientImage, " ")
		if _, err := ReadServer(v); err != nil {
			t.Fatalf("ReadServer() unexpectedly validated client image: %v", err)
		}
	})

	t.Run("client ignores empty server image", func(t *testing.T) {
		v := viper.New()
		Configure(v)
		v.Set(KeyContainerServerImage, " ")
		if _, err := ReadContainer(v); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadClientImage(v); err != nil {
			t.Fatalf("ReadClientImage() unexpectedly validated server image: %v", err)
		}
	})

	t.Run("selected client image must not be empty", func(t *testing.T) {
		v := viper.New()
		Configure(v)
		v.Set(KeyContainerClientImage, " ")
		if _, err := ReadClientImage(v); err == nil {
			t.Fatal("expected empty client image error")
		}
	})
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
