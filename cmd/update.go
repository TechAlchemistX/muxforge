package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mandeep/muxforge/internal/config"
	"github.com/mandeep/muxforge/internal/lock"
	"github.com/mandeep/muxforge/internal/plugin"
	"github.com/mandeep/muxforge/internal/ui"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "update [plugin]",
		Short: "Update all plugins or a single plugin to their latest version",
		Long: `Pull the latest changes for all managed plugins, or for a single
plugin if specified. Updates the lock file with new commit hashes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runUpdateAll(dryRun)
			}
			return runUpdateOne(args[0], dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned actions without making any changes")

	return cmd
}

// runUpdateAll pulls the latest changes for all managed plugins.
// Plugins that fail are skipped with an error message; the lock file is
// written with all successful updates at the end.
func runUpdateAll(dryRun bool) error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf and re-run 'muxforge update'")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		ui.PrintError("failed to read lock file", err.Error(), "delete the lock file and run 'muxforge sync' to recreate it")
		os.Exit(1)
	}

	if len(cfg.ManagedPlugins) == 0 {
		ui.Info("no plugins declared in the managed block")
		ui.Hint("run 'muxforge install owner/repo' to add a plugin")
		return nil
	}

	updatedLF := lock.NewLockFile()
	// Preserve any lock entries that are not in the managed block.
	for _, lp := range lf.Plugins {
		updatedLF.Plugins = append(updatedLF.Plugins, lp)
	}

	for _, raw := range cfg.ManagedPlugins {
		p, err := plugin.NewPlugin(raw)
		if err != nil {
			ui.Error(fmt.Sprintf("invalid plugin %q: %v", raw, err))
			continue
		}

		if dryRun {
			fmt.Printf("[dry-run] would pull %s\n", p.Name)
			continue
		}

		if err := updatePlugin(p, lf, updatedLF); err != nil {
			ui.Error(fmt.Sprintf("%s: %v", p.Name, err))
			// Keep the old lock entry on failure — it was already copied above.
		}
	}

	if dryRun {
		return nil
	}

	if err := lock.WriteLock(cfg.LockPath, updatedLF); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	return nil
}

// runUpdateOne pulls the latest changes for a single named plugin.
func runUpdateOne(raw string, dryRun bool) error {
	p, err := plugin.NewPlugin(raw)
	if err != nil {
		ui.PrintError(
			fmt.Sprintf("invalid plugin %q", raw),
			err.Error(),
			"use 'owner/repo' or a full HTTPS URL",
		)
		os.Exit(1)
	}

	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf and re-run 'muxforge update'")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	// Verify the plugin is in the managed block.
	found := false
	for _, managed := range cfg.ManagedPlugins {
		if plugin.NormalizeName(managed) == p.Name {
			found = true
			break
		}
	}
	if !found {
		ui.PrintError(
			fmt.Sprintf("plugin %q not found in managed block", p.Name),
			"the plugin is not declared in your tmux.conf",
			fmt.Sprintf("run 'muxforge install %s' to add it", p.Name),
		)
		os.Exit(1)
	}

	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		ui.PrintError("failed to read lock file", err.Error(), "delete the lock file and run 'muxforge sync' to recreate it")
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("[dry-run] would pull %s\n", p.Name)
		return nil
	}

	updatedLF := lock.NewLockFile()
	// Copy all existing lock entries.
	for _, lp := range lf.Plugins {
		updatedLF.Plugins = append(updatedLF.Plugins, lp)
	}

	if err := updatePlugin(p, lf, updatedLF); err != nil {
		ui.Error(fmt.Sprintf("%s: %v", p.Name, err))
		os.Exit(1)
	}

	if err := lock.WriteLock(cfg.LockPath, updatedLF); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	return nil
}

// updatePlugin performs the actual update for a single plugin, printing the
// result and modifying updatedLF in place. The oldLF is used to look up the
// previous commit for comparison.
func updatePlugin(p *plugin.Plugin, oldLF, updatedLF *lock.LockFile) error {
	oldEntry := lock.FindPlugin(oldLF, p.Name)

	// Plugin not installed locally — clone it first.
	if !dirExists(p.InstallPath) {
		var s ui.Spinner
		s.Start(fmt.Sprintf("cloning %s (not installed)...", p.Name))
		if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
			s.Stop()
			return fmt.Errorf("clone failed: %w", err)
		}
		s.Stop()

		commit, err := plugin.HeadCommit(p.InstallPath)
		if err != nil {
			return fmt.Errorf("get HEAD commit after clone: %w", err)
		}

		upsertLockEntry(updatedLF, lock.LockedPlugin{
			Name:        p.Name,
			Source:      p.Source,
			Commit:      commit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		})
		ui.Success(fmt.Sprintf("installed %s @ %s", p.Name, commit[:7]))
		return nil
	}

	// Get commit before pull.
	oldCommit, err := plugin.HeadCommit(p.InstallPath)
	if err != nil {
		return fmt.Errorf("get HEAD commit before pull: %w", err)
	}

	// Pull.
	var s ui.Spinner
	s.Start(fmt.Sprintf("pulling %s...", p.Name))
	if err := plugin.Pull(p.InstallPath); err != nil {
		s.Stop()
		return fmt.Errorf("pull failed: %w", err)
	}
	s.Stop()

	// Get commit after pull.
	newCommit, err := plugin.HeadCommit(p.InstallPath)
	if err != nil {
		return fmt.Errorf("get HEAD commit after pull: %w", err)
	}

	if oldCommit == newCommit {
		fmt.Printf("%s: already up to date\n", p.Name)
		// Do not update InstalledAt when nothing changed.
		return nil
	}

	// Commits differ — update lock entry.
	fmt.Printf("%s: %s → %s\n", p.Name, oldCommit[:7], newCommit[:7])

	var installedAt string
	if oldEntry != nil {
		installedAt = oldEntry.InstalledAt
	} else {
		installedAt = time.Now().UTC().Format(time.RFC3339)
	}

	upsertLockEntry(updatedLF, lock.LockedPlugin{
		Name:        p.Name,
		Source:      p.Source,
		Commit:      newCommit,
		InstalledAt: installedAt,
	})

	return nil
}

// upsertLockEntry replaces the existing lock entry for the given plugin name,
// or appends it if no entry exists.
func upsertLockEntry(lf *lock.LockFile, entry lock.LockedPlugin) {
	for i, lp := range lf.Plugins {
		if lp.Name == entry.Name {
			lf.Plugins[i] = entry
			return
		}
	}
	lf.Plugins = append(lf.Plugins, entry)
}
