// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type workspaceMount struct {
	Type        string
	Name        string
	Source      string
	Destination string
	SubPath     string
}

// Discovery is only attempted inside a container. A host's hostname must not
// accidentally select an unrelated container managed by its daemon.
func detectWorkspaceContainer() (string, error) {
	inside := false
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			inside = true
		}
	}
	if !inside {
		return "", nil
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "", fmt.Errorf("detect workspace container hostname: %v; set --workspace-container to its ID/name", err)
	}
	return hostname, nil
}

func (engine *CLIEngine) loadWorkspaceMounts(ctx context.Context) ([]workspaceMount, error) {
	engine.workspaceMu.Lock()
	defer engine.workspaceMu.Unlock()
	if engine.workspaceLoaded {
		return engine.workspaceMounts, nil
	}
	containerID := engine.workspaceContainer
	if containerID == "auto" {
		var err error
		containerID, err = engine.detectWorkspace()
		if err != nil {
			return nil, err
		}
	}
	if containerID == "" || containerID == "none" {
		engine.workspaceLoaded = true
		return nil, nil
	}
	inspectCtx, cancel := context.WithTimeout(ctx, engineProbeTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	if err := engine.runner.Run(inspectCtx, engine.executable, []string{"container", "inspect", containerID}, &stdout, &stderr); err != nil {
		return nil, fmt.Errorf("inspect workspace container %q: %w; set --workspace-container to the job container ID/name, or none when the daemon already sees the checkout path", containerID, commandError(engine.name+" container inspect", err, stderr.String()))
	}
	var containers []struct {
		Mounts     []workspaceMount
		HostConfig struct {
			Mounts []struct {
				Target        string
				VolumeOptions *struct{ Subpath string }
			}
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &containers); err != nil {
		return nil, fmt.Errorf("decode workspace container mounts: %w", err)
	}
	if len(containers) != 1 {
		return nil, fmt.Errorf("inspect workspace container: expected one container, got %d", len(containers))
	}
	// Keep a non-nil empty slice: inspection succeeded, but no local paths
	// can be translated from this container's writable layer.
	mounts := append([]workspaceMount{}, containers[0].Mounts...)
	for i := range mounts {
		if !filepath.IsAbs(mounts[i].Destination) {
			return nil, fmt.Errorf("workspace mount has invalid destination %q", mounts[i].Destination)
		}
		mounts[i].Destination = filepath.Clean(mounts[i].Destination)
		// Docker records a volume's original subpath in HostConfig.Mounts.
		for _, configured := range containers[0].HostConfig.Mounts {
			if configured.Target == mounts[i].Destination && configured.VolumeOptions != nil {
				mounts[i].SubPath = configured.VolumeOptions.Subpath
			}
		}
	}
	engine.workspaceMounts = mounts
	engine.workspaceLoaded = true
	return mounts, nil
}

func relativeWithin(parent, child string) (string, bool) {
	relative, err := filepath.Rel(parent, child)
	return relative, err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (engine *CLIEngine) resolveWorkspace(ctx context.Context, spec ContainerSpec) (ContainerSpec, error) {
	if len(spec.BindMounts) == 0 {
		return spec, nil
	}
	mounts, err := engine.loadWorkspaceMounts(ctx)
	if err != nil {
		return ContainerSpec{}, err
	}
	if mounts == nil {
		return spec, nil
	}
	resolved := spec
	resolved.BindMounts = nil
	resolved.VolumeMounts = append([]VolumeMount(nil), spec.VolumeMounts...)
	for _, requested := range spec.BindMounts {
		var match *workspaceMount
		for i := range mounts {
			mount := &mounts[i]
			if _, ok := relativeWithin(mount.Destination, requested.Source); ok && (match == nil || len(mount.Destination) > len(match.Destination)) {
				match = mount
			}
			if rel, ok := relativeWithin(requested.Source, mount.Destination); ok && rel != "." {
				return ContainerSpec{}, fmt.Errorf("workspace checkout %q contains nested mount %q; sharing this layout is unsupported", requested.Source, mount.Destination)
			}
		}
		if match == nil {
			return ContainerSpec{}, fmt.Errorf("checkout %q is not inside a workspace container mount; files in the container writable layer cannot be shared", requested.Source)
		}
		relative, _ := relativeWithin(match.Destination, requested.Source)
		switch match.Type {
		case "bind":
			if !filepath.IsAbs(match.Source) {
				return ContainerSpec{}, fmt.Errorf("workspace bind mount has invalid source %q", match.Source)
			}
			requested.Source = filepath.Join(match.Source, relative)
			resolved.BindMounts = append(resolved.BindMounts, requested)
		case "volume":
			if match.Name == "" {
				return ContainerSpec{}, fmt.Errorf("workspace volume at %q has no name", match.Destination)
			}
			if filepath.IsAbs(match.SubPath) || match.SubPath == ".." || strings.HasPrefix(filepath.Clean(match.SubPath), ".."+string(filepath.Separator)) {
				return ContainerSpec{}, fmt.Errorf("workspace volume has unsupported subpath %q", match.SubPath)
			}
			subpath := filepath.Join(match.SubPath, relative)
			if subpath == "." {
				subpath = ""
			}
			resolved.VolumeMounts = append(resolved.VolumeMounts, VolumeMount{
				Source: match.Name, Target: requested.Target, Subpath: subpath, ReadOnly: requested.ReadOnly,
			})
		default:
			return ContainerSpec{}, fmt.Errorf("workspace mount %q has unsupported type %q", match.Destination, match.Type)
		}
	}
	return resolved, nil
}
