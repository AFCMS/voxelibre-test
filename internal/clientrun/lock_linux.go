//go:build linux

// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package clientrun

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func acquireProfileLock(path string) (func() error, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errors.Join(errors.New("profile is already in use by another vltest process"), closeErr)
		}
		return nil, errors.Join(fmt.Errorf("acquire file lock: %w", err), closeErr)
	}

	return func() error {
		return errors.Join(
			syscall.Flock(int(file.Fd()), syscall.LOCK_UN),
			file.Close(),
		)
	}, nil
}
