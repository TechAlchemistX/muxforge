package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/ui"
	"github.com/spf13/cobra"
)

func newPurgeCmd() *cobra.Command {
	var dryRun bool
	var purgePlugins bool

	cmd := &cobra.Command{
		Use:     "purge",
		GroupID: "maintenance-commands",
		Short:   "Remove muxforge markers and bootstrap from tmux.conf",
		Long: `Purge removes all muxforge-specific lines from your tmux.conf while
preserving your plugin declarations so another plugin manager can pick
them up. It also removes the lock file.

This is the recommended cleanup step before running 'brew uninstall muxforge'.

With --purge-plugins, the plugins directory is also deleted.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPurge(dryRun, purgePlugins)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned actions without making any changes")
	cmd.Flags().BoolVar(&purgePlugins, "purge-plugins", false, "Also remove the plugins directory")

	return cmd
}

func runPurge(dryRun, purgePlugins bool) error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "nothing to purge")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	// Filter out muxforge markers and bootstrap lines, keep everything else
	// (including plugin declarations).
	var cleaned []string
	removedMarkers := 0
	removedBootstrap := 0

	for _, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case config.BlockStart, config.BlockEnd:
			removedMarkers++
			if dryRun {
				fmt.Printf("[dry-run] would remove: %s\n", trimmed)
			}
			continue

		case config.BootstrapLine, config.BootstrapLineLegacy:
			removedBootstrap++
			if dryRun {
				fmt.Printf("[dry-run] would remove: %s\n", trimmed)
			}
			continue
		}

		cleaned = append(cleaned, line)
	}

	if removedMarkers == 0 && removedBootstrap == 0 {
		ui.Info("no muxforge markers or bootstrap lines found in " + cfgPath)
	}

	// Write cleaned config.
	if !dryRun && (removedMarkers > 0 || removedBootstrap > 0) {
		cfg.Lines = cleaned
		if err := config.WriteConfig(cfg); err != nil {
			ui.Error(fmt.Sprintf("failed to write config: %v", err))
			os.Exit(1)
		}
		ui.Success(fmt.Sprintf("cleaned %s — removed %d marker(s), %d bootstrap line(s)",
			cfgPath, removedMarkers, removedBootstrap))
		ui.Info("plugin declarations preserved for use by another plugin manager")
	}

	// Remove lock file.
	lockRemoved := false
	if _, err := os.Stat(cfg.LockPath); err == nil {
		if dryRun {
			fmt.Printf("[dry-run] would remove lock file: %s\n", cfg.LockPath)
		} else {
			if err := os.Remove(cfg.LockPath); err != nil {
				ui.Warning(fmt.Sprintf("could not remove lock file: %v", err))
			} else {
				ui.Success("removed lock file " + cfg.LockPath)
				lockRemoved = true
			}
		}
	}

	// Optionally remove plugins directory.
	pluginsDir := config.PluginsDir(cfgPath)
	pluginsRemoved := false
	if purgePlugins {
		if dirExists(pluginsDir) {
			if dryRun {
				fmt.Printf("[dry-run] would remove plugins directory: %s\n", pluginsDir)
			} else {
				if err := os.RemoveAll(pluginsDir); err != nil {
					ui.Warning(fmt.Sprintf("could not remove plugins directory: %v", err))
				} else {
					ui.Success("removed plugins directory " + pluginsDir)
					pluginsRemoved = true
				}
			}
		} else {
			ui.Info("plugins directory does not exist: " + pluginsDir)
		}
	}

	if dryRun {
		return nil
	}

	// Summary.
	if removedMarkers == 0 && removedBootstrap == 0 && !lockRemoved && !pluginsRemoved {
		ui.Info("nothing to purge")
	} else {
		fmt.Println()
		ui.Success("muxforge purge complete")
		if !purgePlugins && dirExists(pluginsDir) {
			ui.Hint("plugin directory preserved: " + pluginsDir)
			ui.Hint("run with --purge-plugins to remove it")
		}
	}

	return nil
}
