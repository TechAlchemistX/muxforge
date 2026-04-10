package plugin

import (
	"testing"
)

func TestNewPlugin(t *testing.T) {
	pluginsDir := "/tmp/test-plugins"

	tests := []struct {
		name       string
		raw        string
		wantName   string
		wantSource string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "shorthand owner/repo",
			raw:        "tmux-plugins/tmux-sensible",
			wantName:   "tmux-plugins/tmux-sensible",
			wantSource: "https://github.com/tmux-plugins/tmux-sensible",
			wantPath:   "/tmp/test-plugins/tmux-sensible",
		},
		{
			name:       "full https URL",
			raw:        "https://github.com/tmux-plugins/tmux-resurrect",
			wantName:   "tmux-plugins/tmux-resurrect",
			wantSource: "https://github.com/tmux-plugins/tmux-resurrect",
			wantPath:   "/tmp/test-plugins/tmux-resurrect",
		},
		{
			name:    "invalid input",
			raw:     "notarepo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPlugin(tt.raw, pluginsDir)
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
			if p.InstallPath != tt.wantPath {
				t.Errorf("InstallPath: got %q, want %q", p.InstallPath, tt.wantPath)
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
