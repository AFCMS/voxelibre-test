// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newServerCommand(configuration *viper.Viper, deps dependencies) *cobra.Command {
	serverCommand := &cobra.Command{
		Use:   "server",
		Short: "Test VoxeLibre with supported Luanti servers",
		Args:  cobra.NoArgs,
	}
	serverCommand.AddCommand(newServerUnitTestsCommand(configuration, deps))
	return serverCommand
}
