package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mandeep/muxforge/internal/config"
	"github.com/mandeep/muxforge/internal/lock"
	"github.com/mandeep/muxforge/internal/plugin"
	"github.com/mandeep/muxforge/internal/ui"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "remove <plugin>",
		Short: "Remove a plugin from config, lock file, and disk",
		Long: `Remove a plugin by name (owner/repo or repo name only).
The plugin declaration is removed from the managed block, its directory
is deleted, and the lock file entry is removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				ui.PrintError(
					"remove requires exactly one argument",
					fmt.Sprintf("got %d arguments", len(args)),
					"usage: muxforge remove <owner/repo>",
				)
				os.Exit(1)
			}
			return runRemove(args[0], dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned actions without making any changes")

	return cmd
}

func runRemove(arg string, dryRun bool) error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf with managed plugins first")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	if cfg.ManagedBlockStart == -1 {
		ui.PrintError(
			"no managed block found",
			"your tmux.conf does not contain a muxforge managed block",
			"run 'muxforge install owner/repo' to set up the managed block",
		)
		os.Exit(1)
	}

	// Match the argument against managed plugins.
	// Support both full name (owner/repo) and partial repo name (last segment).
	argNorm := plugin.NormalizeName(arg)
	argRepo := repoSegment(argNorm)

	var matches []string
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		repo := repoSegment(name)
		if name == argNorm || repo == argRepo {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		ui.PrintError(
			fmt.Sprintf("plugin not found in config: %q", arg),
			"the plugin is not declared in your managed block",
			fmt.Sprintf("run 'muxforge install %s' to add it", arg),
		)
		os.Exit(1)

	case 1:
		// Proceed with the single match.

	default:
		// Ambiguous.
		fmt.Fprintf(os.Stderr, "Error: ambiguous plugin name %q — multiple matches:\n", arg)
		for _, m := range matches {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		fmt.Fprintln(os.Stderr, "       use the full owner/repo name to be specific")
		os.Exit(1)
	}

	target := matches[0]
	p, err := plugin.NewPlugin(target)
	if err != nil {
		ui.Error(fmt.Sprintf("invalid plugin %q: %v", target, err))
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("[dry-run] would remove: set -g @plugin '%s'\n", target)
		fmt.Printf("[dry-run] would delete directory: %s\n", p.InstallPath)
		fmt.Printf("[dry-run] would remove from lock file\n")
		return nil
	}

	// Remove from managed block.
	newPlugins := make([]string, 0, len(cfg.ManagedPlugins)-1)
	for _, raw := range cfg.ManagedPlugins {
		if plugin.NormalizeName(raw) != target {
			newPlugins = append(newPlugins, raw)
		}
	}

	if err := config.UpdateManagedBlock(cfg, newPlugins); err != nil {
		ui.Error(fmt.Sprintf("failed to update config: %v", err))
		os.Exit(1)
	}

	// Remove plugin directory.
	if err := plugin.RemovePlugin(p.InstallPath); err != nil {
		if errors.Is(err, plugin.ErrAlreadyGone) {
			ui.Warning(fmt.Sprintf("plugin directory already gone: %s", p.InstallPath))
		} else {
			ui.Error(fmt.Sprintf("failed to remove plugin directory: %v", err))
			os.Exit(1)
		}
	}

	// Remove from lock file.
	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		ui.Error(fmt.Sprintf("failed to read lock file: %v", err))
		os.Exit(1)
	}

	filtered := make([]lock.LockedPlugin, 0, len(lf.Plugins))
	for _, lp := range lf.Plugins {
		if lp.Name != target {
			filtered = append(filtered, lp)
		}
	}
	lf.Plugins = filtered

	if err := lock.WriteLock(cfg.LockPath, lf); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	ui.Success(fmt.Sprintf("removed %s", target))
	return nil
}

// repoSegment returns the last path segment of a plugin name.
// For "owner/repo" it returns "repo".
func repoSegment(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}
