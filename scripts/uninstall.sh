#!/usr/bin/env sh
# scripts/uninstall.sh
# muxforge uninstaller
#
# Usage:
#   sh uninstall.sh                  — remove binary, clean tmux.conf, keep plugins
#   sh uninstall.sh --purge-plugins  — also delete ~/.tmux/plugins/
#
# The script mirrors muxforge's own config detection order so it finds the
# same tmux.conf that muxforge was managing.

set -e

BINARY_NAME="muxforge"
INSTALL_DIR="/usr/local/bin"
PURGE_PLUGINS=0

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
for arg in "$@"; do
  case "${arg}" in
    --purge-plugins)
      PURGE_PLUGINS=1
      ;;
    --help|-h)
      echo "Usage: uninstall.sh [--purge-plugins]"
      echo ""
      echo "Options:"
      echo "  --purge-plugins   Also remove ~/.tmux/plugins/ (deletes all plugin dirs)"
      exit 0
      ;;
    *)
      echo "Error: unknown option: ${arg}" >&2
      exit 1
      ;;
  esac
done

# ---------------------------------------------------------------------------
# Locate the binary (check PATH first, then the default install location)
# ---------------------------------------------------------------------------
BINARY_PATH="$(command -v "${BINARY_NAME}" 2>/dev/null || true)"
if [ -z "${BINARY_PATH}" ]; then
  BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"
fi

if [ ! -f "${BINARY_PATH}" ]; then
  echo "! ${BINARY_NAME} not found — nothing to uninstall"
  exit 0
fi

echo "-> Uninstalling ${BINARY_NAME} from ${BINARY_PATH}..."

# ---------------------------------------------------------------------------
# Locate tmux.conf (mirrors muxforge's FindConfig order)
# ---------------------------------------------------------------------------
find_tmux_config() {
  if [ -n "${TMUX_CONFIG}" ] && [ -f "${TMUX_CONFIG}" ]; then
    printf '%s' "${TMUX_CONFIG}"
    return
  fi
  XDG_BASE="${XDG_CONFIG_HOME:-${HOME}/.config}"
  if [ -f "${XDG_BASE}/tmux/tmux.conf" ]; then
    printf '%s' "${XDG_BASE}/tmux/tmux.conf"
    return
  fi
  if [ -f "${HOME}/.tmux.conf" ]; then
    printf '%s' "${HOME}/.tmux.conf"
    return
  fi
}

# ---------------------------------------------------------------------------
# Step 1: Clean tmux.conf
# ---------------------------------------------------------------------------
CONFIG_PATH="$(find_tmux_config)"
if [ -n "${CONFIG_PATH}" ]; then
  echo "-> Cleaning ${CONFIG_PATH}..."

  TMP_CONF="$(mktemp)"
  # Remove the muxforge managed block (start marker through end marker inclusive),
  # and both the current and legacy bootstrap lines.
  sed \
    -e '/^# --- muxforge plugins (managed) ---/,/^# --- end muxforge ---/d' \
    -e "/^run 'muxforge load'/d" \
    -e "/^run 'muxforge'/d" \
    "${CONFIG_PATH}" > "${TMP_CONF}"
  mv "${TMP_CONF}" "${CONFIG_PATH}"

  echo "✓ Removed muxforge managed block and bootstrap line"

  # Remove the lock file (lives in the same directory as tmux.conf).
  CONFIG_DIR="$(dirname "${CONFIG_PATH}")"
  CONFIG_BASE="$(basename "${CONFIG_PATH}" .conf)"
  LOCK_PATH="${CONFIG_DIR}/${CONFIG_BASE}.lock"
  if [ -f "${LOCK_PATH}" ]; then
    rm "${LOCK_PATH}"
    echo "✓ Removed lock file ${LOCK_PATH}"
  fi
else
  echo "! No tmux config found — skipping config cleanup"
fi

# ---------------------------------------------------------------------------
# Step 2: Remove plugin directories (opt-in)
# ---------------------------------------------------------------------------
PLUGINS_DIR="${HOME}/.tmux/plugins"
if [ "${PURGE_PLUGINS}" = "1" ]; then
  if [ -d "${PLUGINS_DIR}" ]; then
    rm -rf "${PLUGINS_DIR}"
    echo "✓ Removed plugin directory ${PLUGINS_DIR}"
  fi
else
  if [ -d "${PLUGINS_DIR}" ]; then
    echo "! Plugin directory ${PLUGINS_DIR} was left in place"
    echo "  Run with --purge-plugins to remove it, or delete it manually:"
    echo "    rm -rf ${PLUGINS_DIR}"
  fi
fi

# ---------------------------------------------------------------------------
# Step 3: Remove the binary
# ---------------------------------------------------------------------------
if [ -w "${BINARY_PATH}" ]; then
  rm "${BINARY_PATH}"
else
  sudo rm "${BINARY_PATH}"
fi

echo "✓ Removed ${BINARY_PATH}"
echo ""
echo "✓ muxforge uninstalled"
echo "-> Open a new tmux session (or run: tmux source-file ${CONFIG_PATH:-~/.tmux.conf}) to apply"
