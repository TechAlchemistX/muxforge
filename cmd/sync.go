package cmd

import (
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile config, installed plugins, and lock file",
		Long: `Sync is the "fix whatever is wrong" command. It reconciles the declared
plugins in tmux.conf, the installed directories in ~/.tmux/plugins/, and the
pinned versions in tmux.lock. Safe to run at any time — it never removes
anything automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 3 implementation.
			return nil
		},
	}
	return cmd
}
