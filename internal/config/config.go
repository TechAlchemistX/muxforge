// Package config handles tmux.conf detection, parsing, and writing.
package config

import (
	"os"
	"path/filepath"
)

const (
	// BlockStart is the marker line that begins the muxforge-managed plugin block.
	BlockStart = "# --- muxforge plugins (managed) ---"
	// BlockEnd is the marker line that ends the muxforge-managed plugin block.
	BlockEnd = "# --- end muxforge ---"
	// BootstrapLine is the line that invokes muxforge's plugin loader from tmux.
	BootstrapLine = "run 'muxforge load'"
	// BootstrapLineLegacy is the old bootstrap line used before the load subcommand
	// was introduced. Detected during parsing for backward compatibility.
	BootstrapLineLegacy = "run 'muxforge'"
	// PluginPrefix is the prefix used for plugin declarations in tmux.conf.
	PluginPrefix = "set -g @plugin '"
)

// Config represents a parsed tmux configuration file.
type Config struct {
	// Path is the absolute path to tmux.conf.
	Path string

	// LockPath is the absolute path to the lock file (same directory as config,
	// with .lock extension replacing .conf).
	LockPath string

	// Lines contains all lines from tmux.conf, preserving original content verbatim.
	Lines []string

	// ManagedBlockStart is the line index of the block start marker (-1 if absent).
	ManagedBlockStart int

	// ManagedBlockEnd is the line index of the block end marker (-1 if absent).
	ManagedBlockEnd int

	// BootstrapLineIndex is the line index of the "run 'muxforge'" line (-1 if absent).
	BootstrapLineIndex int

	// ManagedPlugins contains plugin declarations inside the managed block.
	ManagedPlugins []string

	// LegacyPlugins contains plugin declarations outside the managed block.
	LegacyPlugins []string
}

// homeDir returns the current user's home directory.
func homeDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}

// fileExists reports whether path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// PluginsDir returns the directory where plugins should be installed,
// derived from the config file path. The convention mirrors tmux/TPM:
//   - ~/.tmux.conf         → ~/.tmux/plugins/
//   - ~/.config/tmux/tmux.conf → ~/.config/tmux/plugins/
//
// In general, if the config lives directly in $HOME, plugins go in
// ~/.tmux/plugins/; otherwise they go in a "plugins" subdirectory
// alongside the config file.
func PluginsDir(configPath string) string {
	dir := filepath.Dir(configPath)
	home := homeDir()
	if dir == home {
		return filepath.Join(home, ".tmux", "plugins")
	}
	return filepath.Join(dir, "plugins")
}

// lockPathFor derives the lock file path from a config file path.
// The lock file lives in the same directory as the config, with the
// extension changed from .conf to .lock.
func lockPathFor(configPath string) string {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return filepath.Join(dir, name+".lock")
}
