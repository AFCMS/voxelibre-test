// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"fmt"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/appconfig"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/buildextract"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luanti"
	"github.com/spf13/cobra"
)

var (
	extractBuildsVersion string
	extractBuildsAll     bool
	extractBuildsKind    string
)

// extractBuildsCmd represents the extract-builds command.
var extractBuildsCmd = &cobra.Command{
	Use:   "extract-builds",
	Short: "Export Luanti builds from the configured images",
	Long: `Export one or more run-in-place Luanti build directories from the
configured server and client images to the local filesystem. Only images needed
by the selected build kinds are used. Existing build directories are never
overwritten or merged.`,
	Example: `  vltest extract-builds --version 5.16.1
  vltest extract-builds --version 5.16.1 --kind client --output-dir ./artifacts
  vltest extract-builds --all --kind server
  vltest extract-builds --all`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		kind, err := luanti.ParseBuildKind(extractBuildsKind)
		if err != nil {
			return err
		}
		builds, err := luanti.SelectBuilds(extractBuildsVersion, extractBuildsAll, kind)
		if err != nil {
			return err
		}

		containerSettings, err := appconfig.ReadContainer(configuration)
		if err != nil {
			return fmt.Errorf("validate container configuration: %w", err)
		}
		images, err := configuredBuildImages(builds)
		if err != nil {
			return fmt.Errorf("validate container image configuration: %w", err)
		}
		extractSettings, err := appconfig.ReadExtractBuilds(configuration)
		if err != nil {
			return fmt.Errorf("validate extraction configuration: %w", err)
		}
		pullPolicy, err := container.ParsePullPolicy(containerSettings.PullPolicy)
		if err != nil {
			return fmt.Errorf("validate pull policy: %w", err)
		}
		engine, err := commandDependencies.newEngine(cmd.Context(), containerSettings.Engine)
		if err != nil {
			return err
		}

		runner := buildextract.NewRunner(
			engine,
			images,
			pullPolicy,
			extractSettings.OutputDir,
			builds,
			cmd.OutOrStdout(),
		)
		return runner.Run(cmd.Context())
	},
}

func configuredBuildImages(builds []luanti.Build) (buildextract.ImageReferences, error) {
	var images buildextract.ImageReferences
	for _, build := range builds {
		switch build.Kind {
		case luanti.BuildKindServer:
			if images.Server != "" {
				continue
			}
			image, err := appconfig.ReadServerImage(configuration)
			if err != nil {
				return buildextract.ImageReferences{}, err
			}
			images.Server = image
		case luanti.BuildKindClient:
			if images.Client != "" {
				continue
			}
			image, err := appconfig.ReadClientImage(configuration)
			if err != nil {
				return buildextract.ImageReferences{}, err
			}
			images.Client = image
		default:
			return buildextract.ImageReferences{}, fmt.Errorf("unsupported build kind %q", build.Kind)
		}
	}
	return images, nil
}

func init() {
	rootCmd.AddCommand(extractBuildsCmd)

	extractBuildsCmd.Flags().StringVar(
		&extractBuildsVersion,
		"version",
		"",
		"supported Luanti version to extract",
	)
	extractBuildsCmd.Flags().BoolVar(
		&extractBuildsAll,
		"all",
		false,
		"extract every supported Luanti version",
	)
	extractBuildsCmd.Flags().StringVar(
		&extractBuildsKind,
		"kind",
		string(luanti.BuildKindAll),
		"build kind to extract (all, server, or client)",
	)
	if err := appconfig.AddExtractBuildFlags(configuration, extractBuildsCmd.Flags()); err != nil {
		panic(err)
	}

	extractBuildsCmd.MarkFlagsMutuallyExclusive("version", "all")
	extractBuildsCmd.MarkFlagsOneRequired("version", "all")
}
