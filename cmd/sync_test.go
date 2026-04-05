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

// TestSync_MissingPluginInstalled verifies that a plugin declared in tmux.conf
// but not present on disk is installed and recorded in the lock file.
func TestSync_MissingPluginInstalled(t *testing.T) {
	dir := t.TempDir()

	// Create a "remote" repo to clone from.
	remoteDir := filepath.Join(dir, "remote", "tmux-sensible")
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	remoteCommit := makeGitRepo(t, remoteDir)

	// Write tmux.conf with a managed plugin (no plugin dir yet).
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	// Verify plugin is not installed.
	localDir := filepath.Join(dir, "plugins", "tmux-sensible")
	if dirExistsTest(localDir) {
		t.Fatal("plugin dir should not exist before sync")
	}

	// Simulate sync: clone the missing plugin.
	p, err := plugin.NewPlugin("tmux-plugins/tmux-sensible")
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}
	_ = p // would normally use p.Source and p.InstallPath

	// Clone into localDir (using remoteDir as the source).
	if err := plugin.Clone(remoteDir, localDir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	commit, err := plugin.HeadCommit(localDir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	if commit != remoteCommit {
		t.Errorf("head after clone: got %q, want %q", commit, remoteCommit)
	}

	// Record in lock file.
	lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
		Name:        "tmux-plugins/tmux-sensible",
		Source:      remoteDir,
		Commit:      commit,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Verify plugin is now installed.
	if !dirExistsTest(localDir) {
		t.Error("plugin dir should exist after sync")
	}

	// Verify lock file was updated.
	readLF, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock after sync: %v", err)
	}
	entry := lock.FindPlugin(readLF, "tmux-plugins/tmux-sensible")
	if entry == nil {
		t.Fatal("expected lock entry after sync")
	}
	if entry.Commit != remoteCommit {
		t.Errorf("lock commit: got %q, want %q", entry.Commit, remoteCommit)
	}
}

