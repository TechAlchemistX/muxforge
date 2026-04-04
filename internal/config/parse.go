package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseConfig reads the tmux config file at path and returns a fully
// populated Config struct. All original lines are preserved verbatim in
// Config.Lines. Plugin declarations are classified as ManagedPlugins
// (inside the managed block) or LegacyPlugins (outside the managed block).
func ParseConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer f.Close()

	cfg := &Config{
		Path:               path,
		LockPath:           lockPathFor(path),
		ManagedBlockStart:  -1,
		ManagedBlockEnd:    -1,
		BootstrapLineIndex: -1,
	}

	scanner := bufio.NewScanner(f)
	insideBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		idx := len(cfg.Lines)
		cfg.Lines = append(cfg.Lines, line)

		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == BlockStart:
			cfg.ManagedBlockStart = idx
			insideBlock = true

		case trimmed == BlockEnd:
			cfg.ManagedBlockEnd = idx
			insideBlock = false

		case trimmed == BootstrapLine:
			cfg.BootstrapLineIndex = idx

		case strings.HasPrefix(trimmed, PluginPrefix):
			// Extract the plugin name from: set -g @plugin 'owner/repo'
			pluginName := extractPluginName(trimmed)
			if insideBlock {
				cfg.ManagedPlugins = append(cfg.ManagedPlugins, pluginName)
			} else {
				cfg.LegacyPlugins = append(cfg.LegacyPlugins, pluginName)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	return cfg, nil
}

// extractPluginName strips the set -g @plugin '...' wrapper and returns
// just the plugin identifier. For example:
//
//	"set -g @plugin 'tmux-plugins/tmux-sensible'" → "tmux-plugins/tmux-sensible"
func extractPluginName(line string) string {
	// Remove the PluginPrefix, then trim the trailing single quote.
	name := strings.TrimPrefix(line, PluginPrefix)
	name = strings.TrimSuffix(name, "'")
	return name
}
