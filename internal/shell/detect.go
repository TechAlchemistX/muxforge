// Package shell detects the user's shell and manages rc file PATH updates.
package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Shell represents a supported shell type.
type Shell int

const (
	ShellUnknown Shell = iota
	ShellBash
	ShellZsh
	ShellFish
)

// DetectShell returns the Shell type based on the $SHELL environment variable.
func DetectShell() Shell {
	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "zsh"):
		return ShellZsh
	case strings.HasSuffix(shell, "bash"):
		return ShellBash
	case strings.HasSuffix(shell, "fish"):
		return ShellFish
	default:
		return ShellUnknown
	}
}

// RCFilePath returns the path to the primary rc file for the given shell.
func RCFilePath(s Shell) string {
	home, _ := os.UserHomeDir()
	switch s {
	case ShellZsh:
		return filepath.Join(home, ".zshrc")
	case ShellBash:
		// Prefer .bashrc; callers can check .bash_profile as a fallback.
		return filepath.Join(home, ".bashrc")
	case ShellFish:
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		return filepath.Join(xdg, "fish", "config.fish")
	default:
		return ""
	}
}

// AddToPath adds binDir to the PATH in the shell's rc file, but only if it
// is not already present. A comment and the appropriate PATH export line are
// prepended to the rc file's end. For unknown shells, no file is modified.
func AddToPath(s Shell, binDir string) error {
	rcPath := RCFilePath(s)
	if rcPath == "" {
		// Unknown shell — print manual instructions, modify nothing.
		fmt.Printf("Add the following line to your shell's rc file:\n")
		switch s {
		case ShellFish:
			fmt.Printf("  fish_add_path %s\n", binDir)
		default:
			fmt.Printf("  export PATH=\"$PATH:%s\"\n", binDir)
		}
		return nil
	}

	// Read the existing rc file if it exists.
	existing, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read rc file %q: %w", rcPath, err)
	}

	content := string(existing)

	// Check if binDir is already referenced to avoid duplicates.
	if strings.Contains(content, binDir) {
		return nil
	}

	// Build the line to append based on shell type.
	var appendLines string
	if s == ShellFish {
		appendLines = "\n# Added by muxforge installer\nfish_add_path " + binDir + "\n"
	} else {
		appendLines = "\n# Added by muxforge installer\nexport PATH=\"$PATH:" + binDir + "\"\n"
	}

	// Ensure the rc file directory exists.
	if err := os.MkdirAll(filepath.Dir(rcPath), 0755); err != nil {
		return fmt.Errorf("create rc file directory: %w", err)
	}

	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open rc file %q: %w", rcPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(appendLines); err != nil {
		return fmt.Errorf("write to rc file %q: %w", rcPath, err)
	}

	return nil
}
