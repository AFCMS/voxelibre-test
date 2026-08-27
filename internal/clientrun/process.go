// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package clientrun

import (
	"context"
	"os/exec"
)

type ExecProcessRunner struct{}

func (ExecProcessRunner) Run(ctx context.Context, spec ProcessSpec) error {
	command := exec.CommandContext(ctx, spec.Executable, spec.Arguments...)
	command.Dir = spec.Directory
	command.Env = spec.Environment
	command.Stdin = spec.Stdin
	command.Stdout = spec.Stdout
	command.Stderr = spec.Stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}
