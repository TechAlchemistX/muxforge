package cmd

import (
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [plugin]",
		Short: "Install plugins from config or add a new plugin",
		Long: `Install all plugins declared in the managed block, or add and install
a single plugin by specifying its name (owner/repo or full HTTPS URL).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 2 implementation.
			return nil
		},
	}
	return cmd
}
