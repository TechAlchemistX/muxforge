package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/lock"
	"github.com/TechAlchemistX/muxforge/internal/plugin"
	"github.com/TechAlchemistX/muxforge/internal/ui"
	"github.com/spf13/cobra"
)

// tpmBootstrapPatterns lists known TPM run-line variants for exact matching.
// isTpmBootstrap also performs a content-based fallback for other variants.
var tpmBootstrapPatterns = []string{
	"run '~/.tmux/plugins/tpm/tpm'",
	`run "~/.tmux/plugins/tpm/tpm"`,
	"run-shell '~/.tmux/plugins/tpm/tpm'",
	`run-shell "~/.tmux/plugins/tpm/tpm"`,
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate an existing TPM setup to muxforge",
		Long: `Migrate converts a TPM-style tmux.conf to muxforge management.
It moves all @plugin declarations into the muxforge managed block, replaces
the TPM bootstrap line with the muxforge bootstrap line, and creates a
lock file recording current commit hashes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate()
		},
	}
	return cmd
}

func runMigrate() error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		ui.PrintError("no tmux config found", err.Error(), "create a tmux.conf and re-run 'muxforge migrate'")
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		ui.PrintError("failed to parse config", err.Error(), "check that your tmux.conf is valid")
		os.Exit(1)
	}

	// Check whether any TPM bootstrap line is still present in the config.
	// This can happen if a previous migrate attempt was incomplete, or if
	// muxforge install created the managed block before migrate was run.
	tpmLinePresent := false
	for _, line := range cfg.Lines {
		if isTpmBootstrap(strings.TrimSpace(line)) {
			tpmLinePresent = true
			break
		}
	}

	// Already migrated: managed block with plugins, no legacy plugins, and no
	// TPM remnants. If a TPM line is still present we fall through to clean it up.
	if cfg.ManagedBlockStart != -1 && len(cfg.ManagedPlugins) > 0 &&
		len(cfg.LegacyPlugins) == 0 && !tpmLinePresent {
		ui.Info("already using muxforge — no migration needed")
		return nil
	}

	// Nothing to do: no plugins at all.
	if len(cfg.ManagedPlugins) == 0 && len(cfg.LegacyPlugins) == 0 {
		ui.Info("no plugins found — nothing to migrate")
		ui.Hint("run 'muxforge install owner/repo' to add a plugin")
		return nil
	}

	// Consolidate all plugins: managed first, then legacy, preserving order.
	// Deduplicate by name (managed takes precedence). Exclude tpm itself — it
	// is a plugin manager, not a user plugin, and muxforge has replaced it.
	seen := make(map[string]bool)
	allPlugins := make([]string, 0, len(cfg.ManagedPlugins)+len(cfg.LegacyPlugins))

	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] && name != "tmux-plugins/tpm" {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}
	for _, raw := range cfg.LegacyPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] && name != "tmux-plugins/tpm" {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}

	// Build new Lines:
	// 1. Remove all legacy plugin lines, TPM bootstrap line, and old managed block.
	// 2. Insert new managed block at the position of the first removed item
	//    (or at the end if nothing was removed).
	// 3. Add muxforge bootstrap if not already present.

	newLines, insertAt := buildMigratedLines(cfg, allPlugins)
	_ = insertAt

	cfg.Lines = newLines

	// Reparse to get fresh indices.
	rewritten := strings.Join(cfg.Lines, "\n") + "\n"
	tmpPath := cfg.Path + ".migrate-tmp"
	if err := os.WriteFile(tmpPath, []byte(rewritten), 0644); err != nil {
		ui.Error(fmt.Sprintf("failed to write temp config: %v", err))
		os.Exit(1)
	}
	newCfg, err := config.ParseConfig(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		ui.Error(fmt.Sprintf("failed to re-parse migrated config: %v", err))
		os.Exit(1)
	}
	_ = os.Remove(tmpPath)

	// Write the final config.
	newCfg.Path = cfg.Path
	newCfg.LockPath = cfg.LockPath
	if err := config.WriteConfig(newCfg); err != nil {
		ui.Error(fmt.Sprintf("failed to write config: %v", err))
		os.Exit(1)
	}

	// Build the lock file.
	lf := lock.NewLockFile()
	installed := 0
	skipped := 0

	for _, name := range allPlugins {
		p, err := plugin.NewPlugin(name)
		if err != nil {
			ui.Warning(fmt.Sprintf("skipping invalid plugin %q: %v", name, err))
			continue
		}

		if dirExists(p.InstallPath) {
			// Already installed — record current commit, do NOT re-clone.
			commit, err := plugin.HeadCommit(p.InstallPath)
			if err != nil {
				ui.Warning(fmt.Sprintf("cannot read HEAD for %s: %v — skipping lock entry", p.Name, err))
				continue
			}
			lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
				Name:        p.Name,
				Source:      p.Source,
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			})
			skipped++
		} else {
			// Not installed — clone.
			var s ui.Spinner
			s.Start(fmt.Sprintf("cloning %s...", p.Name))
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
			lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
				Name:        p.Name,
				Source:      p.Source,
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			})
			installed++
		}
	}

	if err := lock.WriteLock(cfg.LockPath, lf); err != nil {
		ui.Error(fmt.Sprintf("failed to write lock file: %v", err))
		os.Exit(1)
	}

	// Remove the tpm directory — it is no longer needed now that muxforge
	// handles both plugin management and loading.
	tpmRemoved := false
	home, _ := os.UserHomeDir()
	tpmDir := filepath.Join(home, ".tmux", "plugins", "tpm")
	if dirExists(tpmDir) {
		if err := os.RemoveAll(tpmDir); err != nil {
			ui.Warning(fmt.Sprintf("could not remove tpm directory: %v", err))
		} else {
			ui.Success("removed ~/.tmux/plugins/tpm — replaced by muxforge")
			tpmRemoved = true
		}
	}

	msg := fmt.Sprintf(
		"migration complete — %d plugin(s) moved to managed block, %d installed, %d already on disk",
		len(allPlugins), installed, skipped,
	)
	if tpmRemoved {
		msg += ", tpm removed"
	}
	ui.Success(msg)
	ui.Hint("reload tmux with: tmux source-file " + cfg.Path)

	return nil
}

// buildMigratedLines constructs the new Lines slice for the config after
// migration. It:
//   - Removes all legacy @plugin lines
//   - Removes the old managed block (markers + inner lines)
//   - Removes any TPM bootstrap line
//   - Inserts the new managed block (with all plugins) at the position of the
//     first item that was removed (or appended at end if nothing was removed)
//   - Appends the muxforge bootstrap line if absent
func buildMigratedLines(cfg *config.Config, allPlugins []string) ([]string, int) {
	// Identify line indices to remove.
	removeSet := make(map[int]bool)

	// Remove managed block lines (start marker through end marker inclusive).
	if cfg.ManagedBlockStart != -1 && cfg.ManagedBlockEnd != -1 {
		for i := cfg.ManagedBlockStart; i <= cfg.ManagedBlockEnd; i++ {
			removeSet[i] = true
		}
	}

	// Remove legacy @plugin lines outside the managed block.
	for i, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, config.PluginPrefix) {
			// Only remove if outside the managed block.
			inBlock := cfg.ManagedBlockStart != -1 &&
				i > cfg.ManagedBlockStart && i < cfg.ManagedBlockEnd
			if !inBlock {
				removeSet[i] = true
			}
		}
	}

	// Remove TPM bootstrap line.
	for i, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		if isTpmBootstrap(trimmed) {
			removeSet[i] = true
		}
	}

	// Remove any muxforge bootstrap line (current or legacy form) — re-added at the end.
	for i, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == config.BootstrapLine || trimmed == config.BootstrapLineLegacy {
			removeSet[i] = true
		}
	}

	// Determine insert position: position of first removed line, or end.
	insertAt := len(cfg.Lines)
	for i := range cfg.Lines {
		if removeSet[i] {
			insertAt = i
			break
		}
	}

	// Build the new managed block lines.
	blockLines := buildManagedBlockLines(allPlugins)

	// Assemble: keep non-removed lines, inserting the block at insertAt.
	result := make([]string, 0, len(cfg.Lines)+len(blockLines)+1)
	inserted := false

	for i, line := range cfg.Lines {
		if removeSet[i] {
			continue
		}
		if !inserted && i >= insertAt {
			result = append(result, blockLines...)
			inserted = true
		}
		result = append(result, line)
	}
	if !inserted {
		result = append(result, blockLines...)
	}

	// Append muxforge bootstrap line.
	result = append(result, config.BootstrapLine)

	return result, insertAt
}

// buildManagedBlockLines returns the lines for the muxforge managed block
// including start marker, plugin declarations, and end marker.
func buildManagedBlockLines(plugins []string) []string {
	lines := make([]string, 0, len(plugins)+2)
	lines = append(lines, config.BlockStart)
	for _, name := range plugins {
		lines = append(lines, config.PluginPrefix+name+"'")
	}
	lines = append(lines, config.BlockEnd)
	return lines
}

// isTpmBootstrap reports whether a (trimmed) config line is a TPM run line.
func isTpmBootstrap(trimmed string) bool {
	for _, pat := range tpmBootstrapPatterns {
		if trimmed == pat {
			return true
		}
	}
	// Also handle trailing spaces after closing quote/bracket variants.
	trimmedFurther := strings.TrimRight(trimmed, " \t")
	for _, pat := range tpmBootstrapPatterns {
		if trimmedFurther == pat {
			return true
		}
	}
	return false
}
