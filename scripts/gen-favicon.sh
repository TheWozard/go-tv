#!/usr/bin/env bash
# Generates favicon assets from static/images/tv.svg using ImageMagick.
# Output files are .gitignored; re-run this script to regenerate them.
set -euo pipefail

SRC="$(dirname "$0")/images/tv.svg"
OUT="$(dirname "$0")/../static"

magick -background none "$SRC" -resize 16x16   "$OUT/favicon-16x16.png"
magick -background none "$SRC" -resize 32x32   "$OUT/favicon-32x32.png"
magick -background none "$SRC" -resize 180x180 "$OUT/apple-touch-icon.png"
magick -background none "$SRC" -define icon:auto-resize=16,32 "$OUT/favicon.ico"

echo "favicon assets written to $OUT"
