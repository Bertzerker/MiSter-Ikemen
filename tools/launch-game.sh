#!/bin/bash
set -euo pipefail
ENGINE=${IKEMEN_ENGINE_DIR:-$(cd "$(dirname "$0")" && pwd)}
GAMES=${IKEMEN_GAMES_DIR:-/media/fat/games/MUGEN}
export SDL_VIDEODRIVER=dummy SDL_AUDIODRIVER=dummy
export IKEMEN_FRONTIER=1 IKEMEN_LOW_MEMORY=1 GOMEMLIMIT=256MiB

game=${1:-}
if [ -z "$game" ]; then
  choices=()
  for path in "$GAMES"/*; do
    [ -d "$path/data" ] || continue
    if [ -f "$path/data/mugen.cfg" ] || [ -f "$path/data/system.def" ]; then
      choices+=("$(basename "$path")" "")
    fi
  done
  if [ ${#choices[@]} -eq 0 ]; then
    echo "No games found. Add game folders to $GAMES."
    exit 1
  fi
  game=$(dialog --stdout --title 'MUGEN / IKEMEN Frontier' --menu 'Choose a game' 20 70 12 "${choices[@]}") || exit 0
fi
# Accept a folder name, never a path escaping the configured library.
case "$game" in ''|.|..|*/*) echo 'Expected a game folder name.' >&2; exit 1;; esac
GAME="$GAMES/$game"
[ -d "$GAME/data" ] || { echo "Missing game data: $GAME" >&2; exit 1; }
mkdir -p "$ENGINE/profiles/$game"
PROFILE="$ENGINE/profiles/$game"
# Optional content-addressed PCM cache. Missing entries use original music.
music_cache="$PROFILE/music-cache"
# Reuse an existing cache after a game-folder rename without copying its PCM.
if [ -f "$PROFILE/music-cache-path.txt" ]; then
  IFS= read -r shared_cache < "$PROFILE/music-cache-path.txt" || true
  shared_cache=${shared_cache%$'\r'}
  [ ! -d "$shared_cache" ] || music_cache=$shared_cache
fi
if [ -d "$music_cache" ]; then
  export IKEMEN_AUDIO_CACHE="$music_cache"
fi
# A per-game lock prevents concurrent sessions from changing the same saves.
exec 9>"$PROFILE/session.lock"
flock -n 9 || { echo 'This game is already running.' >&2; exit 1; }

