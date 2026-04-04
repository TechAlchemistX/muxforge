package lock

import (
	"encoding/json"
	"fmt"
	"os"
)

// WriteLock serialises lf to disk at path using an atomic temp-file + rename
// pattern. Plugins are sorted alphabetically before writing so the output is
// stable for diffs.
func WriteLock(path string, lf *LockFile) error {
	SortPlugins(lf)

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock file: %w", err)
	}
	// Append a trailing newline for POSIX compliance.
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write lock file %q: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename lock file %q → %q: %w", tmpPath, path, err)
	}

	return nil
}
