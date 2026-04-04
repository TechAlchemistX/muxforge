package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddManagedBlock_CreatesBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")
	// Start with a config that has no managed block.
	initial := "set -g prefix C-a\nset -g mouse on\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	plugins := []string{"tmux-plugins/tmux-sensible", "tmux-plugins/tmux-resurrect"}
	if err := AddManagedBlock(cfg, plugins); err != nil {
		t.Fatalf("AddManagedBlock: %v", err)
	}

	// Re-parse to verify disk state.
	cfg2, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig after write: %v", err)
	}

	if cfg2.ManagedBlockStart == -1 {
		t.Error("ManagedBlockStart should be set after AddManagedBlock")
	}
	if cfg2.ManagedBlockEnd == -1 {
		t.Error("ManagedBlockEnd should be set after AddManagedBlock")
	}
	if cfg2.BootstrapLineIndex == -1 {
		t.Error("BootstrapLineIndex should be set after AddManagedBlock")
	}
	if len(cfg2.ManagedPlugins) != 2 {
		t.Errorf("ManagedPlugins: got %d, want 2", len(cfg2.ManagedPlugins))
	}

	// Verify original lines are preserved (first two lines untouched).
	if cfg2.Lines[0] != "set -g prefix C-a" {
		t.Errorf("Line 0 modified: got %q", cfg2.Lines[0])
	}
	if cfg2.Lines[1] != "set -g mouse on" {
		t.Errorf("Line 1 modified: got %q", cfg2.Lines[1])
	}
}

func TestUpdateManagedBlock_ReplacesPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")

	// Config with an existing managed block.
	initial := BlockStart + "\n" +
		PluginPrefix + "tmux-plugins/tmux-sensible'\n" +
		BlockEnd + "\n" +
		BootstrapLine + "\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	newPlugins := []string{
		"tmux-plugins/tmux-sensible",
		"tmux-plugins/tmux-resurrect",
		"christoomey/vim-tmux-navigator",
	}
	if err := UpdateManagedBlock(cfg, newPlugins); err != nil {
		t.Fatalf("UpdateManagedBlock: %v", err)
	}

	// Re-parse.
	cfg2, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig after update: %v", err)
	}

	if len(cfg2.ManagedPlugins) != 3 {
		t.Errorf("ManagedPlugins: got %d, want 3", len(cfg2.ManagedPlugins))
	}
	if cfg2.ManagedPlugins[2] != "christoomey/vim-tmux-navigator" {
		t.Errorf("ManagedPlugins[2]: got %q", cfg2.ManagedPlugins[2])
	}
	if cfg2.BootstrapLineIndex == -1 {
		t.Error("BootstrapLineIndex should still be present")
	}
}

func TestUpdateManagedBlock_NothingOutsideModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")

	// Config with content both before and after the block.
	initial := "set -g prefix C-a\n" +
		BlockStart + "\n" +
		PluginPrefix + "tmux-plugins/tmux-sensible'\n" +
		BlockEnd + "\n" +
		"set -g mouse on\n" +
		BootstrapLine + "\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if err := UpdateManagedBlock(cfg, []string{"tmux-plugins/tmux-resurrect"}); err != nil {
		t.Fatalf("UpdateManagedBlock: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Lines outside the block must be preserved.
	if !strings.Contains(content, "set -g prefix C-a") {
		t.Error("line before block was removed")
	}
	if !strings.Contains(content, "set -g mouse on") {
		t.Error("line after block was removed")
	}
	if !strings.Contains(content, BootstrapLine) {
		t.Error("bootstrap line was removed")
	}
	// Old plugin should be gone, new plugin present.
	if strings.Contains(content, "tmux-sensible") {
		t.Error("old plugin should have been replaced")
	}
	if !strings.Contains(content, "tmux-resurrect") {
		t.Error("new plugin should be present")
	}
}

func TestAddManagedBlock_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	plugins := []string{"tmux-plugins/tmux-sensible"}
	if err := AddManagedBlock(cfg, plugins); err != nil {
		t.Fatalf("first AddManagedBlock: %v", err)
	}

	// Second call should return an error (block already exists), not corrupt state.
	err = AddManagedBlock(cfg, plugins)
	if err == nil {
		t.Error("expected error on second AddManagedBlock call (block already exists)")
	}

	// Disk state should still be valid.
	cfg2, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig after second call: %v", err)
	}
	if len(cfg2.ManagedPlugins) != 1 {
		t.Errorf("ManagedPlugins: got %d, want 1", len(cfg2.ManagedPlugins))
	}
}

func TestWriteConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")
	content := "set -g prefix C-a\nset -g mouse on\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// After writing, lines should be the same as what we read.
	cfg2, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig after write: %v", err)
	}
	if len(cfg2.Lines) != len(cfg.Lines) {
		t.Errorf("Lines count changed: got %d, want %d", len(cfg2.Lines), len(cfg.Lines))
	}
}
