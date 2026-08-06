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
	newEngine func(context.Context, string) (container.Engine, error)
}

func defaultDependencies() dependencies {
	return dependencies{
		newEngine: func(ctx context.Context, preference string) (container.Engine, error) {
			return container.NewCLIEngine(ctx, preference)
		},
	}
}

func Execute() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return newRootCommand(defaultDependencies()).ExecuteContext(ctx)
}

func newRootCommand(deps dependencies) *cobra.Command {
	configuration := viper.New()
	appconfig.Configure(configuration)

	var configFile string
	rootCommand := &cobra.Command{
		Use:          "vltest",
		Short:        "Run containerized compatibility tests for VoxeLibre",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := appconfig.LoadFile(configuration, configFile); err != nil {
				return err
			}
			if used := configuration.ConfigFileUsed(); used != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Using config file:", used)
			}
			return nil
		},
	}

	rootCommand.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"config file (default: vltest.json in the working or user config directory)",
	)
	if err := appconfig.AddPersistentFlags(configuration, rootCommand.PersistentFlags()); err != nil {
		panic(err)
	}

	rootCommand.AddCommand(newServerCommand(configuration, deps))
	return rootCommand
}
