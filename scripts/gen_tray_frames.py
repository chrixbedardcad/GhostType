#!/usr/bin/env python3
"""Generate tray icon animation frames for GhostSpell's working indicator.

Creates 4 frames per icon (8 files total) with a bouncing + fading ghost effect:
  Frame 1: ghost shifted UP 8px, 70% opacity  (faded out, high position)
  Frame 2: ghost at center, 100% opacity       (fully visible)
  Frame 3: ghost shifted DOWN 8px, 70% opacity (faded out, low position)
  Frame 4: ghost at center, 85% opacity        (partially faded, pulse)

Cycle: up-faded -> center-bright -> down-faded -> center-dim -> repeat

For macOS template icons: opacity is applied by scaling the ALPHA channel of
existing white pixels (macOS uses alpha for template icon visibility).
For colored icons: opacity is applied uniformly to all non-transparent pixels.
"""

from PIL import Image
import os
import sys

ASSETS_DIR = os.path.normpath(os.path.join(os.path.dirname(__file__), "..", "assets"))

# Frame definitions: (y_offset_px, opacity)
# Negative y = shift up, positive y = shift down
FRAMES = [
    (-8, 0.70),  # Frame 1: up 8px, 70% opacity
    (0, 1.00),   # Frame 2: center, 100% opacity
    (8, 0.70),   # Frame 3: down 8px, 70% opacity
    (0, 0.85),   # Frame 4: center, 85% opacity
]

SOURCES = [
    # (source filename, output suffix, is_macos_template)
    ("GhostSpell_tray_64.png", ".png", False),
    ("GhostSpell_tray_64_macOS.png", "_macOS.png", True),
]


def generate_frame(src: Image.Image, dy: int, opacity: float) -> Image.Image:
    """Generate a single animation frame with vertical shift and opacity.

    Args:
        src: Source RGBA image.
        dy: Vertical pixel offset (negative=up, positive=down).
        opacity: Opacity multiplier 0.0-1.0 applied to alpha channel.
    Returns:
        New RGBA image with effects applied.
    """
    w, h = src.size

    # Step 1: Apply opacity by scaling the alpha channel
    faded = src.copy()
    if opacity < 1.0:
        pixels = faded.load()
        for y in range(h):
            for x in range(w):
                r, g, b, a = pixels[x, y]
                if a > 0:
                    pixels[x, y] = (r, g, b, int(a * opacity + 0.5))

    # Step 2: Place on transparent canvas with vertical offset
    frame = Image.new("RGBA", (w, h), (0, 0, 0, 0))

    if dy == 0:
        frame.paste(faded, (0, 0))
    elif dy < 0:
        # Shift up: crop bottom of source, paste at top of canvas
        cropped = faded.crop((0, -dy, w, h))
        frame.paste(cropped, (0, 0))
    else:
        # Shift down: crop top of source, paste lower on canvas
        cropped = faded.crop((0, 0, w, h - dy))
        frame.paste(cropped, (0, dy))

    return frame


def main():
    results = []

    for src_name, out_suffix, is_macos in SOURCES:
        src_path = os.path.join(ASSETS_DIR, src_name)
        if not os.path.exists(src_path):
            print(f"ERROR: Source not found: {src_path}", file=sys.stderr)
            sys.exit(1)

        img = Image.open(src_path).convert("RGBA")
        kind = "macOS template" if is_macos else "colored"
        print(f"Loaded {src_name} ({img.size[0]}x{img.size[1]}, {kind})")

        for i, (dy, opacity) in enumerate(FRAMES, start=1):
            frame = generate_frame(img, dy, opacity)
            out_name = f"GhostSpell_tray_working_{i}{out_suffix}"
            out_path = os.path.join(ASSETS_DIR, out_name)
            frame.save(out_path, "PNG")
            size = os.path.getsize(out_path)
            direction = "UP" if dy < 0 else ("DOWN" if dy > 0 else "CENTER")
            print(f"  Frame {i}: {out_name} (dy={dy:+d}px {direction}, "
                  f"opacity={opacity:.0%}, {size} bytes)")
            results.append((out_name, out_path, size))

    # Verification
    print(f"\n--- Verification ({len(results)} files) ---")
    all_ok = True
    for name, path, size in results:
        exists = os.path.exists(path)
        ok = exists and size > 500
        status = "OK" if ok else "FAIL"
        print(f"  [{status}] {name}: {size} bytes")
        if not ok:
            all_ok = False

    if all_ok:
        print(f"\nAll {len(results)} animation frames generated successfully.")
    else:
        print("\nSome frames FAILED verification!", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
