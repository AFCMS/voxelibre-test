// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"fmt"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/lint"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
	"github.com/spf13/cobra"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint VoxeLibre with LuaLS",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		settings, err := appconfig.ReadLint(configuration)
		if err != nil {
			return fmt.Errorf("validate lint configuration: %w", err)
		}
		pullPolicy, err := container.ParsePullPolicy(settings.Container.PullPolicy)
		if err != nil {
			return fmt.Errorf("validate pull policy: %w", err)
		}
		checkLevel, err := luals.ParseCheckLevel(settings.CheckLevel)
		if err != nil {
			return fmt.Errorf("validate LuaLS check level: %w", err)
		}
		engine, err := commandDependencies.newEngine(cmd.Context(), settings.Container.Engine)
		if err != nil {
			return err
		}

		runner := lint.NewRunner(
			engine,
			settings.Image,
			pullPolicy,
			settings.VoxeLibreCloneDir,
			checkLevel,
			cmd.OutOrStdout(),
		)
		return runner.Run(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)
	if err := appconfig.AddLintFlags(configuration, lintCmd.Flags()); err != nil {
		panic(err)
	}
}
