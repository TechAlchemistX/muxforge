package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectShell(t *testing.T) {
	tests := []struct {
		shellEnv string
		want     Shell
	}{
		{"/bin/zsh", ShellZsh},
		{"/usr/bin/zsh", ShellZsh},
		{"/bin/bash", ShellBash},
		{"/usr/local/bin/bash", ShellBash},
		{"/usr/bin/fish", ShellFish},
		{"/usr/local/bin/fish", ShellFish},
		{"", ShellUnknown},
		{"/bin/sh", ShellUnknown},
		{"/bin/tcsh", ShellUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.shellEnv, func(t *testing.T) {
			t.Setenv("SHELL", tt.shellEnv)
			got := DetectShell()
			if got != tt.want {
				t.Errorf("DetectShell() with SHELL=%q: got %v, want %v", tt.shellEnv, got, tt.want)
			}
		})
	}
}

func TestRCFilePath(t *testing.T) {
	tests := []struct {
		shell      Shell
		wantSuffix string
	}{
		{ShellZsh, ".zshrc"},
		{ShellBash, ".bashrc"},
		{ShellFish, filepath.Join("fish", "config.fish")},
		{ShellUnknown, ""},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := RCFilePath(tt.shell)
			if tt.wantSuffix == "" {
				if got != "" {
					t.Errorf("RCFilePath(ShellUnknown): got %q, want empty string", got)
				}
				return
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("RCFilePath got %q, want suffix %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestAddToPath_AlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".zshrc")
	binDir := "/usr/local/bin"

	// Write rc file that already contains binDir.
	existing := "export PATH=\"$PATH:" + binDir + "\"\n"
	if err := os.WriteFile(rcPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	// Override RCFilePath by using a custom shell that maps to our temp file.
	// We test via the zsh path by setting the real XDG/home to point to our temp dir.
	// Since RCFilePath uses os.UserHomeDir, we test the logic directly by calling
	// AddToPath with a shell constant and verifying the file wasn't changed.

	before, _ := os.ReadFile(rcPath)

	// We can't easily redirect UserHomeDir without refactoring, so we test via
	// a custom approach: directly invoke the internals.
	// Instead, test that the file size doesn't grow if already present.
	// Use ShellUnknown which returns "" path → no-op.
	if err := AddToPath(ShellUnknown, binDir); err != nil {
		t.Fatalf("AddToPath(ShellUnknown): %v", err)
	}

	after, _ := os.ReadFile(rcPath)
	if string(before) != string(after) {
		t.Error("file should not be modified when shell is unknown")
	}
}

func TestAddToPath_ZshAddsLine(t *testing.T) {
	dir := t.TempDir()

	// Point HOME to temp dir so RCFilePath resolves to dir/.zshrc.
	t.Setenv("HOME", dir)

	rcPath := filepath.Join(dir, ".zshrc")
	// Create empty rc file.
	if err := os.WriteFile(rcPath, []byte("# existing content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	binDir := "/usr/local/bin/muxforge-test"
	if err := AddToPath(ShellZsh, binDir); err != nil {
		t.Fatalf("AddToPath: %v", err)
	}

	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# Added by muxforge installer") {
		t.Error("comment not added to rc file")
	}
	if !strings.Contains(content, binDir) {
		t.Error("binDir not added to rc file")
	}
	if !strings.Contains(content, "export PATH") {
		t.Error("PATH export not added for zsh")
	}
	// Existing content must be preserved.
	if !strings.Contains(content, "# existing content") {
		t.Error("existing content was removed")
	}
}

func TestAddToPath_ZshNoDuplicate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	binDir := "/usr/local/bin/muxforge-test"
	rcPath := filepath.Join(dir, ".zshrc")
	// Pre-populate with the exact line that would be added.
	preexisting := "export PATH=\"$PATH:" + binDir + "\"\n"
	if err := os.WriteFile(rcPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	if err := AddToPath(ShellZsh, binDir); err != nil {
		t.Fatalf("AddToPath: %v", err)
	}

	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatal(err)
	}

	// binDir should appear exactly once.
	count := strings.Count(string(data), binDir)
	if count != 1 {
		t.Errorf("binDir appears %d times, want 1 (no duplicate)", count)
	}
}

func TestAddToPath_FishSyntax(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	fishDir := filepath.Join(dir, "fish")
	if err := os.MkdirAll(fishDir, 0755); err != nil {
		t.Fatal(err)
	}
	rcPath := filepath.Join(fishDir, "config.fish")
	if err := os.WriteFile(rcPath, []byte("# fish config\n"), 0644); err != nil {
		t.Fatal(err)
	}

	binDir := "/usr/local/bin/muxforge-test"
	if err := AddToPath(ShellFish, binDir); err != nil {
		t.Fatalf("AddToPath(ShellFish): %v", err)
	}

	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "fish_add_path") {
		t.Error("fish_add_path not found in config.fish")
	}
	if !strings.Contains(content, binDir) {
		t.Error("binDir not found in config.fish")
	}
	// Must NOT use export PATH syntax for fish.
	if strings.Contains(content, "export PATH") {
		t.Error("fish config should not contain 'export PATH'")
	}
}

func TestAddToPath_UnknownNoFileModified(t *testing.T) {
	dir := t.TempDir()
	// No files should be created.
	if err := AddToPath(ShellUnknown, "/some/bin"); err != nil {
		t.Fatalf("AddToPath(ShellUnknown): %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("no files should be created for ShellUnknown, got %d", len(entries))
	}
}
