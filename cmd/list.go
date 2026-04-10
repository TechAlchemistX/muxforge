package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/lock"
	"github.com/TechAlchemistX/muxforge/internal/plugin"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all managed plugins and their status",
		Long: `Display a formatted table of all declared plugins with their install
status and lock file information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList()
		},
	}
	return cmd
}

func runList() error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	lf, err := lock.ReadLock(cfg.LockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.ManagedPlugins) == 0 {
		fmt.Println("No plugins declared in the managed block.")
		fmt.Printf("\nConfig: %s\n", tildeAbbrev(cfg.Path))
		return nil
	}

	pluginsDir := config.PluginsDir(cfgPath)

	// Resolve each declared plugin and collect display rows.
	type row struct {
		name   string
		commit string // 7-char short hash, or empty
		status string // raw status text
		color  *color.Color
		symbol string
	}

	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	rows := make([]row, 0, len(cfg.ManagedPlugins))

	// Track max name length for alignment.
	maxName := 0

	for _, raw := range cfg.ManagedPlugins {
		p, err := plugin.NewPlugin(raw, pluginsDir)
		if err != nil {
			// Best-effort: show the raw name with an error status.
			r := row{
				name:   raw,
				commit: "???????",
				status: "invalid plugin declaration",
				color:  red,
				symbol: "✗",
			}
			rows = append(rows, r)
			if len(raw) > maxName {
				maxName = len(raw)
			}
			continue
		}

		locked := lock.FindPlugin(lf, p.Name)
		installed := dirExists(p.InstallPath)

		var r row
		r.name = p.Name

		switch {
		case installed && locked != nil:
			// Installed and in lock file — up to date.
			r.commit = shortCommit(locked.Commit)
			r.status = "up to date"
			r.color = green
			r.symbol = "✓"

		case installed && locked == nil:
			// Installed but not recorded in lock file.
			r.commit = "???????"
			r.status = "not in lock file"
			r.color = yellow
			r.symbol = "!"

		default:
			// In config but directory not present.
			if locked != nil {
				r.commit = shortCommit(locked.Commit)
			} else {
				r.commit = "???????"
			}
			r.status = "not installed"
			r.color = red
			r.symbol = "✗"
		}

		rows = append(rows, r)
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
	}

	fmt.Printf("Installed plugins (%d)\n\n", len(rows))

	for _, r := range rows {
		padding := strings.Repeat(" ", maxName-len(r.name))
		statusStr := r.color.Sprint(r.symbol) + " " + r.status
		fmt.Printf("  %s%s   %s   %s\n", r.name, padding, r.commit, statusStr)
	}

	fmt.Printf("\nLock file: %s\n", tildeAbbrev(cfg.LockPath))
	fmt.Printf("Config:    %s\n", tildeAbbrev(cfg.Path))

	return nil
}

// shortCommit returns the first 7 characters of a commit hash.
// If the hash is shorter than 7 characters, it is returned as-is.
func shortCommit(commit string) string {
	if len(commit) >= 7 {
		return commit[:7]
	}
	return commit
}

// tildeAbbrev replaces the home directory prefix with ~ for display.
func tildeAbbrev(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
