#!/usr/bin/env bash
# bundle-macos.sh — Package a bare GhostType binary into a macOS .app bundle
# and create a .dmg disk image with a drag-to-Applications layout.
#
# Usage: ./scripts/bundle-macos.sh <binary-path> <arch>
#   e.g.: ./scripts/bundle-macos.sh ghosttype-darwin-arm64 arm64
#
# Produces: GhostType-darwin-<arch>.dmg
set -euo pipefail

BINARY="${1:?Usage: $0 <binary-path> <arch>}"
ARCH="${2:?Usage: $0 <binary-path> <arch>}"

# Read version from source of truth.
VERSION=$(grep 'const Version' internal/version/version.go | sed 's/.*"\(.*\)"/\1/')
echo "Bundling GhostType v${VERSION} for darwin/${ARCH}"

APP_NAME="GhostType.app"
CONTENTS="${APP_NAME}/Contents"
MACOS_DIR="${CONTENTS}/MacOS"
RESOURCES_DIR="${CONTENTS}/Resources"

# Clean any previous bundle.
rm -rf "${APP_NAME}"
mkdir -p "${MACOS_DIR}" "${RESOURCES_DIR}"

# Copy binary.
cp "${BINARY}" "${MACOS_DIR}/GhostType"
chmod +x "${MACOS_DIR}/GhostType"

# Generate .icns from the 1024px PNG icon using macOS built-in tools.
ICONSET_DIR="GhostType.iconset"
rm -rf "${ICONSET_DIR}"
mkdir -p "${ICONSET_DIR}"

ICON_SRC="assets/GhostType_icon_1024.png"
if [ -f "${ICON_SRC}" ]; then
    sips -z 16 16     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_16x16.png"      >/dev/null
    sips -z 32 32     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_16x16@2x.png"   >/dev/null
    sips -z 32 32     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_32x32.png"      >/dev/null
    sips -z 64 64     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_32x32@2x.png"   >/dev/null
    sips -z 128 128   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_128x128.png"    >/dev/null
    sips -z 256 256   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_128x128@2x.png" >/dev/null
    sips -z 256 256   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_256x256.png"    >/dev/null
    sips -z 512 512   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_256x256@2x.png" >/dev/null
    sips -z 512 512   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_512x512.png"    >/dev/null
    sips -z 1024 1024 "${ICON_SRC}" --out "${ICONSET_DIR}/icon_512x512@2x.png" >/dev/null
    iconutil -c icns "${ICONSET_DIR}" -o "${RESOURCES_DIR}/GhostType.icns"
    rm -rf "${ICONSET_DIR}"
    echo "Icon: GhostType.icns created"
else
    echo "Warning: ${ICON_SRC} not found — .app will have no icon"
fi

# Write Info.plist.
cat > "${CONTENTS}/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>GhostType</string>
    <key>CFBundleDisplayName</key>
    <string>GhostType</string>
    <key>CFBundleIdentifier</key>
    <string>com.ghosttype.app</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleExecutable</key>
    <string>GhostType</string>
    <key>CFBundleIconFile</key>
    <string>GhostType</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSAccessibilityUsageDescription</key>
    <string>GhostType needs Accessibility access to register global hotkeys and simulate keyboard shortcuts for text correction.</string>
</dict>
</plist>
PLIST

echo "Info.plist written"

# Create .dmg disk image with drag-to-Applications layout.
DMG_NAME="GhostType-darwin-${ARCH}.dmg"
DMG_STAGING="dmg_contents"
rm -rf "${DMG_STAGING}"
mkdir -p "${DMG_STAGING}"
cp -R "${APP_NAME}" "${DMG_STAGING}/"
ln -s /Applications "${DMG_STAGING}/Applications"

hdiutil create -volname "GhostType" \
    -srcfolder "${DMG_STAGING}" \
    -ov -format UDZO \
    "${DMG_NAME}"

echo "Bundle complete: ${DMG_NAME}"

# Clean up staging files.
rm -rf "${APP_NAME}" "${DMG_STAGING}"
