// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"github.com/spf13/cobra"
)

// clientCmd represents the client command.
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Launch VoxeLibre with a supported Luanti client",
	Args:  cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(clientCmd)
	if err := appconfig.AddClientFlags(configuration, clientCmd.PersistentFlags()); err != nil {
		panic(err)
	}
}
