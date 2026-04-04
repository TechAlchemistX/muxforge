package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixturesDir returns the absolute path to the testdata/configs directory.
func fixturesDir(t *testing.T) string {
	t.Helper()
	// Walk up from this file to the module root.
	_, file, _, _ := runtime.Caller(0)
	// file is .../internal/config/parse_test.go
	moduleRoot := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(moduleRoot, "testdata", "configs")
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join(fixturesDir(t), name)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	dst := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write fixture copy %q: %v", dst, err)
	}
	return dst
}

func TestParseConfig_XDGStyle(t *testing.T) {
	path := copyFixture(t, "xdg-style.conf")
	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.ManagedBlockStart != 0 {
		t.Errorf("ManagedBlockStart: got %d, want 0", cfg.ManagedBlockStart)
	}
	if cfg.ManagedBlockEnd != 4 {
		t.Errorf("ManagedBlockEnd: got %d, want 4", cfg.ManagedBlockEnd)
	}
	if cfg.BootstrapLineIndex != 5 {
		t.Errorf("BootstrapLineIndex: got %d, want 5", cfg.BootstrapLineIndex)
	}
	if len(cfg.ManagedPlugins) != 3 {
		t.Errorf("ManagedPlugins count: got %d, want 3", len(cfg.ManagedPlugins))
	}
	if len(cfg.LegacyPlugins) != 0 {
		t.Errorf("LegacyPlugins count: got %d, want 0", len(cfg.LegacyPlugins))
	}
	if cfg.ManagedPlugins[0] != "tmux-plugins/tmux-sensible" {
		t.Errorf("ManagedPlugins[0]: got %q, want %q", cfg.ManagedPlugins[0], "tmux-plugins/tmux-sensible")
	}
}

func TestParseConfig_TPMStyle(t *testing.T) {
	path := copyFixture(t, "tpm-style.conf")
	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.ManagedBlockStart != -1 {
		t.Errorf("ManagedBlockStart: got %d, want -1", cfg.ManagedBlockStart)
	}
	if cfg.ManagedBlockEnd != -1 {
		t.Errorf("ManagedBlockEnd: got %d, want -1", cfg.ManagedBlockEnd)
	}
	if cfg.BootstrapLineIndex != -1 {
		t.Errorf("BootstrapLineIndex: got %d, want -1 (TPM bootstrap is not muxforge)", cfg.BootstrapLineIndex)
	}
	if len(cfg.LegacyPlugins) != 2 {
		t.Errorf("LegacyPlugins count: got %d, want 2", len(cfg.LegacyPlugins))
	}
	if len(cfg.ManagedPlugins) != 0 {
		t.Errorf("ManagedPlugins count: got %d, want 0", len(cfg.ManagedPlugins))
	}
}

func TestParseConfig_LegacyStyle(t *testing.T) {
	path := copyFixture(t, "legacy-style.conf")
	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.ManagedBlockStart != -1 {
		t.Errorf("ManagedBlockStart: got %d, want -1", cfg.ManagedBlockStart)
	}
	if cfg.ManagedBlockEnd != -1 {
		t.Errorf("ManagedBlockEnd: got %d, want -1", cfg.ManagedBlockEnd)
	}
	if cfg.BootstrapLineIndex != -1 {
		t.Errorf("BootstrapLineIndex: got %d, want -1", cfg.BootstrapLineIndex)
	}
	if len(cfg.LegacyPlugins) != 0 {
		t.Errorf("LegacyPlugins: got %d, want 0", len(cfg.LegacyPlugins))
	}
	if len(cfg.Lines) != 3 {
		t.Errorf("Lines count: got %d, want 3", len(cfg.Lines))
	}
}

func TestParseConfig_Empty(t *testing.T) {
	path := copyFixture(t, "empty.conf")
	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.ManagedBlockStart != -1 {
		t.Errorf("ManagedBlockStart: got %d, want -1", cfg.ManagedBlockStart)
	}
	if len(cfg.Lines) != 0 {
		t.Errorf("Lines: got %d, want 0", len(cfg.Lines))
	}
}

func TestParseConfig_Mixed(t *testing.T) {
	path := copyFixture(t, "mixed.conf")
	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.ManagedBlockStart != 1 {
		t.Errorf("ManagedBlockStart: got %d, want 1", cfg.ManagedBlockStart)
	}
	if cfg.ManagedBlockEnd != 3 {
		t.Errorf("ManagedBlockEnd: got %d, want 3", cfg.ManagedBlockEnd)
	}
	if cfg.BootstrapLineIndex != 4 {
		t.Errorf("BootstrapLineIndex: got %d, want 4", cfg.BootstrapLineIndex)
	}
	if len(cfg.LegacyPlugins) != 1 {
		t.Errorf("LegacyPlugins: got %d, want 1", len(cfg.LegacyPlugins))
	}
	if len(cfg.ManagedPlugins) != 1 {
		t.Errorf("ManagedPlugins: got %d, want 1", len(cfg.ManagedPlugins))
	}
}

func TestParseConfig_LockPath(t *testing.T) {
	tests := []struct {
		configName string
		wantSuffix string
	}{
		{"tmux.conf", ".lock"},
		{"my-config.conf", "my-config.lock"},
	}

	for _, tt := range tests {
		t.Run(tt.configName, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.configName)
			if err := os.WriteFile(path, []byte(""), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := ParseConfig(path)
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}

			if !strings.HasSuffix(cfg.LockPath, tt.wantSuffix) {
				t.Errorf("LockPath %q should end with %q", cfg.LockPath, tt.wantSuffix)
			}
			if filepath.Dir(cfg.LockPath) != dir {
				t.Errorf("LockPath should be in same directory as config")
			}
		})
	}
}

func TestParseConfig_LinesPreserved(t *testing.T) {
	content := "set -g prefix C-a\n# comment\nset -g mouse on\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if len(cfg.Lines) != 3 {
		t.Errorf("Lines count: got %d, want 3", len(cfg.Lines))
	}
	if cfg.Lines[0] != "set -g prefix C-a" {
		t.Errorf("Lines[0]: got %q, want %q", cfg.Lines[0], "set -g prefix C-a")
	}
	if cfg.Lines[1] != "# comment" {
		t.Errorf("Lines[1]: got %q, want %q", cfg.Lines[1], "# comment")
	}
}
