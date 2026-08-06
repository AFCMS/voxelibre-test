// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"fmt"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/servertest"
	"github.com/spf13/cobra"
)

// unittestsCmd represents the unittests command.
var unittestsCmd = &cobra.Command{
	Use:   "unittests",
	Short: "Assert VoxeLibre starts on every supported Luanti server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		settings, err := appconfig.ReadServer(configuration)
		if err != nil {
			return fmt.Errorf("validate configuration: %w", err)
		}
		pullPolicy, err := container.ParsePullPolicy(settings.Container.PullPolicy)
		if err != nil {
			return fmt.Errorf("validate pull policy: %w", err)
		}
		engine, err := commandDependencies.newEngine(cmd.Context(), settings.Container.Engine)
		if err != nil {
			return err
		}

		suite := servertest.NewSuite(
			engine,
			settings.Image,
			pullPolicy,
			settings.VoxeLibreCloneDir,
			cmd.OutOrStdout(),
		)
		return suite.Run(cmd.Context())
	},
}

func init() {
	serverCmd.AddCommand(unittestsCmd)
}
