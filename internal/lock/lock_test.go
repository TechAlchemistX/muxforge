package lock

import (
	"testing"
)

func TestNewLockFile(t *testing.T) {
	lf := NewLockFile()
	if lf.Version != "1" {
		t.Errorf("Version: got %q, want %q", lf.Version, "1")
	}
	if lf.Plugins == nil {
		t.Error("Plugins should be initialised (not nil)")
	}
	if len(lf.Plugins) != 0 {
		t.Errorf("Plugins: got %d, want 0", len(lf.Plugins))
	}
}

func TestSortPlugins(t *testing.T) {
	lf := &LockFile{
		Version: "1",
		Plugins: []LockedPlugin{
			{Name: "z-owner/z-repo"},
			{Name: "a-owner/a-repo"},
			{Name: "m-owner/m-repo"},
		},
	}

	SortPlugins(lf)

	if lf.Plugins[0].Name != "a-owner/a-repo" {
		t.Errorf("[0]: got %q, want %q", lf.Plugins[0].Name, "a-owner/a-repo")
	}
	if lf.Plugins[1].Name != "m-owner/m-repo" {
		t.Errorf("[1]: got %q, want %q", lf.Plugins[1].Name, "m-owner/m-repo")
	}
	if lf.Plugins[2].Name != "z-owner/z-repo" {
		t.Errorf("[2]: got %q, want %q", lf.Plugins[2].Name, "z-owner/z-repo")
	}
}

func TestSortPlugins_AlreadySorted(t *testing.T) {
	lf := &LockFile{
		Plugins: []LockedPlugin{
			{Name: "a/a"},
			{Name: "b/b"},
		},
	}
	SortPlugins(lf)
	if lf.Plugins[0].Name != "a/a" || lf.Plugins[1].Name != "b/b" {
		t.Error("already-sorted slice should remain unchanged")
	}
}

func TestFindPlugin(t *testing.T) {
	lf := &LockFile{
		Plugins: []LockedPlugin{
			{Name: "tmux-plugins/tmux-sensible", Commit: "abc123"},
			{Name: "tmux-plugins/tmux-resurrect", Commit: "def456"},
		},
	}

	t.Run("found", func(t *testing.T) {
		p := FindPlugin(lf, "tmux-plugins/tmux-sensible")
		if p == nil {
			t.Fatal("expected non-nil result")
		}
		if p.Commit != "abc123" {
			t.Errorf("Commit: got %q, want %q", p.Commit, "abc123")
		}
	})

	t.Run("not found", func(t *testing.T) {
		p := FindPlugin(lf, "owner/nonexistent")
		if p != nil {
			t.Errorf("expected nil, got %+v", p)
		}
	})

	t.Run("empty lock file", func(t *testing.T) {
		empty := NewLockFile()
		p := FindPlugin(empty, "owner/repo")
		if p != nil {
			t.Errorf("expected nil, got %+v", p)
		}
	})
}
