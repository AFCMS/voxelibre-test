// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"fmt"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/servertest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newServerUnitTestsCommand(configuration *viper.Viper, deps dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "unittests",
		Short: "Assert VoxeLibre starts on every supported Luanti server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			settings, err := appconfig.Read(configuration)
			if err != nil {
				return fmt.Errorf("validate configuration: %w", err)
			}
			pullPolicy, err := container.ParsePullPolicy(settings.ContainerPull)
			if err != nil {
				return fmt.Errorf("validate pull policy: %w", err)
			}
			engine, err := deps.newEngine(cmd.Context(), settings.ContainerEngine)
			if err != nil {
				return err
			}

			suite := servertest.NewSuite(
				engine,
				settings.ContainerImage,
				pullPolicy,
				settings.VoxeLibreCloneDir,
				cmd.OutOrStdout(),
			)
			return suite.Run(cmd.Context())
		},
	}
}
