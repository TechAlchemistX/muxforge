# muxforge

**Reproducible tmux plugin management for engineers who live on servers.**

[![CI](https://github.com/TechAlchemistX/muxforge/actions/workflows/ci.yml/badge.svg)](https://github.com/TechAlchemistX/muxforge/actions/workflows/ci.yml)
[![Release](https://github.com/TechAlchemistX/muxforge/actions/workflows/release.yml/badge.svg)](https://github.com/TechAlchemistX/muxforge/actions/workflows/release.yml)

TPM still works. But it has no memory, no lock file, and no concept of reproducibility. muxforge does.

```bash
curl -fsSL https://muxforge.dev/install.sh | sh && muxforge install
```

---

## Why muxforge

TPM was built in 2013 for a different era of how engineers work. It installs plugins. That's it.

muxforge manages your tmux environment — the same way Terraform manages infrastructure. Declare what you want. Get exactly that. Every time. On every machine.

- **Lock file** — pins exact plugin versions so two machines set up a month apart are identical
- **Single binary** — nothing to clone, nothing to source, nothing to break
- **Auto-detects your config** — works with `~/.config/tmux/tmux.conf` (XDG) and `~/.tmux.conf` without any configuration
- **Manages your config for you** — installs, removes, and migrates plugins directly in your tmux.conf
- **Works seamlessly with stow and dotfile managers** — the lock file belongs in version control, just like your config
- **One command migration from TPM** — existing setup moves over in seconds

> If your terminal has built-in splits, that's great for local work. tmux lives on the server. muxforge manages that environment.

---

## Install

**curl (universal — Mac, Linux, servers, containers)**
```bash
curl -fsSL https://muxforge.dev/install.sh | sh
```

**Homebrew**
```bash
brew install muxforge
```

**GitHub Releases**

Download the binary for your platform from [releases](https://github.com/TechAlchemistX/muxforge/releases) and drop it in your PATH.

---

## Quick Start

**Fresh setup**

```bash
# 1. Get your tmux.conf in place (copy from dotfiles, scp, curl from a gist)
# 2. Run the one-liner
curl -fsSL https://muxforge.dev/install.sh | sh && muxforge install
```

muxforge will:
- Find your tmux.conf automatically (XDG or legacy path)
- Migrate any existing `@plugin` declarations into its managed block
- Add the bootstrap line to your tmux.conf
- Clone all declared plugins
- Write your lock file

Your tmux.conf will contain a managed block that looks like this:

```tmux
# --- muxforge plugins (managed) ---
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin 'tmux-plugins/tmux-resurrect'
set -g @plugin 'christoomey/vim-tmux-navigator'
# --- end muxforge ---

run 'muxforge'
```

Everything inside the managed block is muxforge's territory. Everything outside is yours. It never touches anything outside that block.

---

## Core Commands

| Command | What it does |
|---|---|
| `muxforge install` | Install all plugins declared in config, respect lock file versions |
| `muxforge install <plugin>` | Add plugin, update config and lock file |
| `muxforge remove <plugin>` | Remove plugin, update config and lock file |
| `muxforge update` | Update all plugins, update lock file |
| `muxforge update <plugin>` | Update specific plugin, update lock file |
| `muxforge list` | Show installed plugins and their pinned versions |
| `muxforge sync` | Reconcile config, installed plugins, and lock file |
| `muxforge migrate` | Migrate from TPM |

---

## Migrating from TPM

Already using TPM? One command.

```bash
muxforge migrate
```

muxforge will find your existing `@plugin` declarations, move them into the managed block, resolve current versions, and write your lock file. Your plugins stay exactly where they are. Nothing reinstalled.

Remove the old TPM bootstrap line from the bottom of your config and you're done.

---

## How It Works With Dotfiles

The lock file lives next to your tmux.conf — version controlled, stowed alongside your config, committed to your dotfiles repo.

**With stow:**
```
~/dotfiles/
  tmux/
    .config/
      tmux/
        tmux.conf       ← your config
        tmux.lock       ← muxforge lock file, commit this
```

The installed plugins themselves (`~/.tmux/plugins/`) are derived from the lock file — they're the equivalent of `node_modules`. Add them to your `.gitignore`.

**New machine workflow:**
```bash
# Clone your dotfiles and stow
git clone https://github.com/you/dotfiles ~/dotfiles
cd ~/dotfiles && stow tmux

# Install muxforge and get exact plugin versions from lock file
curl -fsSL https://muxforge.dev/install.sh | sh && muxforge install
```

Same environment. Every machine. Every time.

---

## Config Detection

muxforge automatically finds your tmux config — no path configuration needed.

It checks in this order, mirroring tmux's own lookup:

1. `$TMUX_CONFIG` — if explicitly set
2. `$XDG_CONFIG_HOME/tmux/tmux.conf` — XDG path (usually `~/.config/tmux/tmux.conf`)
3. `~/.config/tmux/tmux.conf` — XDG default
4. `~/.tmux.conf` — legacy fallback

The lock file always lives in the same directory as your config.

---

## Compatibility

muxforge uses the same `@plugin` declaration syntax as TPM. Every TPM-compatible plugin works without any changes. No plugin modifications needed. No ecosystem changes required.

If a plugin works with TPM, it works with muxforge.

---

## Roadmap

- [ ] Plugin version pinning to specific commits and tags
- [ ] `tmux.lock` diff output on update (see exactly what changed)
- [ ] Non-GitHub plugin sources (GitLab, self-hosted)
- [ ] AUR package
- [ ] Nix flake

Have a feature request? [Open an issue.](https://github.com/TechAlchemistX/muxforge/issues)

---

## The Lock File

`tmux.lock` pins the exact commit hash of every installed plugin.

```json
{
  "version": "1",
  "plugins": [
    {
      "name": "tmux-plugins/tmux-sensible",
      "source": "https://github.com/tmux-plugins/tmux-sensible",
      "commit": "25cb91f42d020f675bb0a4d3f81c1b259b951e31",
      "installed_at": "2026-03-18T10:00:00Z"
    },
    {
      "name": "tmux-plugins/tmux-resurrect",
      "source": "https://github.com/tmux-plugins/tmux-resurrect",
      "commit": "ca9f8a75073bf82f9f1afc8af9b11fa17eb33f74",
      "installed_at": "2026-03-18T10:00:00Z"
    }
  ]
}
```

Commit this file. Your environment becomes reproducible across every machine, every reinstall, every new engineer on the team.

---

## Contributing

muxforge is written in Go. Contributions are welcome.

**Prerequisites**
- Go 1.22+
- tmux 3.0+ (for testing)

**Setup**
```bash
git clone https://github.com/TechAlchemistX/muxforge
cd muxforge
go mod download
go build ./...
go test ./...
```

**Before submitting a PR**
- Run `go test ./...` — all tests must pass
- Run `go vet ./...` — no warnings
- Keep PRs focused — one feature or fix per PR

---

## License

MIT — see [LICENSE](./LICENSE)

---

*Built by [Mandeep Patel](https://github.com/TechAlchemistX) — DevOps Architect, daily AI practitioner, and someone who has been burned by a dropped SSH connection one too many times.*
