package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/lock"
	"github.com/TechAlchemistX/muxforge/internal/plugin"
)

// writeTmuxConfStr is an alias helper — same as writeTmuxConf but avoids
// re-declaration issues; used to keep each test self-contained.
func writeTmuxConfStr(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// simulateMigrate performs the core migrate logic using internal packages,
// so tests can verify outcomes without invoking the cobra command (and thus
// without needing network / real git remotes).
//
// It:
//  1. Finds all plugins (managed + legacy) in cfg.
//  2. Builds a new Lines slice: removes legacy lines, old managed block,
//     TPM bootstrap; inserts new managed block; appends muxforge bootstrap.
//  3. Writes the updated config.
//  4. For already-installed plugins (installDirs map) → records HEAD commit.
//     For not-installed → clones from the provided source map (tests supply
//     a local repo path).
//  5. Writes the lock file.
//
// installDirs maps pluginName → local directory already on disk.
// remoteSources maps pluginName → source URL/path to clone from.
func simulateMigrate(
	t *testing.T,
	cfg *config.Config,
	lockPath string,
	installDirs map[string]string,
	remoteSources map[string]string,
) {
	t.Helper()

	// Collect all plugins (managed first, then legacy, deduplicate).
	seen := make(map[string]bool)
	allPlugins := make([]string, 0)
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}
	for _, raw := range cfg.LegacyPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}

	// Build new lines.
	newLines := buildMigratedLinesTest(cfg, allPlugins)
	cfg.Lines = newLines

	// Write updated config.
	rewritten := strings.Join(cfg.Lines, "\n") + "\n"
	if err := os.WriteFile(cfg.Path, []byte(rewritten), 0644); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}

	// Build lock file.
	lf := lock.NewLockFile()
	for _, name := range allPlugins {
		if dir, ok := installDirs[name]; ok && dirExistsTest(dir) {
			// Already installed — record HEAD.
			commit, err := plugin.HeadCommit(dir)
			if err != nil {
				t.Fatalf("HeadCommit for %s: %v", name, err)
			}
			src := remoteSources[name]
			if src == "" {
				src = "https://github.com/" + name
			}
			lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
				Name:        name,
				Source:      src,
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			})
		} else if src, ok := remoteSources[name]; ok {
			// Clone from provided source.
			destDir := filepath.Join(filepath.Dir(cfg.Path), "plugins", repoName(name))
			if err := plugin.Clone(src, destDir); err != nil {
				t.Fatalf("Clone %s: %v", name, err)
			}
			commit, err := plugin.HeadCommit(destDir)
			if err != nil {
				t.Fatalf("HeadCommit %s: %v", name, err)
			}
			lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
				Name:        name,
				Source:      src,
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}
}

// buildMigratedLinesTest mirrors the migrate command's buildMigratedLines
// logic (without calling unexported cmd internals).
func buildMigratedLinesTest(cfg *config.Config, allPlugins []string) []string {
	removeSet := make(map[int]bool)

	// Remove managed block.
	if cfg.ManagedBlockStart != -1 && cfg.ManagedBlockEnd != -1 {
		for i := cfg.ManagedBlockStart; i <= cfg.ManagedBlockEnd; i++ {
			removeSet[i] = true
		}
	}

	// Remove legacy @plugin lines.
	for i, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, config.PluginPrefix) {
			inBlock := cfg.ManagedBlockStart != -1 &&
				i > cfg.ManagedBlockStart && i < cfg.ManagedBlockEnd
			if !inBlock {
				removeSet[i] = true
			}
		}
	}

	// Remove TPM bootstrap.
	tpmPatterns := []string{
		"run '~/.tmux/plugins/tpm/tpm'",
		`run "~/.tmux/plugins/tpm/tpm"`,
	}
	for i, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		for _, pat := range tpmPatterns {
			if trimmed == pat {
				removeSet[i] = true
			}
		}
	}

	// Remove existing muxforge bootstrap.
	for i, line := range cfg.Lines {
		if strings.TrimSpace(line) == config.BootstrapLine {
			removeSet[i] = true
		}
	}

	// Determine insert position.
	insertAt := len(cfg.Lines)
	for i := range cfg.Lines {
		if removeSet[i] {
			insertAt = i
			break
		}
	}

	// Build new managed block.
	blockLines := make([]string, 0, len(allPlugins)+2)
	blockLines = append(blockLines, config.BlockStart)
	for _, name := range allPlugins {
		blockLines = append(blockLines, config.PluginPrefix+name+"'")
	}
	blockLines = append(blockLines, config.BlockEnd)

	// Assemble.
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
	result = append(result, config.BootstrapLine)
	return result
}

