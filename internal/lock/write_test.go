package lock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteLock_PrettyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	lf := &LockFile{
		Version: "1",
		Plugins: []LockedPlugin{
			{
				Name:        "tmux-plugins/tmux-sensible",
				Source:      "https://github.com/tmux-plugins/tmux-sensible",
				Commit:      "25cb91f42d020f675bb0a4d3f81c1b259b951e31",
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	if err := WriteLock(path, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}

	content := string(data)

	// Must use 2-space indent.
	if !strings.Contains(content, "  \"version\"") {
		t.Error("output should be indented with 2 spaces")
	}
}

func TestWriteLock_SortsAlphabetically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	lf := &LockFile{
		Version: "1",
		Plugins: []LockedPlugin{
			{Name: "z-org/z-plugin", Source: "https://github.com/z-org/z-plugin", Commit: strings.Repeat("z", 40), InstalledAt: time.Now().UTC().Format(time.RFC3339)},
			{Name: "a-org/a-plugin", Source: "https://github.com/a-org/a-plugin", Commit: strings.Repeat("a", 40), InstalledAt: time.Now().UTC().Format(time.RFC3339)},
		},
	}

	if err := WriteLock(path, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// Re-read and verify order.
	lf2, err := ReadLock(path)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	if lf2.Plugins[0].Name != "a-org/a-plugin" {
		t.Errorf("expected a-org/a-plugin first, got %q", lf2.Plugins[0].Name)
	}
	if lf2.Plugins[1].Name != "z-org/z-plugin" {
		t.Errorf("expected z-org/z-plugin second, got %q", lf2.Plugins[1].Name)
	}
}

func TestWriteLock_NoTmpFileRemaining(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	lf := NewLockFile()
	if err := WriteLock(path, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	// The .tmp file must not be present after a successful write.
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf(".tmp file should not remain after successful WriteLock")
	}
}

func TestWriteLock_RFC3339InstalledAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	ts := "2026-03-18T10:00:00Z"
	lf := &LockFile{
		Version: "1",
		Plugins: []LockedPlugin{
			{
				Name:        "owner/repo",
				Source:      "https://github.com/owner/repo",
				Commit:      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
				InstalledAt: ts,
			},
		},
	}

	if err := WriteLock(path, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var out LockFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err = time.Parse(time.RFC3339, out.Plugins[0].InstalledAt)
	if err != nil {
		t.Errorf("InstalledAt %q is not RFC3339: %v", out.Plugins[0].InstalledAt, err)
	}
}

func TestWriteLock_40CharCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	commit := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	lf := &LockFile{
		Version: "1",
		Plugins: []LockedPlugin{
			{
				Name:        "owner/repo",
				Source:      "https://github.com/owner/repo",
				Commit:      commit,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	if err := WriteLock(path, lf); err != nil {
		t.Fatalf("WriteLock: %v", err)
	}

	lf2, err := ReadLock(path)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	if got := lf2.Plugins[0].Commit; len(got) != 40 {
		t.Errorf("Commit length: got %d, want 40 (commit=%q)", len(got), got)
	}
}
