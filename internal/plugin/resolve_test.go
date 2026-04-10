package plugin

import (
	"testing"
)

func TestResolveSource(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "owner/repo shorthand",
			input: "tmux-plugins/tmux-sensible",
			want:  "https://github.com/tmux-plugins/tmux-sensible",
		},
		{
			name:  "full https URL unchanged",
			input: "https://github.com/tmux-plugins/tmux-sensible",
			want:  "https://github.com/tmux-plugins/tmux-sensible",
		},
		{
			name:  "github.com without scheme",
			input: "github.com/tmux-plugins/tmux-sensible",
			want:  "https://github.com/tmux-plugins/tmux-sensible",
		},
		{
			name:  "whitespace trimmed",
			input: "  tmux-plugins/tmux-sensible  ",
			want:  "https://github.com/tmux-plugins/tmux-sensible",
		},
		{
			name:    "invalid bare name",
			input:   "justareponame",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too many slashes",
			input:   "a/b/c",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveSource(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallPath(t *testing.T) {
	tests := []struct {
		input      string
		pluginsDir string
		want       string
	}{
		{"tmux-plugins/tmux-sensible", "/home/user/.tmux/plugins", "/home/user/.tmux/plugins/tmux-sensible"},
		{"https://github.com/tmux-plugins/tmux-sensible", "/home/user/.config/tmux/plugins", "/home/user/.config/tmux/plugins/tmux-sensible"},
		{"owner/myrepo", "/home/user/.tmux/plugins", "/home/user/.tmux/plugins/myrepo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := InstallPath(tt.input, tt.pluginsDir)
			if got != tt.want {
				t.Errorf("InstallPath(%q, %q) = %q, want %q", tt.input, tt.pluginsDir, got, tt.want)
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"tmux-plugins/tmux-sensible", "tmux-plugins/tmux-sensible"},
		{"https://github.com/tmux-plugins/tmux-sensible", "tmux-plugins/tmux-sensible"},
		{"github.com/tmux-plugins/tmux-sensible", "tmux-plugins/tmux-sensible"},
		{"  owner/repo  ", "owner/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
