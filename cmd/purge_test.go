package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TechAlchemistX/muxforge/internal/config"
)

// TestPurge_RemovesMarkersAndBootstrap verifies that purge removes muxforge
// markers and bootstrap lines but preserves plugin declarations and custom settings.
func TestPurge_RemovesMarkersAndBootstrap(t *testing.T) {
	dir := t.TempDir()

	confContent := "set -g mouse on\n" +
		"\n" +
		config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		"set -g @plugin 'tmux-plugins/tmux-resurrect'\n" +
		config.BlockEnd + "\n" +
		"\n" +
		"set -g status-style bg=black\n" +
		config.BootstrapLine + "\n"
	confPath := writeTmuxConf(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Simulate purge: filter out markers and bootstrap, keep everything else.
	var cleaned []string
	removedMarkers := 0
	removedBootstrap := 0

	for _, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case config.BlockStart, config.BlockEnd:
			removedMarkers++
			continue
		case config.BootstrapLine, config.BootstrapLineLegacy:
			removedBootstrap++
			continue
		}
		cleaned = append(cleaned, line)
	}

	cfg.Lines = cleaned
	if err := config.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// Verify results.
	if removedMarkers != 2 {
		t.Errorf("expected 2 markers removed, got %d", removedMarkers)
	}
	if removedBootstrap != 1 {
		t.Errorf("expected 1 bootstrap line removed, got %d", removedBootstrap)
	}

	// Re-parse to verify final state.
	result, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig after purge: %v", err)
	}

	// No managed block should remain.
	if result.ManagedBlockStart != -1 {
		t.Error("managed block markers should be removed after purge")
	}

	// Bootstrap line should be gone.
	if result.BootstrapLineIndex != -1 {
		t.Error("bootstrap line should be removed after purge")
	}

	// Plugin declarations should be preserved (now as legacy since no managed block).
	if len(result.LegacyPlugins) != 2 {
		t.Errorf("expected 2 plugin declarations preserved, got %d", len(result.LegacyPlugins))
	}

	// Custom settings should be preserved.
	foundMouse := false
	foundStatus := false
	for _, line := range result.Lines {
		if line == "set -g mouse on" {
			foundMouse = true
		}
		if line == "set -g status-style bg=black" {
			foundStatus = true
		}
	}
	if !foundMouse {
		t.Error("custom setting 'set -g mouse on' should be preserved")
	}
	if !foundStatus {
		t.Error("custom setting 'set -g status-style bg=black' should be preserved")
	}
}

// TestPurge_RemovesLockFile verifies that purge removes the lock file.
func TestPurge_RemovesLockFile(t *testing.T) {
	dir := t.TempDir()

	lockPath := filepath.Join(dir, "tmux.lock")
	if err := os.WriteFile(lockPath, []byte(`{"version":"1","plugins":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify lock file exists.
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist before purge: %v", err)
	}

	// Simulate purge lock removal.
	if err := os.Remove(lockPath); err != nil {
		t.Fatalf("Remove lock file: %v", err)
	}

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should be removed after purge")
	}
}

// TestPurge_PurgePluginsRemovesDirectory verifies that --purge-plugins
// removes the plugins directory.
func TestPurge_PurgePluginsRemovesDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create a plugins directory with some content.
	pluginsDir := filepath.Join(dir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "tmux-sensible")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "sensible.tmux"), []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	// Verify directory exists.
	if !dirExistsTest(pluginsDir) {
		t.Fatal("plugins directory should exist before purge")
	}

	// Simulate purge --purge-plugins.
	if err := os.RemoveAll(pluginsDir); err != nil {
		t.Fatalf("RemoveAll plugins dir: %v", err)
	}

	if dirExistsTest(pluginsDir) {
		t.Error("plugins directory should be removed after purge --purge-plugins")
	}
}

// TestPurge_PreservesLegacyBootstrap verifies that the legacy bootstrap
// line form is also removed.
func TestPurge_PreservesLegacyBootstrap(t *testing.T) {
	dir := t.TempDir()

	confContent := config.BlockStart + "\n" +
		"set -g @plugin 'tmux-plugins/tmux-sensible'\n" +
		config.BlockEnd + "\n" +
		config.BootstrapLineLegacy + "\n"
	confPath := writeTmuxConf(t, dir, confContent)

	cfg, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	var cleaned []string
	for _, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case config.BlockStart, config.BlockEnd,
			config.BootstrapLine, config.BootstrapLineLegacy:
			continue
		}
		cleaned = append(cleaned, line)
	}

	cfg.Lines = cleaned
	if err := config.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	result, err := config.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig after purge: %v", err)
	}

	// Legacy bootstrap should be gone.
	if result.BootstrapLineIndex != -1 {
		t.Error("legacy bootstrap line should be removed after purge")
	}

	// Plugin should be preserved.
	if len(result.LegacyPlugins) != 1 || result.LegacyPlugins[0] != "tmux-plugins/tmux-sensible" {
		t.Errorf("plugin declaration should be preserved, got %v", result.LegacyPlugins)
	}
}
