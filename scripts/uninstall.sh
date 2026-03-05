#!/usr/bin/env bash
# GhostType uninstaller for macOS and Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/uninstall.sh | bash
#
# What it does:
#   1. Stops GhostType if it's running
#   2. Removes the app binary (macOS: /Applications/GhostType.app, Linux: /usr/local/bin/ghosttype)
#   3. Removes config, logs, and all app data
#
set -euo pipefail

info()  { printf '\033[1;34m%s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m%s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m%s\033[0m\n' "$*"; }

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)      echo "unsupported" ;;
    esac
}

# --- Stop running instance --------------------------------------------------

info "Stopping GhostType if running..."
pkill -f GhostType 2>/dev/null || pkill -f ghosttype 2>/dev/null || true
sleep 1

# --- Platform-specific removal ----------------------------------------------

os=$(detect_os)

case "$os" in
    darwin)
        info "Removing /Applications/GhostType.app..."
        if [ -d /Applications/GhostType.app ]; then
            rm -rf /Applications/GhostType.app 2>/dev/null || {
                info "Need admin permission..."
                sudo rm -rf /Applications/GhostType.app
            }
        fi

        info "Removing app data (~/Library/Application Support/GhostType/)..."
        rm -rf "$HOME/Library/Application Support/GhostType"
        ;;

    linux)
        info "Removing /usr/local/bin/ghosttype..."
        if [ -w /usr/local/bin ]; then
            rm -f /usr/local/bin/ghosttype
        else
            sudo rm -f /usr/local/bin/ghosttype
        fi

        info "Removing app data (~/.config/GhostType/)..."
        rm -rf "$HOME/.config/GhostType"
        ;;

    *)
        warn "Unsupported OS. Remove GhostType manually."
        exit 1
        ;;
esac

ok ""
ok "GhostType has been uninstalled."
ok ""

if [ "$os" = "darwin" ]; then
    warn "NOTE: macOS privacy permissions must be removed manually."
    echo "  Open System Settings and remove GhostType from both:"
    echo "  1. Privacy & Security > Accessibility"
    echo "  2. Privacy & Security > Input Monitoring"
    echo ""
    info "Opening Privacy & Security settings..."
    open "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility" 2>/dev/null || true
fi
