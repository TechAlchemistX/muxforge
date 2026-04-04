package plugin

import (
	"strings"
	"testing"
)

func TestNewPlugin(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantName    string
		wantSource  string
		wantSuffix  string // suffix of InstallPath
		wantErr     bool
	}{
		{
			name:       "shorthand owner/repo",
			raw:        "tmux-plugins/tmux-sensible",
			wantName:   "tmux-plugins/tmux-sensible",
			wantSource: "https://github.com/tmux-plugins/tmux-sensible",
			wantSuffix: "/.tmux/plugins/tmux-sensible",
		},
		{
			name:       "full https URL",
			raw:        "https://github.com/tmux-plugins/tmux-resurrect",
			wantName:   "tmux-plugins/tmux-resurrect",
			wantSource: "https://github.com/tmux-plugins/tmux-resurrect",
			wantSuffix: "/.tmux/plugins/tmux-resurrect",
		},
		{
			name:    "invalid input",
			raw:     "notarepo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPlugin(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Raw != tt.raw {
				t.Errorf("Raw: got %q, want %q", p.Raw, tt.raw)
			}
			if p.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", p.Name, tt.wantName)
			}
			if p.Source != tt.wantSource {
				t.Errorf("Source: got %q, want %q", p.Source, tt.wantSource)
			}
			if !strings.HasSuffix(p.InstallPath, tt.wantSuffix) {
				t.Errorf("InstallPath %q should end with %q", p.InstallPath, tt.wantSuffix)
			}
		})
	}
}

func TestShortName(t *testing.T) {
	tests := []struct {
		plugin *Plugin
		want   string
	}{
		{
			plugin: &Plugin{Name: "tmux-plugins/tmux-sensible"},
			want:   "tmux-sensible",
		},
		{
			plugin: &Plugin{Name: "christoomey/vim-tmux-navigator"},
			want:   "vim-tmux-navigator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ShortName(tt.plugin)
			if got != tt.want {
				t.Errorf("ShortName: got %q, want %q", got, tt.want)
			}
		})
	}
}
