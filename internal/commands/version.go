package commands

import (
	"fmt"

	"vulnsky/internal/version"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print VulnSky version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Version=%s\nCommit=%s\nBuildDate=%s\n", version.Version, version.Commit, version.BuildDate)
			return nil
		},
	}
}
