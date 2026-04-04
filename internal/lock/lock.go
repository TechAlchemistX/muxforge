// Package lock manages the muxforge lock file that pins plugin versions.
package lock

import "sort"

// LockFile is the top-level structure of the muxforge lock file (tmux.lock).
type LockFile struct {
	Version string         `json:"version"`
	Plugins []LockedPlugin `json:"plugins"`
}

// LockedPlugin records a pinned plugin version in the lock file.
type LockedPlugin struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Commit      string `json:"commit"`
	InstalledAt string `json:"installed_at"` // RFC3339 format
}

// NewLockFile returns an initialised LockFile with Version set to "1".
func NewLockFile() *LockFile {
	return &LockFile{
		Version: "1",
		Plugins: []LockedPlugin{},
	}
}

// SortPlugins sorts the Plugins slice alphabetically by Name for stable diffs.
func SortPlugins(lf *LockFile) {
	sort.Slice(lf.Plugins, func(i, j int) bool {
		return lf.Plugins[i].Name < lf.Plugins[j].Name
	})
}
