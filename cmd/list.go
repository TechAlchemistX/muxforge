package cmd

import (
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all managed plugins and their status",
		Long: `Display a formatted table of all declared plugins with their install
status and lock file information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 2 implementation.
			return nil
		},
	}
	return cmd
}
