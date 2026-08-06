// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package container

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type PullPolicy string

const (
	PullAlways  PullPolicy = "always"
	PullMissing PullPolicy = "missing"
	PullNever   PullPolicy = "never"
)

func ParsePullPolicy(value string) (PullPolicy, error) {
	switch policy := PullPolicy(strings.ToLower(strings.TrimSpace(value))); policy {
	case PullAlways, PullMissing, PullNever:
		return policy, nil
	default:
		return "", fmt.Errorf("unsupported pull policy %q", value)
	}
}

type BindMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type ContainerSpec struct {
	Image            string
	Entrypoint       string
	Arguments        []string
	BindMounts       []BindMount
	AnonymousVolumes []string
}

type Engine interface {
	EnsureImage(ctx context.Context, image string, policy PullPolicy) error
	Start(ctx context.Context, spec ContainerSpec) (string, error)
	ReadLogs(ctx context.Context, containerID string) ([]byte, error)
	IsRunning(ctx context.Context, containerID string) (bool, error)
	Wait(ctx context.Context, containerID string) (int, error)
	Stop(ctx context.Context, containerID string, timeout time.Duration) error
	Remove(ctx context.Context, containerID string) error
}

type BuildExporter interface {
	EnsureImage(ctx context.Context, image string, policy PullPolicy) error
	Create(ctx context.Context, image string) (string, error)
	CopyFrom(ctx context.Context, containerID, sourcePath, destinationPath string) error
	Remove(ctx context.Context, containerID string) error
}

type Runtime interface {
	Engine
	BuildExporter
}
