package cmd

import (
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [plugin]",
		Short: "Update all plugins or a single plugin to their latest version",
		Long: `Pull the latest changes for all managed plugins, or for a single
plugin if specified. Updates the lock file with new commit hashes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 3 implementation.
			return nil
		},
	}
	return cmd
}
