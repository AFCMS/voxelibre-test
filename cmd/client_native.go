// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"fmt"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/clientrun"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
	"github.com/spf13/cobra"
)

var (
	clientNativeVersion    string
	clientNativeStartWorld bool
)

// clientNativeCmd represents the client native command.
var clientNativeCmd = &cobra.Command{
	Use:   "native --version VERSION [--data-dir DIR] [--start-world] [-- LUANTI_ARGS...]",
	Short: "Extract and launch a Luanti client on the host",
	Long: `Extract one supported run-in-place Luanti client build and launch it
directly on the Linux/x86-64 host. Without --data-dir the complete profile is
temporary. A data directory keeps one reusable, locked profile per version.`,
	Example: `  vltest client native --version 5.17.0
  vltest client native --version 5.17.0 --start-world
  vltest client native --version 5.17.0 --data-dir ~/.local/share/vltest
  vltest client native --version 5.17.0 -- --verbose`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && cmd.ArgsLenAtDash() < 0 {
			return fmt.Errorf("Luanti arguments must follow --")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := clientrun.ValidateNativePlatform(); err != nil {
			return err
		}
		if err := clientrun.ValidateArguments(args, clientNativeStartWorld); err != nil {
			return err
		}
		builds, err := luanti.SelectBuilds(clientNativeVersion, false, luanti.BuildKindClient)
		if err != nil {
			return err
		}
		settings, err := appconfig.ReadClient(configuration)
		if err != nil {
			return fmt.Errorf("validate client configuration: %w", err)
		}
		pullPolicy, err := container.ParsePullPolicy(settings.Container.PullPolicy)
		if err != nil {
			return fmt.Errorf("validate pull policy: %w", err)
		}
		engine, err := commandDependencies.newEngine(cmd.Context(), settings.Container.Engine)
		if err != nil {
			return err
		}

		runner := clientrun.NewNativeRunner(engine, commandDependencies.runProcess, clientrun.NativeOptions{
			Image:             settings.Image,
			PullPolicy:        pullPolicy,
			VoxeLibreCloneDir: settings.VoxeLibreCloneDir,
			DataDir:           settings.DataDir,
			Build:             builds[0],
			StartWorld:        clientNativeStartWorld,
			Arguments:         args,
			Stdin:             cmd.InOrStdin(),
			Stdout:            cmd.OutOrStdout(),
			Stderr:            cmd.ErrOrStderr(),
		})
		return runner.Run(cmd.Context())
	},
}

func init() {
	clientCmd.AddCommand(clientNativeCmd)
	clientNativeCmd.Flags().StringVar(
		&clientNativeVersion,
		"version",
		"",
		"supported Luanti version to launch",
	)
	clientNativeCmd.Flags().BoolVar(
		&clientNativeStartWorld,
		"start-world",
		false,
		"start the per-version VoxeLibre world immediately",
	)
	if err := clientNativeCmd.MarkFlagRequired("version"); err != nil {
		panic(err)
	}
}
