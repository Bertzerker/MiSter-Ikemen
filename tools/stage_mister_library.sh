#!/usr/bin/env bash
# Produce the shared runtime and MiSTer Scripts entry, without game content.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
TIMING="$ROOT/fpga/output_files/IKEMEN.sta.summary"
if [ ! -f "$TIMING" ] || ! grep -q '^Slack' "$TIMING" || grep -Eq '^Slack[[:space:]]*:[[:space:]]*-' "$TIMING"; then
  echo 'Refusing to package an FPGA core without passing timing results.' >&2
  exit 1
fi
CONTENT=${1:-$ROOT/../TestContent}
[ -f "$CONTENT/data/gofx/gofx.def" ] || { echo 'Pass a standard screenpack checkout containing data/gofx.' >&2; exit 1; }
DEST="$ROOT/build/mister/library"
ENGINE="$DEST/games/IkemenFrontier"
mkdir -p "$ENGINE" "$DEST/Scripts"
for part in data external font; do cp -r "$ROOT/$part" "$ENGINE/"; done
cp -r "$CONTENT/data/gofx" "$ENGINE/data/"
cp "$ROOT/build/mister/Ikemen_GO.armv7" "$ENGINE/"
cp "$ROOT/fpga/output_files/IKEMEN.rbf" "$ENGINE/"
cp "$ROOT/src/resources/defaultConfig.ini" "$ENGINE/"
cp "$ROOT/tools/launch-game.sh" "$ENGINE/"
cp "$ROOT/tools/mugen-width-coordinates.awk" "$ENGINE/"
cp "$ROOT/tools/MUGEN.sh" "$DEST/Scripts/"
chmod +x "$ENGINE/Ikemen_GO.armv7" "$ENGINE/launch-game.sh" "$DEST/Scripts/MUGEN.sh"
tar --owner=0 --group=0 -czf "$ROOT/build/mister/ikemen-library.tar.gz" -C "$DEST" .
