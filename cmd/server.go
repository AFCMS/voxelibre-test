// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"github.com/spf13/cobra"
)

// serverCmd represents the server command.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Test VoxeLibre with supported Luanti servers",
	Args:  cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
