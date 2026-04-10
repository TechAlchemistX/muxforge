// Package plugin provides types and operations for tmux plugins.
package plugin

import (
	"fmt"
	"strings"
)

// Plugin represents a tmux plugin with its resolved metadata.
type Plugin struct {
	// Raw is the declaration as it appears in tmux.conf.
	Raw string

	// Name is the normalized "owner/repo" identifier.
	Name string

	// Source is the full HTTPS URL for cloning.
	Source string

	// InstallPath is the local path where the plugin is installed.
	InstallPath string
}

// NewPlugin constructs a Plugin from the raw declaration string, resolving
// the source URL and install path. pluginsDir is the directory where plugins
// are stored (use config.PluginsDir to derive it from the config path).
// Returns an error if the source cannot be resolved.
func NewPlugin(raw, pluginsDir string) (*Plugin, error) {
	source, err := ResolveSource(raw)
	if err != nil {
		return nil, fmt.Errorf("new plugin %q: %w", raw, err)
	}

	return &Plugin{
		Raw:         raw,
		Name:        NormalizeName(raw),
		Source:      source,
		InstallPath: InstallPath(raw, pluginsDir),
	}, nil
}

// ShortName returns only the repository name (last path segment) of the plugin.
func ShortName(p *Plugin) string {
	parts := strings.Split(p.Name, "/")
	return parts[len(parts)-1]
}