# FAT does not support symlinks. Only directories and symlinks live in /tmp;
# large assets remain on the SD card. Overlay game files over engine defaults.
WORK=$(mktemp -d /tmp/ikemen-game.XXXXXX)
cleanup() { rm -rf -- "$WORK"; }
trap cleanup EXIT
overlay() {
  local source=$1 dest=$2 file name
  mkdir -p "$dest"
  while IFS= read -r -d '' file; do
    name=${file##*/}
    if [ -d "$file" ]; then
      overlay "$file" "$dest/$name"
    else
      ln -sfn "$file" "$dest/$name"
    fi
  done < <(find "$source" -mindepth 1 -maxdepth 1 -print0)
}
for part in data font; do
  [ ! -d "$ENGINE/$part" ] || overlay "$ENGINE/$part" "$WORK/$part"
  [ ! -d "$GAME/$part" ] || overlay "$GAME/$part" "$WORK/$part"
done
ln -s "$ENGINE/external" "$WORK/external"
for part in chars stages sound; do
  [ ! -d "$GAME/$part" ] || ln -s "$GAME/$part" "$WORK/$part"
done
mkdir -p "$PROFILE/save" "$PROFILE/debug"
ln -s "$PROFILE/save" "$WORK/save"
ln -s "$PROFILE/debug" "$WORK/debug"

motif=data/system.def
if [ -f "$GAME/data/mugen.cfg" ]; then
  configured=$(sed -nE 's/^[[:space:]]*[Mm][Oo][Tt][Ii][Ff][[:space:]]*=[[:space:]]*([^;]+).*/\1/p' "$GAME/data/mugen.cfg" | head -n 1 | tr -d '\r' | sed 's/[[:space:]]*$//;s/^"//;s/"$//;s@\\@/@g')
  [ -z "$configured" ] || motif=$configured
fi
# Native IKEMEN collections can include several system.def files. Preserve a
# per-game choice instead of always forcing the root MUGEN-style screenpack.
if [ -f "$PROFILE/motif.txt" ]; then
  IFS= read -r selected_motif < "$PROFILE/motif.txt" || true
  selected_motif=${selected_motif%$'\r'}
  [ -z "$selected_motif" ] || motif=$selected_motif
fi
[ -f "$WORK/$motif" ] || { echo "Missing motif: $motif" >&2; exit 1; }
if [ -f "$GAME/data/mugen.cfg" ]; then
  # Write elsewhere, then replace the overlay symlink: never redirect output
  # through a symlink into the user's original screenpack.
  awk -f "$ENGINE/mugen-width-coordinates.awk" "$WORK/$motif" > "$WORK/motif-adjusted.tmp"
  mv -f "$WORK/motif-adjusted.tmp" "$WORK/$motif"
fi
if [ ! -f "$PROFILE/config.ini" ]; then
  cp "$ENGINE/defaultConfig.ini" "$PROFILE/config.ini"
fi
# Enforce hardware constraints while preserving the game's other saved options.
sed -i -E \
  -e 's/^(RenderMode[[:space:]]*=).*/\1 SDL2 Software/' \
  -e 's/^(GameWidth[[:space:]]*=).*/\1 320/' \
  -e 's/^(GameHeight[[:space:]]*=).*/\1 240/' \
  -e 's/^(HeapMemoryLimit[[:space:]]*=).*/\1 256/' \
  -e 's/^(SampleRate[[:space:]]*=).*/\1 48000/' \
  -e 's/^(BGMRAMBuffer[[:space:]]*=).*/\1 0/' \
  -e 's/^((VSync|EnableModel|EnableModelShadow|RGBSpriteBilinearFilter|WindowScaleMode|EagerSpriteTextures|FirstRun)[[:space:]]*=).*/\1 0/' \
  "$PROFILE/config.ini"

if [ "${IKEMEN_FORCE_CORE:-0}" = 1 ] || [ "$(cat /tmp/CORENAME 2>/dev/null)" != IKEMEN ]; then
  core=${IKEMEN_CORE:-$ENGINE/IKEMEN.rbf}
  [ -f "$core" ] || { echo 'Missing IKEMEN FPGA core.' >&2; exit 1; }
  printf 'load_core %s\n' "$core" > /dev/MiSTer_cmd
  # Allow a forced reload even when CORENAME already says IKEMEN.
  sleep 3
  for ((i=0; i<100; i++)); do
    [ "$(cat /tmp/CORENAME 2>/dev/null)" != IKEMEN ] || break
    sleep 0.1
  done
  sleep 1
fi
cd "$WORK"
# Additional arguments support bounded diagnostics without changing the menu.
shift $(( $# > 0 ? 1 : 0 ))
# Sound is enabled by default; diagnostics may explicitly mute it.
if [ "${IKEMEN_NO_SOUND:-0}" = 1 ]; then set -- -nosound "$@"; fi
set +e
runner=()
# MiSTer's frontend is pinned to one ARM core. An explicit game mask avoids
# inheriting that restriction when launched from its Scripts menu. Leave other
# launchers unchanged unless they request a mask; arrays preserve arguments.
if [ -n "${IKEMEN_CPU_MASK:-}" ]; then
  runner=(taskset "$IKEMEN_CPU_MASK")
fi
# Apply scheduling priority only to the child engine, never the MiSTer frontend.
# The engine's worker threads inherit it and exit with the game.
engine_nice=${IKEMEN_NICE:-0}
if ! [[ "$engine_nice" =~ ^-?[0-9]+$ ]] || [ "$engine_nice" -lt -10 ] || [ "$engine_nice" -gt 19 ]; then
  echo 'IKEMEN_NICE must be an integer from -10 to 19.' >&2
  exit 1
fi
if [ "$engine_nice" -ne 0 ]; then
  runner=(nice -n "$engine_nice" "${runner[@]}")
fi
"${runner[@]}" "${IKEMEN_EXECUTABLE:-$ENGINE/Ikemen_GO.armv7}" -config "$PROFILE/config.ini" -r "$motif" "$@" > "$PROFILE/latest.log" 2>&1
result=$?
set -e
if [ "$result" -ne 0 ]; then
  tail -n 30 "$PROFILE/latest.log"
  echo "Game exited with status $result. Log: $PROFILE/latest.log"
fi
exit "$result"
