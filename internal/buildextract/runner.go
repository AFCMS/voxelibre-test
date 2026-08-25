// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package buildextract

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
)

const cleanupTimeout = 10 * time.Second

type ImageReferences struct {
	Server string
	Client string
}

func (images ImageReferences) For(kind luanti.BuildKind) (string, error) {
	var image string
	switch kind {
	case luanti.BuildKindServer:
		image = images.Server
	case luanti.BuildKindClient:
		image = images.Client
	default:
		return "", fmt.Errorf("no container image for build kind %q", kind)
	}
	image = strings.TrimSpace(image)
	if image == "" {
		return "", fmt.Errorf("container %s image must not be empty", kind)
	}
	return image, nil
}

type Runner struct {
	engine     container.BuildExporter
	images     ImageReferences
	pullPolicy container.PullPolicy
	outputDir  string
	builds     []luanti.Build
	output     io.Writer
}

func NewRunner(
	engine container.BuildExporter,
	images ImageReferences,
	pullPolicy container.PullPolicy,
	outputDir string,
	builds []luanti.Build,
	output io.Writer,
) *Runner {
	if output == nil {
		output = io.Discard
	}
	return &Runner{
		engine:     engine,
		images:     images,
		pullPolicy: pullPolicy,
		outputDir:  outputDir,
		builds:     append([]luanti.Build(nil), builds...),
		output:     output,
	}
}

func (runner *Runner) Run(ctx context.Context) (resultErr error) {
	if len(runner.builds) == 0 {
		return errors.New("no Luanti builds selected for extraction")
	}
	groups, err := runner.groupBuilds()
	if err != nil {
		return err
	}

	outputDir, err := prepareOutputDir(runner.outputDir, runner.builds)
	if err != nil {
		return err
	}
	stagingDir, err := os.MkdirTemp(outputDir, ".vltest-extract-")
	if err != nil {
		return fmt.Errorf("create extraction staging directory in %q: %w", outputDir, err)
	}
	defer func() {
		if err := os.RemoveAll(stagingDir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove extraction staging directory: %w", err))
		}
	}()

	var containerIDs []string
	defer func() {
		for index := len(containerIDs) - 1; index >= 0; index-- {
			cleanupContext, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			err := runner.engine.Remove(cleanupContext, containerIDs[index])
			cancel()
			if err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("remove extraction container and volumes: %w", err))
			}
		}
	}()

	for _, group := range groups {
		if err := runner.engine.EnsureImage(ctx, group.image, runner.pullPolicy); err != nil {
			return fmt.Errorf("prepare %s container image %q: %w", group.kind, group.image, err)
		}
		containerID, err := runner.engine.Create(ctx, group.image)
		if err != nil {
			return fmt.Errorf("create %s extraction container: %w", group.kind, err)
		}
		containerIDs = append(containerIDs, containerID)

		for _, build := range group.builds {
			if _, err := fmt.Fprintf(runner.output, "EXTRACT %s\n", build.Name()); err != nil {
				return fmt.Errorf("write extraction status: %w", err)
			}
			stagedPath := filepath.Join(stagingDir, build.Name())
			if err := runner.engine.CopyFrom(ctx, containerID, build.ContainerPath(), stagedPath); err != nil {
				return fmt.Errorf("extract %s from image: %w", build.Name(), err)
			}
			info, err := os.Stat(stagedPath)
			if err != nil {
				return fmt.Errorf("inspect extracted build %s: %w", build.Name(), err)
			}
			if !info.IsDir() {
				return fmt.Errorf("extracted build %s is not a directory", build.Name())
			}
		}
	}

	committedPaths := make([]string, 0, len(runner.builds))
	for _, build := range runner.builds {
		stagedPath := filepath.Join(stagingDir, build.Name())
		destinationPath := filepath.Join(outputDir, build.Name())
		if err := os.Rename(stagedPath, destinationPath); err != nil {
			return errors.Join(
				fmt.Errorf("commit extracted build %s: %w", build.Name(), err),
				rollbackCommitted(committedPaths),
			)
		}
		committedPaths = append(committedPaths, destinationPath)
	}
	for _, destinationPath := range committedPaths {
		if _, err := fmt.Fprintf(runner.output, "WROTE   %s\n", destinationPath); err != nil {
			return fmt.Errorf("write extraction status: %w", err)
		}
	}

	return nil
}

type buildGroup struct {
	kind   luanti.BuildKind
	image  string
	builds []luanti.Build
}

func (runner *Runner) groupBuilds() ([]buildGroup, error) {
	var groups []buildGroup
	groupIndexes := make(map[luanti.BuildKind]int, 2)
	for _, build := range runner.builds {
		image, err := runner.images.For(build.Kind)
		if err != nil {
			return nil, fmt.Errorf("select image for %s: %w", build.Name(), err)
		}
		index, exists := groupIndexes[build.Kind]
		if !exists {
			index = len(groups)
			groupIndexes[build.Kind] = index
			groups = append(groups, buildGroup{kind: build.Kind, image: image})
		}
		groups[index].builds = append(groups[index].builds, build)
	}
	return groups, nil
}

func prepareOutputDir(outputDir string, builds []luanti.Build) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory %q: %w", outputDir, err)
	}
	resolvedOutputDir, err := filepath.EvalSymlinks(outputDir)
	if err != nil {
		return "", fmt.Errorf("resolve output directory %q: %w", outputDir, err)
	}
	info, err := os.Stat(resolvedOutputDir)
	if err != nil {
		return "", fmt.Errorf("inspect output directory %q: %w", resolvedOutputDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("output path %q is not a directory", resolvedOutputDir)
	}

	seen := make(map[string]struct{}, len(builds))
	for _, build := range builds {
		name := build.Name()
		if _, duplicate := seen[name]; duplicate {
			return "", fmt.Errorf("build %s was selected more than once", name)
		}
		seen[name] = struct{}{}

		destinationPath := filepath.Join(resolvedOutputDir, name)
		if _, err := os.Lstat(destinationPath); err == nil {
			return "", fmt.Errorf("refusing to overwrite existing build path %q", destinationPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect build destination %q: %w", destinationPath, err)
		}
	}

	return resolvedOutputDir, nil
}

func rollbackCommitted(paths []string) error {
	var rollbackErrors []error
	for index := len(paths) - 1; index >= 0; index-- {
		if err := os.RemoveAll(paths[index]); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("roll back %q: %w", paths[index], err))
		}
	}
	return errors.Join(rollbackErrors...)
}
