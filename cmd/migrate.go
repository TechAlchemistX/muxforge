package cmd

import (
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate an existing TPM setup to muxforge",
		Long: `Migrate converts a TPM-style tmux.conf to muxforge management.
It moves all @plugin declarations into the muxforge managed block, replaces
the TPM bootstrap line with the muxforge bootstrap line, and creates a
lock file recording current commit hashes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 3 implementation.
			return nil
		},
	}
	return cmd
}