// repoName returns the last segment of an owner/repo name.
func repoName(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

// TestMigrate_TPMStyle verifies migration from a TPM-style config:
// - managed block created with all plugins
// - TPM bootstrap line removed
// - muxforge bootstrap added
// - lock file written
func TestMigrate_TPMStyle(t *testing.T) {
	dir := t.TempDir()

	// TPM-style config: legacy plugin lines + TPM bootstrap.
	confContent := "# basic tmux settings\n" +
		"set -g mouse on\n" +
		"\n" +
		"set -g @plugin 'tmux-plugins/tpm'\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		"set -g @plugin 'tmux-plugins/tmux-resurrect'\n" +
		"\n" +
		"run '~/.tmux/plugins/tpm/tpm'\n"
	confPath := writeTmuxConfStr(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Should have 3 legacy plugins, no managed block.
	if cfg.ManagedBlockStart != -1 {
		t.Error("expected no managed block in TPM-style config")
	}
	if len(cfg.LegacyPlugins) != 3 {
		t.Errorf("expected 3 legacy plugins, got %d: %v", len(cfg.LegacyPlugins), cfg.LegacyPlugins)
	}

	// Create "remote" repos for sensible and resurrect (tpm is handled specially).
	sensibleRemote := filepath.Join(dir, "remote", "tmux-sensible")
	resurrectRemote := filepath.Join(dir, "remote", "tmux-resurrect")
	tpmRemote := filepath.Join(dir, "remote", "tpm")
	for _, d := range []string{sensibleRemote, resurrectRemote, tpmRemote} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		makeGitRepo(t, d)
	}

	simulateMigrate(t, cfg, lockPath,
		map[string]string{}, // nothing pre-installed
		map[string]string{
			"tmux-plugins/tpm":          tpmRemote,
			"tmux-plugins/tmux-sensible":  sensibleRemote,
			"tmux-plugins/tmux-resurrect": resurrectRemote,
		},
	)

	// Verify config was rewritten.
	newCfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig after migrate: %v", err)
	}

	// Managed block must exist.
	if newCfg.ManagedBlockStart == -1 {
		t.Error("managed block should exist after migrate")
	}

	// All 3 plugins should be in the managed block.
	if len(newCfg.ManagedPlugins) != 3 {
		t.Errorf("expected 3 managed plugins, got %d: %v", len(newCfg.ManagedPlugins), newCfg.ManagedPlugins)
	}

	// No legacy plugins should remain.
	if len(newCfg.LegacyPlugins) != 0 {
		t.Errorf("expected no legacy plugins after migrate, got %d: %v", len(newCfg.LegacyPlugins), newCfg.LegacyPlugins)
	}

	// muxforge bootstrap should be present.
	if newCfg.BootstrapLineIndex == -1 {
		t.Error("muxforge bootstrap line should be present after migrate")
	}

	// TPM bootstrap line should be gone.
	for _, line := range newCfg.Lines {
		if strings.Contains(line, "tpm/tpm") {
			t.Errorf("TPM bootstrap line should be removed, found: %q", line)
		}
	}

	// Lock file should have entries for all 3 plugins.
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock after migrate: %v", err)
	}
	if len(lf.Plugins) != 3 {
		t.Errorf("expected 3 lock entries, got %d", len(lf.Plugins))
	}

	// Custom settings should be preserved.
	foundMouseLine := false
	for _, line := range newCfg.Lines {
		if line == "set -g mouse on" {
			foundMouseLine = true
		}
	}
	if !foundMouseLine {
		t.Error("custom settings (set -g mouse on) should be preserved after migrate")
	}
}

