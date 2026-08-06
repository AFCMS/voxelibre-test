// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package buildextract

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
)

type fakeExporter struct {
	ensureImages []string
	ensurePulls  []container.PullPolicy
	createImages []string
	copies       []string
	removed      []string
	copyErrAt    int
	removeErr    error
}

func (engine *fakeExporter) EnsureImage(_ context.Context, image string, policy container.PullPolicy) error {
	engine.ensureImages = append(engine.ensureImages, image)
	engine.ensurePulls = append(engine.ensurePulls, policy)
	return nil
}

func (engine *fakeExporter) Create(_ context.Context, image string) (string, error) {
	engine.createImages = append(engine.createImages, image)
	return "container-" + image, nil
}

func (engine *fakeExporter) CopyFrom(_ context.Context, containerID, sourcePath, destinationPath string) error {
	index := len(engine.copies)
	engine.copies = append(engine.copies, containerID+":"+sourcePath)
	if engine.copyErrAt >= 0 && index == engine.copyErrAt {
		return errors.New("copy failed")
	}
	if err := os.MkdirAll(filepath.Join(destinationPath, "bin"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destinationPath, "bin", "luanti"), []byte("binary"), 0o755)
}

func (engine *fakeExporter) Remove(_ context.Context, containerID string) error {
	engine.removed = append(engine.removed, containerID)
	return engine.removeErr
}

func TestRunnerExtractsBuildsTransactionally(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "builds")
	engine := &fakeExporter{copyErrAt: -1}
	builds := []luanti.Build{
		{Version: "5.16.1", Kind: luanti.BuildKindServer},
		{Version: "5.16.1", Kind: luanti.BuildKindClient},
	}
	var output bytes.Buffer
	images := ImageReferences{Server: "server:local", Client: "client:local"}
	runner := NewRunner(engine, images, container.PullNever, outputDir, builds, &output)

	if err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(engine.ensureImages, []string{"server:local", "client:local"}) {
		t.Fatalf("ensured images = %#v", engine.ensureImages)
	}
	if !reflect.DeepEqual(engine.ensurePulls, []container.PullPolicy{container.PullNever, container.PullNever}) {
		t.Fatalf("pull policies = %#v", engine.ensurePulls)
	}
	if !reflect.DeepEqual(engine.createImages, engine.ensureImages) || len(engine.removed) != 2 || len(engine.copies) != 2 {
		t.Fatalf("lifecycle = create:%d copy:%d remove:%d", len(engine.createImages), len(engine.copies), len(engine.removed))
	}
	if !strings.HasPrefix(engine.copies[0], "container-server:local:") ||
		!strings.HasPrefix(engine.copies[1], "container-client:local:") {
		t.Fatalf("copies used wrong containers: %#v", engine.copies)
	}
	for _, build := range builds {
		binaryPath := filepath.Join(outputDir, build.Name(), "bin", "luanti")
		if _, err := os.Stat(binaryPath); err != nil {
			t.Fatalf("missing extracted file %s: %v", binaryPath, err)
		}
		if !strings.Contains(output.String(), "WROTE   "+filepath.Join(outputDir, build.Name())) {
			t.Fatalf("output %q does not report %s", output.String(), build.Name())
		}
	}
}

func TestRunnerOnlyUsesImagesRequiredBySelectedBuilds(t *testing.T) {
	tests := []struct {
		name   string
		build  luanti.Build
		images ImageReferences
		want   string
	}{
		{
			name:   "server does not require client image",
			build:  luanti.Build{Version: "5.16.1", Kind: luanti.BuildKindServer},
			images: ImageReferences{Server: "server:local"},
			want:   "server:local",
		},
		{
			name:   "client does not require server image",
			build:  luanti.Build{Version: "5.16.1", Kind: luanti.BuildKindClient},
			images: ImageReferences{Client: "client:local"},
			want:   "client:local",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := &fakeExporter{copyErrAt: -1}
			runner := NewRunner(
				engine,
				test.images,
				container.PullMissing,
				t.TempDir(),
				[]luanti.Build{test.build},
				nil,
			)

			if err := runner.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(engine.ensureImages, []string{test.want}) {
				t.Fatalf("ensured images = %#v, want only %q", engine.ensureImages, test.want)
			}
			if !reflect.DeepEqual(engine.createImages, []string{test.want}) {
				t.Fatalf("created from images = %#v, want only %q", engine.createImages, test.want)
			}
		})
	}
}

func TestRunnerRefusesExistingBuildBeforeContainerOperations(t *testing.T) {
	outputDir := t.TempDir()
	build := luanti.Build{Version: "5.16.1", Kind: luanti.BuildKindClient}
	if err := os.Mkdir(filepath.Join(outputDir, build.Name()), 0o755); err != nil {
		t.Fatal(err)
	}
	engine := &fakeExporter{copyErrAt: -1}
	runner := NewRunner(engine, ImageReferences{Client: "client"}, container.PullMissing, outputDir, []luanti.Build{build}, nil)

	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("got %v, want overwrite refusal", err)
	}
	if len(engine.createImages) != 0 || len(engine.copies) != 0 || len(engine.removed) != 0 {
		t.Fatalf("unexpected engine operations: create:%d copy:%d remove:%d", len(engine.createImages), len(engine.copies), len(engine.removed))
	}
}

func TestRunnerLeavesNoBuildsAfterCopyFailure(t *testing.T) {
	outputDir := t.TempDir()
	engine := &fakeExporter{copyErrAt: 1}
	builds := []luanti.Build{
		{Version: "5.14.0", Kind: luanti.BuildKindServer},
		{Version: "5.15.2", Kind: luanti.BuildKindServer},
	}
	runner := NewRunner(engine, ImageReferences{Server: "server"}, container.PullMissing, outputDir, builds, nil)

	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "copy failed") {
		t.Fatalf("got %v, want copy failure", err)
	}
	if len(engine.removed) != 1 {
		t.Fatalf("remove calls = %d, want 1", len(engine.removed))
	}
	for _, build := range builds {
		if _, err := os.Lstat(filepath.Join(outputDir, build.Name())); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("build %s was committed after failure", build.Name())
		}
	}
}

func TestRunnerReportsCleanupFailure(t *testing.T) {
	outputDir := t.TempDir()
	engine := &fakeExporter{copyErrAt: -1, removeErr: errors.New("remove failed")}
	build := luanti.Build{Version: "5.14.0", Kind: luanti.BuildKindClient}
	runner := NewRunner(engine, ImageReferences{Client: "client"}, container.PullMissing, outputDir, []luanti.Build{build}, nil)

	err := runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("got %v, want cleanup failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, build.Name())); statErr != nil {
		t.Fatalf("successfully extracted build should remain after cleanup failure: %v", statErr)
	}
}
