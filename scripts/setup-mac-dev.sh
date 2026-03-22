#!/usr/bin/env bash
# GhostSpell — macOS development setup + build script.
#
# Run this once on a fresh Mac to install all dependencies,
# clone the repo, build everything, and launch GhostSpell.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/setup-mac-dev.sh | bash
#
# Or if you already have the repo:
#   ./scripts/setup-mac-dev.sh
#
set -euo pipefail

info()  { printf '\033[1;34m→ %s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m⚠ %s\033[0m\n' "$*"; }
fail()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ============================================================
# Step 1: Install prerequisites
# ============================================================
info "Checking prerequisites..."

# Homebrew
if ! command -v brew &>/dev/null; then
    info "Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi
ok "Homebrew"

# Go
if ! command -v go &>/dev/null; then
    info "Installing Go..."
    brew install go
fi
ok "Go $(go version | awk '{print $3}')"

# Node.js
if ! command -v node &>/dev/null; then
    info "Installing Node.js..."
    brew install node
fi
ok "Node $(node --version)"

# CMake
if ! command -v cmake &>/dev/null; then
    info "Installing CMake..."
    brew install cmake
fi
ok "CMake $(cmake --version | head -1 | awk '{print $3}')"

# Claude Code (optional)
if ! command -v claude &>/dev/null; then
    info "Installing Claude Code..."
    npm install -g @anthropic-ai/claude-code 2>/dev/null || warn "Claude Code install failed — install manually: npm install -g @anthropic-ai/claude-code"
else
    ok "Claude Code $(claude --version 2>/dev/null || echo 'installed')"
fi

# ============================================================
# Step 2: Clone or update repo
# ============================================================
REPO_DIR="${GHOSTSPELL_DIR:-$HOME/Projects/GhostSpell}"

if [ -d "$REPO_DIR/.git" ]; then
    info "Updating existing repo at $REPO_DIR..."
    cd "$REPO_DIR"
    git pull origin main
else
    info "Cloning GhostSpell to $REPO_DIR..."
    git clone https://github.com/chrixbedardcad/GhostSpell.git "$REPO_DIR"
    cd "$REPO_DIR"
fi
ok "Repo ready at $REPO_DIR"

# ============================================================
# Step 3: Build Ghost-AI (llama.cpp)
# ============================================================
info "Building Ghost-AI (llama.cpp)..."
chmod +x scripts/build-ghostai.sh
./scripts/build-ghostai.sh
ok "Ghost-AI built"

# ============================================================
# Step 4: Build Ghost Voice (whisper.cpp + ghostvoice)
# ============================================================
info "Building Ghost Voice (whisper.cpp)..."
chmod +x scripts/build-ghostvoice.sh
./scripts/build-ghostvoice.sh
ok "Ghost Voice built"

# ============================================================
# Step 5: Build React frontend
# ============================================================
info "Building React frontend..."
cd gui/frontend
npm ci
npm run build
cd ../..
ok "Frontend built"

# ============================================================
# Step 6: Build GhostSpell
# ============================================================
info "Building ghostspell..."
CGO_ENABLED=1 go build -tags "production ghostai" -o ghostspell .
ok "ghostspell built"

# ============================================================
# Step 7: Summary
# ============================================================
echo ""
echo "============================================"
echo "  BUILD COMPLETE"
echo "============================================"
echo ""
echo "  ghostspell     — main app ($(ls -lh ghostspell | awk '{print $5}'))"

GHOSTVOICE=$(ls ghostvoice-darwin-* ghostvoice_bin ghostvoice 2>/dev/null | head -1)
if [ -n "$GHOSTVOICE" ]; then
    echo "  $GHOSTVOICE — voice helper ($(ls -lh "$GHOSTVOICE" | awk '{print $5}'))"
fi

echo ""
echo "  Run:   ./ghostspell"
echo "  Debug: claude"
echo ""
echo "  First launch:"
echo "    1. Grant Accessibility + Input Monitoring in System Settings"
echo "    2. Download an AI model in the setup wizard"
echo "    3. Download whisper-base in Settings > Voice"
echo "    4. Press F7 to use"
echo ""

# ============================================================
# Step 8: Launch (optional)
# ============================================================
read -p "Launch GhostSpell now? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./ghostspell &
    ok "GhostSpell launched"
fi