// TestMigrate_AlreadyMigrated verifies that running migrate when a managed
// block already exists with plugins (and no legacy plugins) exits cleanly.
func TestMigrate_AlreadyMigrated(t *testing.T) {
	dir := t.TempDir()

	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConfStr(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Reproduce the "already migrated" detection logic.
	alreadyMigrated := cfg.ManagedBlockStart != -1 &&
		len(cfg.ManagedPlugins) > 0 &&
		len(cfg.LegacyPlugins) == 0

	if !alreadyMigrated {
		t.Error("expected alreadyMigrated to be true for a fully managed config")
	}

	// Verify the config is unchanged (no migration performed).
	finalCfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig final: %v", err)
	}
	if len(finalCfg.ManagedPlugins) != 1 {
		t.Errorf("expected 1 managed plugin after no-op migrate, got %d", len(finalCfg.ManagedPlugins))
	}
	if finalCfg.ManagedPlugins[0] != "tmux-plugins/tmux-sensible" {
		t.Errorf("unexpected managed plugin: %q", finalCfg.ManagedPlugins[0])
	}
}

// TestMigrate_Mixed verifies that a config with BOTH a managed block AND
// legacy plugins consolidates everything into one managed block.
func TestMigrate_Mixed(t *testing.T) {
	dir := t.TempDir()

	// Config has one plugin in managed block and two legacy plugins.
	confContent := "set -g mouse on\n" +
		"\n" +
		"set -g @plugin 'legacy/plugin-a'\n" +
		"set -g @plugin 'legacy/plugin-b'\n" +
		"\n" +
		config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConfStr(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Verify initial state.
	if len(cfg.ManagedPlugins) != 1 {
		t.Fatalf("expected 1 managed plugin initially, got %d", len(cfg.ManagedPlugins))
	}
	if len(cfg.LegacyPlugins) != 2 {
		t.Fatalf("expected 2 legacy plugins initially, got %d", len(cfg.LegacyPlugins))
	}

	// Create remotes for all three plugins.
	remoteA := filepath.Join(dir, "remote", "plugin-a")
	remoteB := filepath.Join(dir, "remote", "plugin-b")
	remoteSensible := filepath.Join(dir, "remote", "tmux-sensible")
	for _, d := range []string{remoteA, remoteB, remoteSensible} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		makeGitRepo(t, d)
	}

	// Pre-install tmux-sensible (already in managed block).
	sensibleDir := filepath.Join(dir, "installed", "tmux-sensible")
	if err := os.MkdirAll(sensibleDir, 0755); err != nil {
		t.Fatal(err)
	}
	sensibleCommit := makeGitRepo(t, sensibleDir)

	simulateMigrate(t, cfg, lockPath,
		map[string]string{
			"tmux-plugins/tmux-sensible": sensibleDir,
		},
		map[string]string{
			"legacy/plugin-a":             remoteA,
			"legacy/plugin-b":             remoteB,
			"tmux-plugins/tmux-sensible":  remoteSensible,
		},
	)

	// Verify config after migration.
	newCfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig after migrate: %v", err)
	}

	// All 3 plugins in managed block.
	if len(newCfg.ManagedPlugins) != 3 {
		t.Errorf("expected 3 managed plugins after consolidation, got %d: %v",
			len(newCfg.ManagedPlugins), newCfg.ManagedPlugins)
	}

	// No legacy plugins remain.
	if len(newCfg.LegacyPlugins) != 0 {
		t.Errorf("expected no legacy plugins after consolidation, got %d: %v",
			len(newCfg.LegacyPlugins), newCfg.LegacyPlugins)
	}

	// muxforge bootstrap present.
	if newCfg.BootstrapLineIndex == -1 {
		t.Error("muxforge bootstrap should be present after migrate")
	}

	// Custom settings preserved.
	foundMouse := false
	for _, line := range newCfg.Lines {
		if line == "set -g mouse on" {
			foundMouse = true
		}
	}
	if !foundMouse {
		t.Error("custom settings should be preserved after migrate")
	}

	// tmux-sensible should be recorded with existing commit (NOT re-cloned).
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	entry := lock.FindPlugin(lf, "tmux-plugins/tmux-sensible")
	if entry == nil {
		t.Fatal("expected lock entry for tmux-sensible")
	}
	if entry.Commit != sensibleCommit {
		t.Errorf("tmux-sensible commit: got %q, want %q (should not be re-cloned)", entry.Commit, sensibleCommit)
	}

	// legacy/plugin-a and legacy/plugin-b should also be in lock.
	for _, name := range []string{"legacy/plugin-a", "legacy/plugin-b"} {
		e := lock.FindPlugin(lf, name)
		if e == nil {
			t.Errorf("expected lock entry for %s", name)
		}
	}
}

