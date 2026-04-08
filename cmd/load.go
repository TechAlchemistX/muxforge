package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TechAlchemistX/muxforge/internal/config"
	"github.com/TechAlchemistX/muxforge/internal/plugin"
	"github.com/spf13/cobra"
)

func newLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load",
		Short: "Source all managed plugins into the current tmux session",
		Long: `Load is called automatically by tmux at startup via the bootstrap line:

    run 'muxforge load'

It sources each plugin's entry point script (.tmux file) so that plugin
keybindings, options, and hooks are activated in the running session.
You can also run it manually to reload plugins without restarting tmux.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoad()
		},
	}
}

func runLoad() error {
	cfgPath, err := config.FindConfig()
	if err != nil {
		// Silent failure during tmux startup — no config means nothing to load.
		return nil
	}

	cfg, err := config.ParseConfig(cfgPath)
	if err != nil {
		return nil
	}

	if len(cfg.ManagedPlugins) == 0 {
		return nil
	}

	home, _ := os.UserHomeDir()
	// TMUX_PLUGIN_MANAGER_PATH is expected by many plugins to locate their own scripts.
	pluginsDir := filepath.Join(home, ".tmux", "plugins") + string(filepath.Separator)

	for _, raw := range cfg.ManagedPlugins {
		p, err := plugin.NewPlugin(raw)
		if err != nil {
			continue
		}

		// Discover all *.tmux entry points in the plugin directory, matching
		// how TPM finds plugin scripts rather than guessing the filename.
		entries, err := os.ReadDir(p.InstallPath)
		if err != nil {
			// Plugin directory missing — may not be installed yet.
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmux") {
				continue
			}

			entryPoint := filepath.Join(p.InstallPath, entry.Name())

			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Ensure the entry point is executable (git clone preserves bits, but
			// some file systems or copy operations may strip the execute bit).
			if info.Mode()&0111 == 0 {
				_ = os.Chmod(entryPoint, info.Mode()|0755)
			}

			execCmd := exec.Command(entryPoint)
			execCmd.Env = append(os.Environ(), "TMUX_PLUGIN_MANAGER_PATH="+pluginsDir)
			// Errors from individual plugins are non-fatal — continue loading others.
			_ = execCmd.Run()
		}
	}

	return nil
}
