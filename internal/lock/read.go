package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ReadLock reads the lock file at path and returns the parsed LockFile.
// If the file does not exist, ReadLock returns a new empty LockFile with no
// error — this is the expected state on first run.
// If the file exists but contains malformed JSON, a descriptive error is returned.
func ReadLock(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewLockFile(), nil
		}
		return nil, fmt.Errorf("read lock file %q: %w", path, err)
	}

	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse lock file %q: %w — delete the file and run 'muxforge sync' to recreate it", path, err)
	}

	return &lf, nil
}

// FindPlugin returns the LockedPlugin entry with the given name, or nil if
// no entry with that name exists in the lock file.
func FindPlugin(lf *LockFile, name string) *LockedPlugin {
	for i := range lf.Plugins {
		if lf.Plugins[i].Name == name {
			return &lf.Plugins[i]
		}
	}
	return nil
}
