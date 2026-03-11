#!/usr/bin/env bash
# GhostSpell installer for macOS and Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/install.sh | bash
#
# What it does:
#   1. Detects your OS (macOS or Linux) and architecture (amd64 or arm64)
#   2. Downloads the latest GhostSpell release from GitHub
#   3. macOS: Installs GhostSpell.app to /Applications
#   4. Linux: Installs the binary to /usr/local/bin
#
set -euo pipefail

REPO="chrixbedardcad/GhostSpell"
INSTALL_DIR="/usr/local/bin"

# --- Helpers ----------------------------------------------------------------

info()  { printf '\033[1;34m%s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m%s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m%s\033[0m\n' "$*"; }
fail()  { printf '\033[1;31mError: %s\033[0m\n' "$*" >&2; exit 1; }

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but not found. Please install it and retry."
}

# --- Detect platform --------------------------------------------------------

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)      fail "Unsupported OS: $(uname -s). GhostSpell supports macOS and Linux." ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        arm64|aarch64)  echo "arm64" ;;
        *)              fail "Unsupported architecture: $(uname -m). GhostSpell supports amd64 and arm64." ;;
    esac
}

# --- Resolve latest release -------------------------------------------------

latest_version() {
    need_cmd curl
    local url="https://api.github.com/repos/${REPO}/releases/latest"
    curl -fsSL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'
}

# --- macOS installer --------------------------------------------------------

install_macos() {
    local arch="$1" version="$2"
    local asset="GhostSpell-darwin-${arch}.dmg"
    local url="https://github.com/${REPO}/releases/download/${version}/${asset}"
    local tmpdir
    tmpdir=$(mktemp -d)

    info "Downloading ${asset} (${version})..."
    curl -fsSL -o "${tmpdir}/${asset}" "$url" || fail "Download failed. Check your internet connection."

    info "Mounting disk image..."
    local mount_output
    mount_output=$(hdiutil attach "${tmpdir}/${asset}" -nobrowse 2>&1) || fail "Failed to mount DMG: ${mount_output}"

    # Extract mount point — grab everything after /Volumes (handles spaces in path).
    local mount_point
    mount_point=$(echo "$mount_output" | grep '/Volumes/' | sed 's|.*\(/Volumes/.*\)|\1|' | head -1 | xargs)

    if [ -z "$mount_point" ] || [ ! -d "$mount_point" ]; then
        # Fallback: check common mount point.
        mount_point="/Volumes/GhostSpell"
    fi

    if [ ! -d "${mount_point}/GhostSpell.app" ]; then
        hdiutil detach "$mount_point" -quiet 2>/dev/null || true
        rm -rf "$tmpdir"
        fail "Could not find GhostSpell.app in DMG (mount: ${mount_point})"
    fi

    # Kill any running GhostSpell before installing — otherwise macOS will
    # bring the old process to the front instead of launching the new binary.
    killall GhostSpell 2>/dev/null || true
    sleep 1

    info "Installing GhostSpell.app to /Applications..."
    # Remove old version if present, then copy.
    if [ -w /Applications ] || [ ! -d /Applications/GhostSpell.app ]; then
        rm -rf /Applications/GhostSpell.app 2>/dev/null || true
        cp -R "${mount_point}/GhostSpell.app" /Applications/ || {
            info "Need admin permission to install to /Applications..."
            sudo cp -R "${mount_point}/GhostSpell.app" /Applications/
        }
    else
        info "Need admin permission to install to /Applications..."
        sudo rm -rf /Applications/GhostSpell.app
        sudo cp -R "${mount_point}/GhostSpell.app" /Applications/
    fi

    info "Unmounting disk image..."
    hdiutil detach "$mount_point" -quiet 2>/dev/null || true

    # Remove quarantine flag — not needed for notarized builds, but kept
    # as a safety net for users who download manually without notarization.
    xattr -dr com.apple.quarantine /Applications/GhostSpell.app 2>/dev/null || true

    # Force macOS to refresh the app icon (clears Launch Services cache).
    touch /Applications/GhostSpell.app
    /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f /Applications/GhostSpell.app 2>/dev/null || true

    rm -rf "$tmpdir"

    echo ""
    echo "============================================"
    ok "  GhostSpell ${version} installed successfully!"
    echo "============================================"
    echo ""

    info "Launching GhostSpell..."
    open /Applications/GhostSpell.app

    echo ""
    echo "  GhostSpell is running in your menu bar (top right)."
    echo "  Look for the GhostSpell icon — there is no app window."
    echo ""
    echo "  If this is your first install, GhostSpell needs two macOS permissions:"
    echo ""
    echo "  1. ACCESSIBILITY     — for keyboard simulation (Cmd+A, Cmd+C, Cmd+V)"
    echo "  2. INPUT MONITORING  — for global hotkeys (Cmd+G)"
    echo ""
    echo "  The app will prompt you to grant them if needed."
    echo "  Apple docs:"
    echo "    Accessibility:    https://support.apple.com/guide/mac-help/mh43185/mac"
    echo "    Input Monitoring: https://support.apple.com/guide/mac-help/mchl4cedafb6/mac"
    echo ""
    info "Config is stored in: ~/Library/Application Support/GhostSpell/"
}

