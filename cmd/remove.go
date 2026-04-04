package cmd

import (
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <plugin>",
		Short: "Remove a plugin from config, lock file, and disk",
		Long: `Remove a plugin by name (owner/repo or repo name only).
The plugin declaration is removed from the managed block, its directory
is deleted, and the lock file entry is removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 2 implementation.
			return nil
		},
	}
	return cmd
}
