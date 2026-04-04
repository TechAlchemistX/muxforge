package plugin

import (
	"errors"
	"fmt"
	"os"
)

// ErrAlreadyGone is returned by RemovePlugin when the plugin directory does
// not exist. Callers should treat this as a warning rather than a hard error.
var ErrAlreadyGone = errors.New("plugin directory not found")

// RemovePlugin deletes the plugin directory at installPath. If the directory
// is already absent, ErrAlreadyGone is returned so the caller can print a
// warning without treating it as a fatal error.
func RemovePlugin(installPath string) error {
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return ErrAlreadyGone
	}

	if err := os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("remove plugin at %q: %w", installPath, err)
	}
	return nil
}
