package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mandeep/muxforge/internal/config"
	"github.com/mandeep/muxforge/internal/lock"
	"github.com/mandeep/muxforge/internal/plugin"
)

// setupRemoveFixture creates a tmux.conf with two plugins and a lock file,
// optionally also creating the plugin directory on disk.
// Returns the config path, lock path, and the plugin directory path.
func setupRemoveFixture(t *testing.T, createDir bool) (cfgPath, lockPath, pluginDir string) {
	t.Helper()
	dir := t.TempDir()

	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		"set -g @plugin 'tmux-plugins/tmux-resurrect'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	cfgPath = writeTmuxConf(t, dir, confContent)
	lockPath = filepath.Join(dir, "tmux.lock")
	pluginDir = filepath.Join(dir, "plugins", "tmux-resurrect")

	if createDir {
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}
		makeGitRepo(t, pluginDir)
	}

	commit := strings.Repeat("a", 40)
	lf := lock.NewLockFile()
	lf.Plugins = []lock.LockedPlugin{
		{
			Name:        "tmux-plugins/tmux-sensible",
			Source:      "https://github.com/tmux-plugins/tmux-sensible",
			Commit:      commit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		},
		{
			Name:        "tmux-plugins/tmux-resurrect",
			Source:      "https://github.com/tmux-plugins/tmux-resurrect",
			Commit:      commit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatal(err)
	}

	return cfgPath, lockPath, pluginDir
}

// simulateRemove mimics the remove command's core logic using internal packages.
// It removes the target plugin from the config, deletes installDir if
// it exists (tolerating the "already gone" case), and updates the lock file.
// installDir is the actual directory to remove (test-controlled path).
func simulateRemove(t *testing.T, cfgPath, lockPath, targetName, installDir string) (dirGone bool) {
	t.Helper()

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Build new plugin list without the target.
	newPlugins := make([]string, 0, len(cfg.ManagedPlugins))
	for _, raw := range cfg.ManagedPlugins {
		if plugin.NormalizeName(raw) != targetName {
			newPlugins = append(newPlugins, raw)
		}
	}
	if err := config.UpdateManagedBlock(cfg, newPlugins); err != nil {
		t.Fatalf("UpdateManagedBlock: %v", err)
	}

	// Remove directory using the test-provided path.
	if err := plugin.RemovePlugin(installDir); err != nil {
		if errors.Is(err, plugin.ErrAlreadyGone) {
			dirGone = true
		} else {
			t.Fatalf("RemovePlugin: %v", err)
		}
	}

	// Update lock file.
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	filtered := make([]lock.LockedPlugin, 0, len(lf.Plugins))
	for _, lp := range lf.Plugins {
		if lp.Name != targetName {
			filtered = append(filtered, lp)
		}
	}
	lf.Plugins = filtered
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	return dirGone
}

// TestRemove_RemovesFromConfigLockAndDirectory verifies the happy path:
// plugin is removed from config, its directory is deleted, and it is removed
// from the lock file.
func TestRemove_RemovesFromConfigLockAndDirectory(t *testing.T) {
	cfgPath, lockPath, pluginDir := setupRemoveFixture(t, true /* createDir */)

	// Verify preconditions.
	if _, err := os.Stat(pluginDir); err != nil {
		t.Fatalf("plugin dir should exist before remove: %v", err)
	}

	simulateRemove(t, cfgPath, lockPath, "tmux-plugins/tmux-resurrect", pluginDir)

	// Plugin directory should be gone.
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("expected plugin directory to be deleted after remove")
	}

	// Config should no longer contain the removed plugin.
	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		t.Fatalf("ParseConfig after remove: %v", err)
	}
	for _, p := range cfg.ManagedPlugins {
		if p == "tmux-plugins/tmux-resurrect" {
			t.Error("removed plugin still present in managed block")
		}
	}
	// The other plugin should still be there.
	found := false
	for _, p := range cfg.ManagedPlugins {
		if p == "tmux-plugins/tmux-sensible" {
			found = true
		}
	}
	if !found {
		t.Error("tmux-sensible should still be in managed block after removing tmux-resurrect")
	}

	// Lock file should no longer contain the removed plugin.
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock after remove: %v", err)
	}
	if entry := lock.FindPlugin(lf, "tmux-plugins/tmux-resurrect"); entry != nil {
		t.Error("removed plugin still present in lock file")
	}
	if entry := lock.FindPlugin(lf, "tmux-plugins/tmux-sensible"); entry == nil {
		t.Error("tmux-sensible should still be in lock file")
	}
}

// TestRemove_PluginNotFound verifies that attempting to remove a plugin that
// is not in the managed block produces the expected not-found condition.
func TestRemove_PluginNotFound(t *testing.T) {
	dir := t.TempDir()

	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Simulate the match loop from runRemove.
	targetArg := "tmux-plugins/nonexistent"
	var matches []string
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		repo := repoSegmentTest(name)
		argNorm := plugin.NormalizeName(targetArg)
		argRepo := repoSegmentTest(argNorm)
		if name == argNorm || repo == argRepo {
			matches = append(matches, name)
		}
	}

	if len(matches) != 0 {
		t.Errorf("expected no matches for %q, got %v", targetArg, matches)
	}
}

// TestRemove_DirectoryAlreadyGone verifies that when the plugin directory has
// been manually deleted, the remove still cleans up config and lock file,
// and ErrAlreadyGone is returned by RemovePlugin.
func TestRemove_DirectoryAlreadyGone(t *testing.T) {
	cfgPath, lockPath, _ := setupRemoveFixture(t, false /* createDir — not created */)

	// Simulate remove — the plugin directory does not exist.
	// Use a path that definitely does not exist.
	dir := filepath.Dir(filepath.Dir(cfgPath))
	nonExistentDir := filepath.Join(dir, "plugins", "tmux-resurrect")
	dirGone := simulateRemove(t, cfgPath, lockPath, "tmux-plugins/tmux-resurrect", nonExistentDir)

	if !dirGone {
		t.Error("expected ErrAlreadyGone when directory does not exist")
	}

	// Config should still be cleaned up.
	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		t.Fatalf("ParseConfig after remove: %v", err)
	}
	for _, p := range cfg.ManagedPlugins {
		if p == "tmux-plugins/tmux-resurrect" {
			t.Error("removed plugin still present in managed block despite missing directory")
		}
	}

	// Lock file should still be cleaned up.
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock after remove: %v", err)
	}
	if entry := lock.FindPlugin(lf, "tmux-plugins/tmux-resurrect"); entry != nil {
		t.Error("removed plugin still present in lock file despite missing directory")
	}
}

// TestRemove_AmbiguousMatch verifies that a partial name that matches multiple
// plugins results in an ambiguous match condition (multiple results).
func TestRemove_AmbiguousMatch(t *testing.T) {
	dir := t.TempDir()

	// Two plugins with the same repo segment but different owners.
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'owner-a/tmux-shared'\n" +
		"set -g @plugin 'owner-b/tmux-shared'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Match by partial repo name — should hit both.
	argRepo := "tmux-shared"
	var matches []string
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		repo := repoSegmentTest(name)
		if repo == argRepo {
			matches = append(matches, name)
		}
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches for ambiguous name %q, got %d: %v", argRepo, len(matches), matches)
	}
}

// repoSegmentTest mirrors the cmd.repoSegment logic for use in tests.
func repoSegmentTest(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}
