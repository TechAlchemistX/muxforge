package lock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLock_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	content := `{
  "version": "1",
  "plugins": [
    {
      "name": "tmux-plugins/tmux-sensible",
      "source": "https://github.com/tmux-plugins/tmux-sensible",
      "commit": "25cb91f42d020f675bb0a4d3f81c1b259b951e31",
      "installed_at": "2026-03-18T10:00:00Z"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLock(path)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}

	if lf.Version != "1" {
		t.Errorf("Version: got %q, want %q", lf.Version, "1")
	}
	if len(lf.Plugins) != 1 {
		t.Fatalf("Plugins count: got %d, want 1", len(lf.Plugins))
	}
	p := lf.Plugins[0]
	if p.Name != "tmux-plugins/tmux-sensible" {
		t.Errorf("Name: got %q", p.Name)
	}
	if p.Commit != "25cb91f42d020f675bb0a4d3f81c1b259b951e31" {
		t.Errorf("Commit: got %q", p.Commit)
	}
}

func TestReadLock_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	lf, err := ReadLock(path)
	if err != nil {
		t.Fatalf("ReadLock on missing file should not error: %v", err)
	}
	if lf == nil {
		t.Fatal("expected non-nil LockFile")
	}
	if lf.Version != "1" {
		t.Errorf("Version: got %q, want %q", lf.Version, "1")
	}
	if len(lf.Plugins) != 0 {
		t.Errorf("Plugins: got %d, want 0", len(lf.Plugins))
	}
}

func TestReadLock_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.lock")

	if err := os.WriteFile(path, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadLock(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if len(err.Error()) == 0 {
		t.Error("error message should not be empty")
	}
}