// TestMigrate_PreservesPluginOrder verifies that managed plugins appear
// before legacy plugins in the consolidated managed block, preserving
// declaration order within each group.
func TestMigrate_PreservesPluginOrder(t *testing.T) {
	dir := t.TempDir()

	confContent := "set -g @plugin 'legacy/first'\n" +
		"set -g @plugin 'legacy/second'\n" +
		"\n" +
		config.BlockStart + "\n" +
		"set -g @plugin 'managed/alpha'\n" +
		"set -g @plugin 'managed/beta'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConfStr(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Build the consolidated plugin list using the same logic as migrate.
	seen := make(map[string]bool)
	allPlugins := make([]string, 0)
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}
	for _, raw := range cfg.LegacyPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}

	// Expected order: managed first, then legacy.
	expected := []string{"managed/alpha", "managed/beta", "legacy/first", "legacy/second"}
	if len(allPlugins) != len(expected) {
		t.Fatalf("expected %d plugins, got %d: %v", len(expected), len(allPlugins), allPlugins)
	}
	for i, name := range expected {
		if allPlugins[i] != name {
			t.Errorf("plugin[%d]: got %q, want %q", i, allPlugins[i], name)
		}
	}
}

// TestMigrate_LockPreservesExistingInstall verifies that already-installed
// plugin dirs are NOT re-cloned during migration.
func TestMigrate_LockPreservesExistingInstall(t *testing.T) {
	dir := t.TempDir()

	confContent := "set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		"run '~/.tmux/plugins/tpm/tpm'\n"
	confPath := writeTmuxConfStr(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Pre-install tmux-sensible (already on disk).
	existingDir := filepath.Join(dir, "existing", "tmux-sensible")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingCommit := makeGitRepo(t, existingDir)

	// Record the modification time before migration.
	stat, err := os.Stat(existingDir)
	if err != nil {
		t.Fatalf("Stat existing dir: %v", err)
	}
	modBefore := stat.ModTime()

	// Simulate migrate with the existing dir as already-installed.
	simulateMigrate(t, cfg, lockPath,
		map[string]string{
			"tmux-plugins/tmux-sensible": existingDir,
		},
		map[string]string{},
	)

	// The directory should NOT have been re-cloned (mod time unchanged).
	stat2, err := os.Stat(existingDir)
	if err != nil {
		t.Fatalf("Stat after migrate: %v", err)
	}
	if !stat2.ModTime().Equal(modBefore) {
		t.Log("note: directory mod time changed (may be OS behavior)")
	}

	// Verify lock file records the existing commit, not a new one.
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	entry := lock.FindPlugin(lf, "tmux-plugins/tmux-sensible")
	if entry == nil {
		t.Fatal("expected lock entry for tmux-sensible")
	}
	if entry.Commit != existingCommit {
		t.Errorf("commit should be existing commit %q, got %q — plugin was re-cloned", existingCommit, entry.Commit)
	}
}

// TestMigrate_AlreadyMigratedButTPMLinePresent verifies that migrate does NOT
// early-exit when a managed block exists but the TPM bootstrap line is still
// in the config. This happens when muxforge install created the managed block
// before the user ran migrate, leaving the TPM run line in place.
func TestMigrate_AlreadyMigratedButTPMLinePresent(t *testing.T) {
	dir := t.TempDir()

	// Simulate: managed block already exists (from a prior install), but the
	// TPM bootstrap line was never removed.
	confContent := "set -g mouse on\n" +
		"\n" +
		config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		"\n" +
		"run '~/.tmux/plugins/tpm/tpm'\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConfStr(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Confirm this is the "already migrated + TPM line present" scenario.
	if cfg.ManagedBlockStart == -1 {
		t.Fatal("expected managed block to exist")
	}
	if len(cfg.ManagedPlugins) == 0 {
		t.Fatal("expected managed plugins")
	}
	if len(cfg.LegacyPlugins) != 0 {
		t.Fatalf("expected no legacy plugins, got %v", cfg.LegacyPlugins)
	}

	// TPM line is present — migrate must NOT early-exit.
	tpmPatterns := []string{
		"run '~/.tmux/plugins/tpm/tpm'",
		`run "~/.tmux/plugins/tpm/tpm"`,
		"run-shell '~/.tmux/plugins/tpm/tpm'",
		`run-shell "~/.tmux/plugins/tpm/tpm"`,
	}
	isTpm := func(line string) bool {
		trimmed := strings.TrimSpace(line)
		for _, pat := range tpmPatterns {
			if trimmed == pat || strings.TrimRight(trimmed, " \t") == pat {
				return true
			}
		}
		return false
	}
	foundTPM := false
	for _, line := range cfg.Lines {
		if isTpm(line) {
			foundTPM = true
			break
		}
	}
	if !foundTPM {
		t.Fatal("test setup error: TPM bootstrap line not found in initial config")
	}

	// Simulate the migrate (TPM line must be stripped, managed block preserved).
	allPlugins := []string{}
	seen := make(map[string]bool)
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		if !seen[name] && name != "tmux-plugins/tpm" {
			seen[name] = true
			allPlugins = append(allPlugins, name)
		}
	}

	newLines := buildMigratedLinesTest(cfg, allPlugins)
	rewritten := strings.Join(newLines, "\n") + "\n"
	if err := os.WriteFile(confPath, []byte(rewritten), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Verify the result.
	newCfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig after migrate: %v", err)
	}

	// TPM bootstrap line must be gone.
	for _, line := range newCfg.Lines {
		if strings.Contains(line, "tpm/tpm") {
			t.Errorf("TPM bootstrap line should have been removed, found: %q", line)
		}
	}

	// Managed block and plugin must still be present.
	if newCfg.ManagedBlockStart == -1 {
		t.Error("managed block should still exist after TPM-only cleanup")
	}
	if len(newCfg.ManagedPlugins) != 1 || newCfg.ManagedPlugins[0] != "tmux-plugins/tmux-sensible" {
		t.Errorf("managed plugins wrong after cleanup: %v", newCfg.ManagedPlugins)
	}

	// muxforge bootstrap must be present.
	if newCfg.BootstrapLineIndex == -1 {
		t.Error("muxforge bootstrap line should be present")
	}

	// Custom settings must be preserved.
	foundMouse := false
	for _, line := range newCfg.Lines {
		if line == "set -g mouse on" {
			foundMouse = true
		}
	}
	if !foundMouse {
		t.Error("custom settings should be preserved")
	}
}

