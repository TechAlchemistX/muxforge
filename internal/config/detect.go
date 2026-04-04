package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindConfig locates the tmux config file using tmux's own lookup order.
// It returns the absolute path to the config file, or a descriptive error
// if no config file is found. It never creates files.
//
// Lookup order:
//  1. $TMUX_CONFIG environment variable
//  2. $XDG_CONFIG_HOME/tmux/tmux.conf (defaults to ~/.config/tmux/tmux.conf)
//  3. ~/.tmux.conf (legacy fallback)
func FindConfig() (string, error) {
	// 1. Check $TMUX_CONFIG env var.
	if cfg := os.Getenv("TMUX_CONFIG"); cfg != "" {
		if fileExists(cfg) {
			return cfg, nil
		}
		// Env var set but file missing — still report not found rather than silently
		// falling through, so the user knows their override is broken.
		return "", fmt.Errorf(
			"tmux config not found at $TMUX_CONFIG path %q — create the file or unset the variable",
			cfg,
		)
	}

	// 2. Check $XDG_CONFIG_HOME/tmux/tmux.conf.
	xdgBase := os.Getenv("XDG_CONFIG_HOME")
	if xdgBase == "" {
		xdgBase = filepath.Join(homeDir(), ".config")
	}
	xdgPath := filepath.Join(xdgBase, "tmux", "tmux.conf")
	if fileExists(xdgPath) {
		return xdgPath, nil
	}

	// 3. Check ~/.tmux.conf (legacy).
	legacyPath := filepath.Join(homeDir(), ".tmux.conf")
	if fileExists(legacyPath) {
		return legacyPath, nil
	}

	return "", fmt.Errorf(
		"no tmux config found — create one at %s or %s",
		xdgPath, legacyPath,
	)
}
