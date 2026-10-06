#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
CONTENT=${1:?Pass the standard screenpack checkout directory}
DEST="$ROOT/build/mister/test"
mkdir -p "$DEST"
for part in chars data font sound stages; do
  cp -r "$CONTENT/$part" "$DEST/"
done
for part in data external font; do
  cp -r "$ROOT/$part" "$DEST/"
done
cp "$ROOT/build/mister/Ikemen_GO.armv7" "$DEST/"
cp "$ROOT/tools/run_mister_headless.sh" "$DEST/"
cp "$ROOT/tools/run_mister_video.sh" "$DEST/"
cp "$ROOT/tools/run_mister_play.sh" "$DEST/"
cp "$ROOT/src/resources/defaultConfig.ini" "$DEST/test.ini"
sed -i -E \
  -e 's/^(RenderMode[[:space:]]*=).*/\1 SDL2 Software/' \
  -e 's/^(GameWidth[[:space:]]*=).*/\1 320/' \
  -e 's/^(GameHeight[[:space:]]*=).*/\1 240/' \
  -e 's/^(HeapMemoryLimit[[:space:]]*=).*/\1 256/' \
  -e 's/^((VSync|EnableModel|EnableModelShadow|RGBSpriteBilinearFilter|WindowScaleMode)[[:space:]]*=).*/\1 0/' \
  "$DEST/test.ini"
tar --owner=0 --group=0 -czf "$ROOT/build/mister/ikemen-test.tar.gz" -C "$DEST" .
