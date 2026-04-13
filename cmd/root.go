// Package cmd contains the muxforge CLI command definitions.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

// Execute builds and runs the root cobra command with the provided build
// metadata injected at link time.
func Execute(version, commit, date string) {
	rootCmd = newRootCmd(version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd(version, commit, date string) *cobra.Command {
	var showVersion bool

	root := &cobra.Command{
		Use:   "muxforge",
		Short: "Reproducible tmux plugin manager",
		Long: `muxforge is a lock-file-based tmux plugin manager.
It manages plugins inside a dedicated block in your tmux.conf
and pins versions via a tmux.lock file for reproducible setups.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Printf("muxforge v%s (commit: %s, built: %s)\n", version, commit, date)
				return nil
			}
			return cmd.Help()
		},
	}

	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version information")

	root.AddGroup(
		&cobra.Group{ID: "plugin-commands", Title: "Plugin Commands:"},
		&cobra.Group{ID: "setup-commands", Title: "Setup Commands:"},
		&cobra.Group{ID: "maintenance-commands", Title: "Maintenance Commands:"},
	)

	root.AddCommand(newInstallCmd())
	root.AddCommand(newRemoveCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newMigrateCmd())
	root.AddCommand(newLoadCmd())
	root.AddCommand(newPurgeCmd())

	return root
}
