// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type dependencies struct {
	newEngine func(context.Context, string) (container.Runtime, error)
}

var (
	configFile          string
	configuration       = viper.New()
	commandDependencies = defaultDependencies()
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          "vltest",
	Short:        "Run containerized compatibility tests for VoxeLibre",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if err := appconfig.LoadFile(configuration, configFile); err != nil {
			return err
		}
		if used := configuration.ConfigFileUsed(); used != "" {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Loaded config file %q\n", used); err != nil {
				return fmt.Errorf("write config file status: %w", err)
			}
		} else {
			if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "No config file found; continuing without one"); err != nil {
				return fmt.Errorf("write config file status: %w", err)
			}
		}
		return nil
	},
}

func defaultDependencies() dependencies {
	return dependencies{
		newEngine: func(ctx context.Context, preference string) (container.Runtime, error) {
			return container.NewCLIEngine(ctx, preference)
		},
	}
}

func Execute() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	appconfig.Configure(configuration)

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"config file (default: vltest.json in the working or user config directory)",
	)
	if err := appconfig.AddPersistentFlags(configuration, rootCmd.PersistentFlags()); err != nil {
		panic(err)
	}
}
