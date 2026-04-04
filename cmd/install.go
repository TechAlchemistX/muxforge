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

func newInstallCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "install [plugin]",
		Short: "Install plugins from config or add a new plugin",
		Long: `Install all plugins declared in the managed block, or add and install
a single plugin by specifying its name (owner/repo or full HTTPS URL).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runInstallAll(dryRun)
			}
			return runInstallOne(args[0], dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned actions without making any changes")

	return cmd
}

// runInstallAll installs every plugin declared in the managed block,
// respecting pinned versions from the lock file.
func runInstallAll(dryRun bool) error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf and re-run 'muxforge install'")
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

	// Collect results before writing the lock file. On any network failure
	// we abort and write nothing.
	type result struct {
		locked lock.LockedPlugin
		skipped bool
	}
	results := make([]result, 0, len(cfg.ManagedPlugins))

	installed := 0
	skipped := 0

	for _, raw := range cfg.ManagedPlugins {
		p, err := plugin.NewPlugin(raw)
		if err != nil {
			ui.Error(fmt.Sprintf("invalid plugin %q: %v", raw, err))
			os.Exit(1)
		}

		locked := lock.FindPlugin(lf, p.Name)

		// Check if already installed on disk.
		if dirExists(p.InstallPath) {
			headCommit, err := plugin.HeadCommit(p.InstallPath)
			if err == nil {
				// Dir exists and is a valid git repo.
				if locked != nil && headCommit == locked.Commit {
					ui.Info(fmt.Sprintf("%s — already installed", p.Name))
					results = append(results, result{locked: *locked, skipped: true})
					skipped++
					continue
				}
				// Dir exists but is not in lock or has a different commit — record current state.
				entry := lock.LockedPlugin{
					Name:        p.Name,
					Source:      p.Source,
					Commit:      headCommit,
					InstalledAt: time.Now().UTC().Format(time.RFC3339),
				}
				results = append(results, result{locked: entry, skipped: true})
				skipped++
				continue
			}
		}

		var s ui.Spinner

		if locked != nil {
			// Clone then checkout pinned commit.
			if dryRun {
				fmt.Printf("[dry-run] would clone %s → %s\n", p.Name, p.InstallPath)
				fmt.Printf("[dry-run] would checkout pinned commit %s\n", locked.Commit[:7])
				results = append(results, result{locked: *locked, skipped: false})
				installed++
				continue
			}

			s.Start(fmt.Sprintf("cloning %s (pinned %s)...", p.Name, locked.Commit[:7]))
			if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
				s.Stop()
				ui.Error(fmt.Sprintf("failed to clone %s: %v", p.Name, err))
				os.Exit(1)
			}
			if err := plugin.CheckoutCommit(p.InstallPath, locked.Commit); err != nil {
				s.Stop()
				ui.Error(fmt.Sprintf("failed to checkout %s in %s: %v", locked.Commit[:7], p.Name, err))
				os.Exit(1)
			}
			s.Stop()

			results = append(results, result{locked: *locked, skipped: false})
			installed++
			ui.Success(fmt.Sprintf("installed %s @ %s", p.Name, locked.Commit[:7]))

		} else {
			// Clone latest.
			if dryRun {
				fmt.Printf("[dry-run] would clone %s → %s\n", p.Name, p.InstallPath)
				fmt.Printf("[dry-run] would record HEAD commit\n")
				// Use placeholder — dry-run makes no disk writes.
				results = append(results, result{
					locked: lock.LockedPlugin{
						Name:        p.Name,
						Source:      p.Source,
						Commit:      "(would be recorded)",
						InstalledAt: time.Now().UTC().Format(time.RFC3339),
					},
					skipped: false,
				})
				installed++
				continue
			}

			s.Start(fmt.Sprintf("cloning %s...", p.Name))
			if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
				s.Stop()
				ui.Error(fmt.Sprintf("failed to clone %s: %v", p.Name, err))
				os.Exit(1)
			}
			s.Stop()

			commit, err := plugin.HeadCommit(p.InstallPath)
			if err != nil {
				ui.Error(fmt.Sprintf("failed to get HEAD commit for %s: %v", p.Name, err))
				os.Exit(1)
			}

			entry := lock.LockedPlugin{
				Name:        p.Name,
				Source:      p.Source,
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			}
			results = append(results, result{locked: entry, skipped: false})
			installed++
			ui.Success(fmt.Sprintf("installed %s @ %s", p.Name, commit[:7]))
		}
	}

	if dryRun {
		fmt.Println("[dry-run] would write lock file")
		return nil
	}

	// Ensure managed block and bootstrap line exist.
	if cfg.ManagedBlockStart == -1 {
		if err := config.AddManagedBlock(cfg, cfg.ManagedPlugins); err != nil {
			ui.Error(fmt.Sprintf("failed to add managed block: %v", err))
			os.Exit(1)
		}
	}

	// Build a new lock file from all results.
	newLF := lock.NewLockFile()
	for _, r := range results {
		if r.locked.Commit != "" && r.locked.Commit != "(would be recorded)" {
			newLF.Plugins = append(newLF.Plugins, r.locked)
		}
	}

	if err := lock.WriteLock(cfg.LockPath, newLF); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	fmt.Printf("\n%d installed, %d skipped\n", installed, skipped)
	return nil
}

// runInstallOne installs a single plugin by name, adding it to the managed
// block if it is not already present.
func runInstallOne(raw string, dryRun bool) error {
	// Validate and resolve the argument.
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
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf and re-run 'muxforge install'")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	// Check if already in managed block.
	for _, existing := range cfg.ManagedPlugins {
		if plugin.NormalizeName(existing) == p.Name {
			ui.Info(fmt.Sprintf("%s is already installed", p.Name))
			return nil
		}
	}

	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		ui.PrintError("failed to read lock file", err.Error(), "delete the lock file and run 'muxforge sync' to recreate it")
		os.Exit(1)
	}

	var commit string

	if dirExists(p.InstallPath) {
		// Directory exists but not in config — record current commit without re-cloning.
		headCommit, err := plugin.HeadCommit(p.InstallPath)
		if err != nil {
			ui.Error(fmt.Sprintf("directory %s exists but is not a valid git repo: %v", p.InstallPath, err))
			os.Exit(1)
		}
		commit = headCommit

		if dryRun {
			fmt.Printf("[dry-run] would add: set -g @plugin '%s'\n", p.Name)
			fmt.Printf("[dry-run] would record existing commit %s\n", commit[:7])
			fmt.Printf("[dry-run] would write lock file\n")
			return nil
		}

		ui.Info(fmt.Sprintf("%s already present on disk — recording existing installation", p.Name))
	} else {
		// Clone.
		if dryRun {
			fmt.Printf("[dry-run] would clone %s → %s\n", p.Name, p.InstallPath)
			fmt.Printf("[dry-run] would add: set -g @plugin '%s'\n", p.Name)
			fmt.Printf("[dry-run] would write lock file\n")
			return nil
		}

		var s ui.Spinner
		s.Start(fmt.Sprintf("cloning %s...", p.Name))
		if err := plugin.Clone(p.Source, p.InstallPath); err != nil {
			s.Stop()
			ui.Error(fmt.Sprintf("failed to clone %s: %v", p.Name, err))
			os.Exit(1)
		}
		s.Stop()

		commit, err = plugin.HeadCommit(p.InstallPath)
		if err != nil {
			ui.Error(fmt.Sprintf("failed to get HEAD commit for %s: %v", p.Name, err))
			os.Exit(1)
		}
	}

	// Add to managed block.
	newPlugins := append(cfg.ManagedPlugins, p.Name)
	if cfg.ManagedBlockStart == -1 {
		if err := config.AddManagedBlock(cfg, newPlugins); err != nil {
			ui.Error(fmt.Sprintf("failed to add managed block: %v", err))
			os.Exit(1)
		}
	} else {
		if err := config.UpdateManagedBlock(cfg, newPlugins); err != nil {
			ui.Error(fmt.Sprintf("failed to update managed block: %v", err))
			os.Exit(1)
		}
	}

	// Update lock file.
	entry := lock.LockedPlugin{
		Name:        p.Name,
		Source:      p.Source,
		Commit:      commit,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Remove any existing entry for this plugin and append the new one.
	filtered := make([]lock.LockedPlugin, 0, len(lf.Plugins))
	for _, lp := range lf.Plugins {
		if lp.Name != p.Name {
			filtered = append(filtered, lp)
		}
	}
	lf.Plugins = append(filtered, entry)

	if err := lock.WriteLock(cfg.LockPath, lf); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	ui.Success(fmt.Sprintf("installed %s @ %s → %s", p.Name, commit[:7], p.InstallPath))
	ui.Hint("run 'muxforge list' to see all installed plugins")
	return nil
}

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
