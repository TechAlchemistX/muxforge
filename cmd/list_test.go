package cmd_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/lock"
)

// TestList_StatusIndicators verifies that the data conditions for each of the
// three status indicators can be derived correctly from config and lock file
// state, without requiring a real terminal output assertion.
//
// Status mapping:
//   - installed + in lock file  → "up to date"   (green ✓)
//   - installed + not in lock   → "not in lock"  (yellow !)
//   - not installed + in config → "not installed" (red ✗)
func TestList_StatusIndicators(t *testing.T) {
	dir := t.TempDir()

	// Plugin 1: installed on disk and in lock file.
	p1Dir := filepath.Join(dir, "plugins", "tmux-sensible")
	if err := os.MkdirAll(p1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	p1Commit := makeGitRepo(t, p1Dir)

	// Plugin 2: installed on disk but NOT in lock file.
	p2Dir := filepath.Join(dir, "plugins", "tmux-resurrect")
	if err := os.MkdirAll(p2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	// p2 is a valid dir; for the test we just check dirExists.

	// Plugin 3: in config but NOT installed on disk.
	// (no directory created for p3)

	// Write tmux.conf with all three plugins.
	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		"set -g @plugin 'tmux-plugins/tmux-resurrect'\n" +
		"set -g @plugin 'christoomey/vim-tmux-navigator'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)
	lockPath := filepath.Join(dir, "tmux.lock")

	// Write lock file with only plugin 1.
	lf := lock.NewLockFile()
	lf.Plugins = []lock.LockedPlugin{
		{
			Name:        "tmux-plugins/tmux-sensible",
			Source:      "https://github.com/tmux-plugins/tmux-sensible",
			Commit:      p1Commit,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := lock.WriteLock(lockPath, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Parse config and read lock.
	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	readLF, err := lock.ReadLock(lockPath)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	type pluginStatus struct {
		name      string
		installed bool
		inLock    bool
	}

	// Compute status for each plugin using the same conditions as runList.
	pluginDirs := map[string]string{
		"tmux-plugins/tmux-sensible":       p1Dir,
		"tmux-plugins/tmux-resurrect":      p2Dir,
		"christoomey/vim-tmux-navigator":   filepath.Join(dir, "plugins", "vim-tmux-navigator"), // does not exist
	}

	statuses := make([]pluginStatus, 0, len(cfg.ManagedPlugins))
	for _, raw := range cfg.ManagedPlugins {
		d := pluginDirs[raw]
		installed := dirExistsTest(d)
		locked := lock.FindPlugin(readLF, raw)
		statuses = append(statuses, pluginStatus{
			name:      raw,
			installed: installed,
			inLock:    locked != nil,
		})
	}

	tests := []struct {
		idx       int
		name      string
		installed bool
		inLock    bool
	}{
		{0, "tmux-plugins/tmux-sensible", true, true},
		{1, "tmux-plugins/tmux-resurrect", true, false},
		{2, "christoomey/vim-tmux-navigator", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := statuses[tt.idx]
			if s.name != tt.name {
				t.Errorf("name: got %q, want %q", s.name, tt.name)
			}
			if s.installed != tt.installed {
				t.Errorf("installed: got %v, want %v", s.installed, tt.installed)
			}
			if s.inLock != tt.inLock {
				t.Errorf("inLock: got %v, want %v", s.inLock, tt.inLock)
			}
		})
	}
}

// TestList_ShortCommit verifies the 7-char short commit truncation helper.
func TestList_ShortCommit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc1234defg5678hijk9012lmno3456pqrs7890", "abc1234"},
		{"abc1234", "abc1234"},
		{"ab123", "ab123"}, // shorter than 7 — returned as-is
		{"", ""},
	}
	for _, tt := range tests {
		got := shortCommitTest(tt.input)
		if got != tt.want {
			t.Errorf("shortCommit(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestList_TildeAbbrev verifies path abbreviation with home directory.
func TestList_TildeAbbrev(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	path := filepath.Join(home, ".config", "tmux", "tmux.conf")
	abbrev := tildeAbbrevTest(path)
	if abbrev != "~/.config/tmux/tmux.conf" {
		t.Errorf("tildeAbbrev(%q) = %q, want %q", path, abbrev, "~/.config/tmux/tmux.conf")
	}

	// Non-home path should be returned unchanged.
	nonHome := "/etc/tmux.conf"
	if got := tildeAbbrevTest(nonHome); got != nonHome {
		t.Errorf("tildeAbbrev(%q) = %q, want %q", nonHome, got, nonHome)
	}
}

// Helpers that mirror the cmd package private helpers for testing.
// These replicate the logic rather than importing unexported symbols.

func shortCommitTest(commit string) string {
	if len(commit) >= 7 {
		return commit[:7]
	}
	return commit
}

func tildeAbbrevTest(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if len(path) >= len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}

func dirExistsTest(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
