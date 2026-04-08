package config

import (
	"fmt"
	"os"
	"strings"
)

// WriteConfig writes cfg.Lines back to disk using an atomic temp-file + rename
// pattern so a partial write never corrupts the config.
func WriteConfig(cfg *Config) error {
	return atomicWrite(cfg.Path, strings.Join(cfg.Lines, "\n")+"\n")
}

// AddManagedBlock appends the managed block (with the given plugins) and the
// bootstrap line to the config when no managed block currently exists.
// It updates cfg.Lines and the block boundary indices in place.
func AddManagedBlock(cfg *Config, plugins []string) error {
	if cfg.ManagedBlockStart != -1 {
		return fmt.Errorf("managed block already exists at line %d", cfg.ManagedBlockStart)
	}

	// Build the block lines.
	block := buildBlock(plugins)

	// Record where the block will start (after existing lines).
	startIdx := len(cfg.Lines)

	cfg.Lines = append(cfg.Lines, block...)
	cfg.ManagedBlockStart = startIdx
	cfg.ManagedBlockEnd = startIdx + len(block) - 1
	cfg.ManagedPlugins = plugins

	// Add bootstrap line if not already present.
	if cfg.BootstrapLineIndex == -1 {
		cfg.BootstrapLineIndex = len(cfg.Lines)
		cfg.Lines = append(cfg.Lines, BootstrapLine)
	}

	return WriteConfig(cfg)
}

// UpdateManagedBlock replaces the plugin declarations between the block
// markers with the new plugin list. Lines outside the managed block are
// never touched.
func UpdateManagedBlock(cfg *Config, plugins []string) error {
	if cfg.ManagedBlockStart == -1 || cfg.ManagedBlockEnd == -1 {
		return fmt.Errorf("no managed block found — call AddManagedBlock first")
	}

	// Build the replacement inner lines (not including the markers).
	inner := pluginLines(plugins)

	// Splice: everything before the start marker + start marker + inner +
	// end marker + everything after the end marker.
	before := cfg.Lines[:cfg.ManagedBlockStart+1] // includes BlockStart line
	after := cfg.Lines[cfg.ManagedBlockEnd:]       // includes BlockEnd line

	newLines := make([]string, 0, len(before)+len(inner)+len(after))
	newLines = append(newLines, before...)
	newLines = append(newLines, inner...)
	newLines = append(newLines, after...)

	// Update the end index to reflect the new layout.
	cfg.ManagedBlockEnd = cfg.ManagedBlockStart + 1 + len(inner)
	cfg.Lines = newLines
	cfg.ManagedPlugins = plugins

	// Re-scan for BootstrapLineIndex in case its position changed.
	// Recognise both the current and legacy bootstrap line forms.
	cfg.BootstrapLineIndex = -1
	for i, line := range cfg.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == BootstrapLine || trimmed == BootstrapLineLegacy {
			cfg.BootstrapLineIndex = i
			break
		}
	}

	return WriteConfig(cfg)
}

// buildBlock constructs the full managed block lines including markers and
// the bootstrap line.
func buildBlock(plugins []string) []string {
	lines := []string{BlockStart}
	lines = append(lines, pluginLines(plugins)...)
	lines = append(lines, BlockEnd)
	return lines
}

// pluginLines converts a slice of plugin names into set -g @plugin '...' lines.
func pluginLines(plugins []string) []string {
	lines := make([]string, len(plugins))
	for i, p := range plugins {
		lines[i] = PluginPrefix + p + "'"
	}
	return lines
}

// atomicWrite writes content to path using a temp file + rename so that
// a crash mid-write never leaves a partially-written file.
func atomicWrite(path, content string) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write temp file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// Best-effort cleanup of the temp file.
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %q → %q: %w", tmpPath, path, err)
	}
	return nil
}
