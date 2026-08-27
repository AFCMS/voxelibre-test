//go:build !linux

// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package clientrun

import "errors"

func acquireProfileLock(string) (func() error, error) {
	return nil, errors.New("client profile locking requires Linux")
}
