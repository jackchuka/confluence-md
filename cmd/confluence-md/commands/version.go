package commands

import (
	"fmt"

	"github.com/jackchuka/confluence-md/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of confluence-md",
	Long:  `Print the version number of confluence-md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(version.Info())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
