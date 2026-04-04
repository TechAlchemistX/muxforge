package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Clone clones a git repository from source into destPath using a shallow
// clone (--depth=1) for speed.
func Clone(source, destPath string) error {
	cmd := exec.Command("git", "clone", "--depth=1", source, destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clone %q into %q: %w", source, destPath, err)
	}
	return nil
}

// Pull fetches the latest changes for the repository at pluginPath.
func Pull(pluginPath string) error {
	cmd := exec.Command("git", "-C", pluginPath, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull in %q: %w", pluginPath, err)
	}
	return nil
}

// HeadCommit returns the full 40-character SHA1 hash of HEAD for the
// repository at pluginPath.
func HeadCommit(pluginPath string) (string, error) {
	cmd := exec.Command("git", "-C", pluginPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse HEAD in %q: %w", pluginPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CheckoutCommit checks out a specific commit in the repository at pluginPath.
// Because muxforge uses shallow clones, it first unshallows the repository so
// that arbitrary commits are reachable.
func CheckoutCommit(pluginPath, commit string) error {
	// Unshallow so the target commit is available.
	unshallow := exec.Command("git", "-C", pluginPath, "fetch", "--unshallow")
	unshallow.Stdout = os.Stdout
	unshallow.Stderr = os.Stderr
	if err := unshallow.Run(); err != nil {
		return fmt.Errorf("fetch --unshallow in %q: %w", pluginPath, err)
	}

	checkout := exec.Command("git", "-C", pluginPath, "checkout", commit)
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("checkout %q in %q: %w", commit, pluginPath, err)
	}
	return nil
}