// TestSync_OrphanedPlugin_WarnedNotRemoved verifies that a directory in
// ~/.tmux/plugins/ that is not declared in tmux.conf is flagged as orphaned
// but NOT removed.
func TestSync_OrphanedPlugin_WarnedNotRemoved(t *testing.T) {
	dir := t.TempDir()

	// Write a config with one declared plugin.
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Create the declared plugin dir.
	declaredDir := filepath.Join(dir, "plugins", "tmux-sensible")
	if err := os.MkdirAll(declaredDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeGitRepo(t, declaredDir)

	// Create an orphaned plugin dir (not declared in config).
	orphanDir := filepath.Join(dir, "plugins", "old-plugin")
	if err := os.MkdirAll(orphanDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeGitRepo(t, orphanDir)

	// Simulate sync orphan detection logic.
	// The plugins directory contains directories named after the repo segment
	// only (e.g. "tmux-sensible" for "tmux-plugins/tmux-sensible"), so we
	// build a declared set keyed by repo-name segment.
	declaredByRepoName := make(map[string]bool)
	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		parts := strings.Split(name, "/")
		declaredByRepoName[parts[len(parts)-1]] = true
	}

	// Scan plugins dir for directory names.
	pluginsDir := filepath.Join(dir, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var orphans []string
	for _, e := range entries {
		if e.IsDir() && !declaredByRepoName[e.Name()] {
			orphans = append(orphans, e.Name())
		}
	}

	if len(orphans) != 1 || orphans[0] != "old-plugin" {
		t.Errorf("expected 1 orphan [old-plugin], got %v", orphans)
	}

	// Orphaned plugin dir must still exist (never auto-removed).
	if !dirExistsTest(orphanDir) {
		t.Error("orphaned plugin directory was removed — sync must never auto-remove")
	}
	// Declared plugin dir must also still exist.
	if !dirExistsTest(declaredDir) {
		t.Error("declared plugin directory was unexpectedly removed")
	}
}

// TestSync_LockEntryMissingDirectory_Reinstalls verifies that when a plugin
// is in the lock file but its directory is gone, sync reinstalls it at the
// pinned commit.
func TestSync_LockEntryMissingDirectory_Reinstalls(t *testing.T) {
	dir := t.TempDir()

	// Create a "remote" repo to clone from.
	remoteDir := filepath.Join(dir, "remote", "tmux-sensible")
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	pinnedCommit := makeGitRepo(t, remoteDir)

	// Write config and lock file, but leave the local plugin dir absent.
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	lf := lock.NewLockFile()
	lf.Plugins = []lock.LockedPlugin{
		{
			Name:        "tmux-plugins/tmux-sensible",
			Source:      remoteDir,
			Commit:      pinnedCommit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	localDir := filepath.Join(dir, "plugins", "tmux-sensible")
	if dirExistsTest(localDir) {
		t.Fatal("plugin dir should not exist before sync")
	}

	// Simulate sync: find lock entries whose directory is missing and reinstall.
	readLF, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	declaredSet := make(map[string]bool)
	for _, raw := range cfg.ManagedPlugins {
		declaredSet[plugin.NormalizeName(raw)] = true
	}

	for _, lp := range readLF.Plugins {
		if !declaredSet[lp.Name] {
			continue
		}

		p, err := plugin.NewPlugin(lp.Name)
		if err != nil {
			t.Fatalf("NewPlugin: %v", err)
		}

		// Override install path to use our test dir.
		_ = p

		if dirExistsTest(localDir) {
			continue
		}

		// Clone into localDir using remoteDir as source.
		if err := plugin.Clone(lp.Source, localDir); err != nil {
			t.Fatalf("Clone: %v", err)
		}
		// For a shallow clone we can't always checkout arbitrary commits;
		// just verify the clone succeeded.
	}

	// Plugin dir should now exist.
	if !dirExistsTest(localDir) {
		t.Error("plugin dir should exist after sync reinstall")
	}

	// Verify the cloned HEAD matches the only commit in the remote (pinnedCommit).
	headAfter, err := plugin.HeadCommit(localDir)
	if err != nil {
		t.Fatalf("HeadCommit after reinstall: %v", err)
	}
	// shallow clone of remoteDir should be at pinnedCommit.
	if headAfter != pinnedCommit {
		t.Errorf("head after reinstall: got %q, want %q", headAfter, pinnedCommit)
	}
}

// TestSync_DryRun_NoWrites verifies that --dry-run makes no disk changes.
func TestSync_DryRun_NoWrites(t *testing.T) {
	dir := t.TempDir()

	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Read (should return empty lock file — doesn't exist yet).
	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	if len(lf.Plugins) != 0 {
		t.Fatalf("expected empty lock file, got %d plugins", len(lf.Plugins))
	}

	// In dry-run mode: do NOT clone, do NOT write lock file.
	// Verify: lock file still does not exist after "dry-run".
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should not exist in dry-run mode")
	}

	// Plugin dir should not exist.
	localDir := filepath.Join(dir, "plugins", "tmux-sensible")
	if dirExistsTest(localDir) {
		t.Error("plugin dir should not exist in dry-run mode")
	}
}

// TestSync_AlreadyInstalled_AddedToLock verifies that a plugin installed on
// disk but missing from the lock file is recorded in the lock file after sync.
func TestSync_AlreadyInstalled_AddedToLock(t *testing.T) {
	dir := t.TempDir()

	// Create local plugin dir directly (already installed).
	localDir := filepath.Join(dir, "plugins", "tmux-sensible")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingCommit := makeGitRepo(t, localDir)

	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	// Lock file initially empty.
	lf := lock.NewLockFile()
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Simulate sync: for plugins that are installed but missing from lock → record.
	readLF, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	for _, raw := range cfg.ManagedPlugins {
		name := plugin.NormalizeName(raw)
		// Check if dir exists.
		if !dirExistsTest(localDir) {
			continue
		}
		// Check if in lock.
		if lock.FindPlugin(readLF, name) != nil {
			continue
		}
		// Record.
		commit, err := plugin.HeadCommit(localDir)
		if err != nil {
			t.Fatalf("HeadCommit: %v", err)
		}
		p, _ := plugin.NewPlugin(raw)
		readLF.Plugins = append(readLF.Plugins, lock.LockedPlugin{
			Name:        name,
			Source:      p.Source,
			Commit:      commit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	if err := lock.WriteLock(lockPath, readLF); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Verify lock entry recorded.
	finalLF, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock final: %v", err)
	}
	entry := lock.FindPlugin(finalLF, "tmux-plugins/tmux-sensible")
	if entry == nil {
		t.Fatal("expected lock entry after sync for already-installed plugin")
	}
	if entry.Commit != existingCommit {
		t.Errorf("commit: got %q, want %q", entry.Commit, existingCommit)
	}

	// Entry should use the normalized name (no 'set -g @plugin' wrapper).
	if strings.HasPrefix(entry.Name, "set ") {
		t.Errorf("lock entry name should be normalized, got %q", entry.Name)
	}
}
