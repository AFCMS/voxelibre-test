// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package luanti

import (
	"fmt"
	"path"
	"strings"
)

type BuildKind string

const (
	BuildKindAll    BuildKind = "all"
	BuildKindServer BuildKind = "server"
	BuildKindClient BuildKind = "client"
)

type Build struct {
	Version string
	Kind    BuildKind
}

func (build Build) Name() string {
	return fmt.Sprintf("luanti-%s-%s", build.Version, build.Kind)
}

func (build Build) ContainerPath() string {
	return path.Join("/work/dist", build.Name())
}

type ServerVersion struct {
	Version    string
	Entrypoint string
}

// availableVersions is the single source of truth for every Luanti build the
// CLI exposes. Keep it synchronized with the version stages in
// docker/luanti/Dockerfile.
var availableVersions = [...]string{
	"5.14.0",
	"5.15.2",
	"5.16.1",
}

func AvailableVersions() []string {
	return append([]string(nil), availableVersions[:]...)
}

func ParseBuildKind(value string) (BuildKind, error) {
	switch kind := BuildKind(strings.ToLower(strings.TrimSpace(value))); kind {
	case BuildKindAll, BuildKindServer, BuildKindClient:
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported build kind %q: expected all, server, or client", value)
	}
}

func SelectBuilds(version string, all bool, kind BuildKind) ([]Build, error) {
	version = strings.TrimSpace(version)
	if all && version != "" {
		return nil, fmt.Errorf("--version and --all are mutually exclusive")
	}
	if !all && version == "" {
		return nil, fmt.Errorf("one of --version or --all is required")
	}
	if kind != BuildKindAll && kind != BuildKindServer && kind != BuildKindClient {
		return nil, fmt.Errorf("unsupported build kind %q", kind)
	}

	if version != "" && !isSupportedVersion(version) {
		return nil, fmt.Errorf("unsupported Luanti version %q: expected one of %s", version, strings.Join(AvailableVersions(), ", "))
	}

	var builds []Build
	for _, supportedVersion := range availableVersions {
		if version != "" && version != supportedVersion {
			continue
		}
		if kind == BuildKindAll || kind == BuildKindServer {
			builds = append(builds, Build{Version: supportedVersion, Kind: BuildKindServer})
		}
		if kind == BuildKindAll || kind == BuildKindClient {
			builds = append(builds, Build{Version: supportedVersion, Kind: BuildKindClient})
		}
	}
	return builds, nil
}

func SupportedServerVersions() []ServerVersion {
	versions := make([]ServerVersion, 0, len(availableVersions))
	for _, version := range availableVersions {
		build := Build{Version: version, Kind: BuildKindServer}
		versions = append(versions, ServerVersion{
			Version:    version,
			Entrypoint: path.Join(build.ContainerPath(), "bin/luantiserver"),
		})
	}
	return versions
}

func isSupportedVersion(version string) bool {
	for _, supportedVersion := range availableVersions {
		if version == supportedVersion {
			return true
		}
	}
	return false
}
