package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mandeep/muxforge/internal/config"
	"github.com/mandeep/muxforge/internal/lock"
	"github.com/mandeep/muxforge/internal/plugin"
	"github.com/mandeep/muxforge/internal/ui"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile config, installed plugins, and lock file",
		Long: `Sync is the "fix whatever is wrong" command. It reconciles the declared
plugins in tmux.conf, the installed directories in ~/.tmux/plugins/, and the
pinned versions in tmux.lock. Safe to run at any time — it never removes
anything automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned actions without making any changes")

	return cmd
}

func runSync(dryRun bool) error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf and re-run 'muxforge sync'")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		ui.PrintError("failed to read lock file", err.Error(), "delete the lock file and re-run 'muxforge sync'")
		os.Exit(1)
	}

	// Build lookup sets keyed by full name ("owner/repo") and by repo segment
	// (the directory name). The plugins directory contains subdirectories named
	// after the repo segment only, so we need both forms.
	declaredByFullName := make(map[string]bool, len(cfg.ManagedPlugins))
	declaredByRepoName := make(map[string]bool, len(cfg.ManagedPlugins))
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		declaredByFullName[name] = true
		declaredByRepoName[lastSegment(name)] = true
	}

	// Scan the plugins directory for installed directory names (repo-name only).
	installedDirNames, err := scanPluginsDir()
	if err != nil {
		// Non-fatal: plugins dir might not exist yet.
		installedDirNames = []string{}
	}

	// Build a new lock file, starting from the existing one.
	newLF := lock.NewLockFile()
	for _, lp := range lf.Plugins {
		newLF.Plugins = append(newLF.Plugins, lp)
	}

	// --- Step 1: For each declared plugin not installed → install it ---
	for _, raw := range cfg.ManagedPlugins {
		p, err := plugin.NewPlugin(raw)
		if err != nil {
			ui.Error(fmt.Sprintf("invalid plugin %q: %v", raw, err))
			continue
		}

		if dirExists(p.InstallPath) {
			// Already installed — ensure it is in the lock file.
			if lock.FindPlugin(newLF, p.Name) == nil {
				commit, err := plugin.HeadCommit(p.InstallPath)
				if err == nil {
					upsertLockEntry(newLF, lock.LockedPlugin{
						Name:        p.Name,
						Source:      p.Source,
						Commit:      commit,
						InstalledAt: time.Now().UTC().Format(time.RFC3339),
					})
				}
			}
			continue
		}

		// Not installed — check if there is a pinned commit in the lock file.
		lockedEntry := lock.FindPlugin(lf, p.Name)

		if dryRun {
			fmt.Printf("[dry-run] would clone %s → %s\n", p.Name, p.InstallPath)
			continue
		}

		if lockedEntry != nil {
			// Clone and checkout the pinned commit.
			var s ui.Spinner
			s.Start(fmt.Sprintf("installing %s @ %s (pinned)...", p.Name, lockedEntry.Commit[:7]))
			if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
				s.Stop()
				ui.Error(fmt.Sprintf("failed to clone %s: %v", p.Name, err))
				continue
			}
			if err := plugin.CheckoutCommit(p.InstallPath, lockedEntry.Commit); err != nil {
				s.Stop()
				ui.Error(fmt.Sprintf("failed to checkout pinned commit for %s: %v", p.Name, err))
				continue
			}
			s.Stop()
			ui.Success(fmt.Sprintf("installed %s @ %s", p.Name, lockedEntry.Commit[:7]))
		} else {
			// Clone latest.
			var s ui.Spinner
			s.Start(fmt.Sprintf("installing %s...", p.Name))
			if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
				s.Stop()
				ui.Error(fmt.Sprintf("failed to clone %s: %v", p.Name, err))
				continue
			}
			s.Stop()

			commit, err := plugin.HeadCommit(p.InstallPath)
			if err != nil {
				ui.Error(fmt.Sprintf("failed to get HEAD commit for %s: %v", p.Name, err))
				continue
			}

			upsertLockEntry(newLF, lock.LockedPlugin{
				Name:        p.Name,
				Source:      p.Source,
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			})
			ui.Success(fmt.Sprintf("installed %s @ %s", p.Name, commit[:7]))
		}
	}

	// --- Step 2: For each installed dir not in config → warn (never auto-remove) ---
	// installedDirNames contains repo-name segments (e.g. "tmux-sensible"),
	// so we compare against declaredByRepoName.
	for _, dirName := range installedDirNames {
		if !declaredByRepoName[dirName] {
			ui.Warning(fmt.Sprintf(
				"orphaned plugin: %s — run 'muxforge remove %s' to clean up",
				dirName, dirName,
			))
		}
	}

	// --- Step 3: For each lock entry with missing directory → reinstall at pinned commit ---
	for _, lp := range lf.Plugins {
		if !declaredByFullName[lp.Name] {
			// Not in config — skip.
			continue
		}

		p, err := plugin.NewPlugin(lp.Name)
		if err != nil {
			continue
		}

		if dirExists(p.InstallPath) {
			// Directory exists — no action needed.
			continue
		}

		if dryRun {
			fmt.Printf("[dry-run] would clone %s → %s\n", lp.Name, p.InstallPath)
			continue
		}

		// Reinstall at pinned commit.
		var s ui.Spinner
		s.Start(fmt.Sprintf("reinstalling %s @ %s...", lp.Name, lp.Commit[:7]))
		if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
			s.Stop()
			ui.Error(fmt.Sprintf("failed to clone %s: %v", lp.Name, err))
			continue
		}
		if err := plugin.CheckoutCommit(p.InstallPath, lp.Commit); err != nil {
			s.Stop()
			ui.Error(fmt.Sprintf("failed to checkout pinned commit for %s: %v", lp.Name, err))
			continue
		}
		s.Stop()
		ui.Success(fmt.Sprintf("reinstalled %s @ %s", lp.Name, lp.Commit[:7]))
	}

	if dryRun {
		fmt.Println("[dry-run] would write lock file")
		return nil
	}

	// Rewrite lock file to reflect final state.
	if err := lock.WriteLock(cfg.LockPath, newLF); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	return nil
}

// scanPluginsDir returns the repo-name segments of all subdirectories inside
// ~/.tmux/plugins/ (top-level entries only).
func scanPluginsDir() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	pluginsDir := filepath.Join(home, ".tmux", "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("read plugins dir %q: %w", pluginsDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// lastSegment returns the last "/"-separated segment of a plugin name.
// For "owner/repo" it returns "repo"; for "repo" it returns "repo".
func lastSegment(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

// splitSlash splits a string by "/".
func splitSlash(s string) []string {
	return strings.Split(s, "/")
}
