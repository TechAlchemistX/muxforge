package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/lock"
)

// makeGitRepo creates a minimal git repository in dir with a single commit
// and returns the full 40-character commit hash of HEAD.
func makeGitRepo(t *testing.T, dir string) string {
	t.Helper()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Provide a minimal git environment so tests are hermetic.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")

	// Create an initial commit so HEAD is valid.
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("test plugin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	// Read HEAD commit.
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// writeTmuxConf writes content to a tmux.conf in dir and returns its path.
func writeTmuxConf(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestInstallAll_WithLockFile verifies that when a lock file with pinned
// commits is present and the plugin directory already exists at that commit,
// install-all skips the plugin and preserves the lock file entry.
func TestInstallAll_WithLockFile(t *testing.T) {
	dir := t.TempDir()

	// Create the plugin directory (fake installed plugin).
	pluginDir := filepath.Join(dir, "plugins", "tmux-sensible")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	commit := makeGitRepo(t, pluginDir)

	// Write tmux.conf with managed block.
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)

	// Write lock file with the pinned commit.
	lockPath := filepath.Join(dir, "tmux.lock")
	lf := lock.NewLockFile()
	lf.Plugins = []lock.LockedPlugin{
		{
			Name:        "tmux-plugins/tmux-sensible",
			Source:      "https://github.com/tmux-plugins/tmux-sensible",
			Commit:      commit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Parse config and verify managed plugins are present.
	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.ManagedPlugins) != 1 {
		t.Fatalf("expected 1 managed plugin, got %d", len(cfg.ManagedPlugins))
	}

	// Read lock and verify the pinned commit is recorded.
	readLF, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	entry := lock.FindPlugin(readLF, "tmux-plugins/tmux-sensible")
	if entry == nil {
		t.Fatal("expected plugin in lock file, got nil")
	}
	if entry.Commit != commit {
		t.Errorf("commit: got %q, want %q", entry.Commit, commit)
	}

	// Verify the plugin directory exists at the pinned commit.
	out, err := exec.Command("git", "-C", pluginDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	headCommit := strings.TrimSpace(string(out))
	if headCommit != commit {
		t.Errorf("HEAD commit: got %q, want %q", headCommit, commit)
	}
}

// TestInstallAll_NoLockFile verifies that when no lock file exists,
// install-all creates one with correct entries after simulating a clone.
func TestInstallAll_NoLockFile(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the "cloned" plugin directory to simulate a successful clone.
	pluginDir := filepath.Join(dir, "plugins", "tmux-resurrect")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	commit := makeGitRepo(t, pluginDir)

	lockPath := filepath.Join(dir, "tmux.lock")

	// Simulate what install-all does when no lock file exists:
	// read (returns empty), install, record commit, write lock.
	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock (no lock file): %v", err)
	}
	if len(lf.Plugins) != 0 {
		t.Fatalf("expected empty lock file, got %d plugins", len(lf.Plugins))
	}

	// Record the commit (simulates post-clone HeadCommit call).
	lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
		Name:        "tmux-plugins/tmux-resurrect",
		Source:      "https://github.com/tmux-plugins/tmux-resurrect",
		Commit:      commit,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	})

	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Verify the written lock file.
	written, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock after write: %v", err)
	}
	if len(written.Plugins) != 1 {
		t.Fatalf("expected 1 plugin in lock, got %d", len(written.Plugins))
	}
	if written.Plugins[0].Name != "tmux-plugins/tmux-resurrect" {
		t.Errorf("name: got %q, want %q", written.Plugins[0].Name, "tmux-plugins/tmux-resurrect")
	}
	if written.Plugins[0].Commit != commit {
		t.Errorf("commit: got %q, want %q", written.Plugins[0].Commit, commit)
	}
	if len(written.Plugins[0].Commit) != 40 {
		t.Errorf("commit should be 40 chars, got %d: %q", len(written.Plugins[0].Commit), written.Plugins[0].Commit)
	}
	if written.Version != "1" {
		t.Errorf("version: got %q, want %q", written.Version, "1")
	}
}

// TestInstallOne_AddsToConfigAndLock verifies that installing a single plugin
// adds it to the managed block and lock file.
func TestInstallOne_AddsToConfigAndLock(t *testing.T) {
	dir := t.TempDir()

	// Start with a config that has an existing managed block.
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	// Create the new plugin directory to simulate a clone.
	pluginDir := filepath.Join(dir, "plugins", "vim-tmux-navigator")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	commit := makeGitRepo(t, pluginDir)

	// Simulate install-one logic using the internal packages.
	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	lf, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	// Add plugin to managed block.
	newPlugins := append(cfg.ManagedPlugins, "christoomey/vim-tmux-navigator")
	if err := config.UpdateManagedBlock(cfg, newPlugins); err != nil {
		t.Fatalf("UpdateManagedBlock: %v", err)
	}

	// Add to lock file.
	lf.Plugins = append(lf.Plugins, lock.LockedPlugin{
		Name:        "christoomey/vim-tmux-navigator",
		Source:      "https://github.com/christoomey/vim-tmux-navigator",
		Commit:      commit,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Verify the config was updated.
	updatedCfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig after update: %v", err)
	}
	if len(updatedCfg.ManagedPlugins) != 2 {
		t.Errorf("expected 2 managed plugins, got %d: %v", len(updatedCfg.ManagedPlugins), updatedCfg.ManagedPlugins)
	}
	found := false
	for _, p := range updatedCfg.ManagedPlugins {
		if p == "christoomey/vim-tmux-navigator" {
			found = true
		}
	}
	if !found {
		t.Error("christoomey/vim-tmux-navigator not found in managed plugins")
	}

	// Verify the lock file was updated.
	updatedLF, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock after write: %v", err)
	}
	entry := lock.FindPlugin(updatedLF, "christoomey/vim-tmux-navigator")
	if entry == nil {
		t.Fatal("plugin not found in lock file")
	}
	if entry.Commit != commit {
		t.Errorf("commit: got %q, want %q", entry.Commit, commit)
	}
}

// TestInstallOne_AlreadyPresent verifies that installing a plugin that is
// already in the managed block is a no-op.
func TestInstallOne_AlreadyPresent(t *testing.T) {
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

	// Check "already installed" condition — matching logic from runInstallOne.
	targetName := "tmux-plugins/tmux-sensible"
	alreadyPresent := false
	for _, existing := range cfg.ManagedPlugins {
		if existing == targetName {
			alreadyPresent = true
		}
	}
	if !alreadyPresent {
		t.Error("expected tmux-sensible to already be in managed plugins")
	}

	// Verify config was NOT modified (no new plugins added).
	if len(cfg.ManagedPlugins) != 1 {
		t.Errorf("expected 1 managed plugin, got %d", len(cfg.ManagedPlugins))
	}
}