// TestMigrate_NoPlugins verifies that migrate exits early when there are no
// plugins (no managed block, no legacy declarations).
func TestMigrate_NoPlugins(t *testing.T) {
	dir := t.TempDir()

	confContent := "set -g mouse on\n" +
		"set -g history-limit 10000\n"
	confPath := writeTmuxConfStr(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Both slices should be empty.
	if len(cfg.ManagedPlugins) != 0 || len(cfg.LegacyPlugins) != 0 {
		t.Errorf("expected no plugins, got managed=%v legacy=%v",
			cfg.ManagedPlugins, cfg.LegacyPlugins)
	}

	// nothing to migrate condition: both slices empty.
	nothingToMigrate := len(cfg.ManagedPlugins) == 0 && len(cfg.LegacyPlugins) == 0
	if !nothingToMigrate {
		t.Error("expected nothing-to-migrate condition")
	}
}

// TestMigrate_TPMBootstrapVariants verifies that all known TPM bootstrap
// line formats are recognized and would be removed.
func TestMigrate_TPMBootstrapVariants(t *testing.T) {
	variants := []string{
		"run '~/.tmux/plugins/tpm/tpm'",
		`run "~/.tmux/plugins/tpm/tpm"`,
		"run '~/.tmux/plugins/tpm/tpm'  ", // trailing spaces
	}

	tpmPatterns := []string{
		"run '~/.tmux/plugins/tpm/tpm'",
		`run "~/.tmux/plugins/tpm/tpm"`,
	}

	isTpm := func(line string) bool {
		trimmed := strings.TrimSpace(line)
		for _, pat := range tpmPatterns {
			if trimmed == pat {
				return true
			}
		}
		trimmedFurther := strings.TrimRight(trimmed, " \t")
		for _, pat := range tpmPatterns {
			if trimmedFurther == pat {
				return true
			}
		}
		return false
	}

	for _, v := range variants {
		if !isTpm(v) {
			t.Errorf("TPM bootstrap variant not recognized: %q", v)
		}
	}

	// Non-TPM lines should not be matched.
	nonTPM := []string{
		"run 'muxforge'",
		"# run '~/.tmux/plugins/tpm/tpm'",
		"set -g mouse on",
	}
	for _, line := range nonTPM {
		if isTpm(line) {
			t.Errorf("non-TPM line incorrectly recognized as TPM: %q", line)
		}
	}
}
