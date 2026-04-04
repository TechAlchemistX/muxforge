package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSource converts any supported plugin input form to a full HTTPS URL.
//
// Supported input forms:
//   - "owner/repo"                    → https://github.com/owner/repo
//   - "https://github.com/owner/repo" → unchanged
//   - "github.com/owner/repo"         → https://github.com/owner/repo
func ResolveSource(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	if strings.HasPrefix(raw, "https://") {
		return raw, nil
	}

	if strings.HasPrefix(raw, "github.com/") {
		return "https://" + raw, nil
	}

	// Assume owner/repo shorthand.
	parts := strings.Split(raw, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return fmt.Sprintf("https://github.com/%s/%s", parts[0], parts[1]), nil
	}

	return "", fmt.Errorf(
		"cannot resolve plugin source from %q — use 'owner/repo' or a full HTTPS URL",
		raw,
	)
}

// InstallPath returns the local installation path for a plugin.
// The path is always ~/.tmux/plugins/<repo-name>.
func InstallPath(raw string) string {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "/")
	repoName := parts[len(parts)-1]
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tmux", "plugins", repoName)
}

// NormalizeName converts any supported input form to the canonical
// "owner/repo" format.
func NormalizeName(raw string) string {
	raw = strings.TrimSpace(raw)

	// Strip https:// or github.com/ prefix.
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "github.com/")

	// raw is now "owner/repo" (possibly with trailing slashes or .git).
	raw = strings.TrimSuffix(raw, ".git")
	raw = strings.Trim(raw, "/")

	return raw
}
