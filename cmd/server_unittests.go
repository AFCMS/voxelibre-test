// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// unittestsCmd represents the unittests command
var unittestsCmd = &cobra.Command{
	Use:   "unittests",
	Short: "Assert server doesn't fail unit tests",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("unittests called")
	},
}

func init() {
	serverCmd.AddCommand(unittestsCmd)
}