# --- Linux installer --------------------------------------------------------

install_linux() {
    local arch="$1" version="$2"

    # Only amd64 is built for Linux currently.
    if [ "$arch" != "amd64" ]; then
        warn "Note: Linux builds are currently amd64 only. Downloading amd64 binary."
        arch="amd64"
    fi

    local asset="ghostspell-linux-${arch}"
    local url="https://github.com/${REPO}/releases/download/${version}/${asset}"
    local tmpdir
    tmpdir=$(mktemp -d)

    # Kill any running GhostSpell before replacing the binary.
    killall ghostspell 2>/dev/null || true
    sleep 1

    info "Downloading ${asset} (${version})..."
    curl -fsSL -o "${tmpdir}/ghostspell" "$url" || fail "Download failed. Check your internet connection."
    chmod +x "${tmpdir}/ghostspell"

    info "Installing to ${INSTALL_DIR}/ghostspell..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/ghostspell" "${INSTALL_DIR}/ghostspell"
    else
        sudo mv "${tmpdir}/ghostspell" "${INSTALL_DIR}/ghostspell"
    fi

    rm -rf "$tmpdir"

    echo ""
    echo "============================================"
    ok "  GhostSpell ${version} installed successfully!"
    echo "============================================"
    echo ""

    # Check for required system dependencies.
    local missing=()
    command -v xclip    >/dev/null 2>&1 || missing+=("xclip")
    command -v xdotool  >/dev/null 2>&1 || missing+=("xdotool")
    dpkg -s libwebkit2gtk-4.1-0 >/dev/null 2>&1 || missing+=("libwebkit2gtk-4.1-0")
    dpkg -s libgtk-3-0 >/dev/null 2>&1 || missing+=("libgtk-3-0")

    if [ ${#missing[@]} -gt 0 ]; then
        warn "Required dependencies not found: ${missing[*]}"
        echo "  Install them with:"
        echo "  sudo apt install ${missing[*]}"
        echo ""
    fi

    info "Config is stored in: ~/.config/GhostSpell/"
    echo ""

    if [ ${#missing[@]} -eq 0 ]; then
        info "Launching GhostSpell..."
        nohup ghostspell >/dev/null 2>&1 &
        ok "GhostSpell is running in your system tray."
        echo "  Look for the GhostSpell icon in your panel (top-right area)."
    fi
    echo ""
    info "To launch manually later:"
    echo "  ghostspell"
}

# --- Main -------------------------------------------------------------------

main() {
    local os arch version
    os=$(detect_os)
    arch=$(detect_arch)

    info "Detected: ${os}/${arch}"

    version=$(latest_version)
    if [ -z "$version" ]; then
        fail "Could not determine latest version. Check https://github.com/${REPO}/releases"
    fi
    info "Latest version: ${version}"

    case "$os" in
        darwin) install_macos "$arch" "$version" ;;
        linux)  install_linux "$arch" "$version" ;;
    esac
}

main
