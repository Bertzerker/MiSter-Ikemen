#!/bin/bash
# Experimental renderer; shares the normal game library, settings and saves.
set -euo pipefail
ENGINE=/media/fat/games/IkemenFrontier
TEST=/media/fat/games/IkemenFrontier-test-20260922
for required in "$TEST/gpu/IKEMEN.rbf" "$TEST/Ikemen_GO.gpu.armv7" "$ENGINE/launch-game.sh"; do
  [ -f "$required" ] || { echo "Missing experimental runtime: $required" >&2; exit 1; }
done
# Do not reload the core underneath another running engine.
exec 8>/tmp/ikemen-frontier-video.lock
flock -n 8 || { echo 'Exit the running game before starting another.' >&2; exit 1; }
flock -u 8
exec 8>&-
restore_menu() {
  if [ "$(cat /tmp/CORENAME 2>/dev/null)" = IKEMEN ] && [ -f /media/fat/menu.rbf ]; then
    printf 'load_core /media/fat/menu.rbf\n' > /dev/MiSTer_cmd
  fi
}
trap restore_menu EXIT
# The shared launcher selects the game BEFORE switching away from the console.
# Force loading this core: software and hardware cores share CORENAME IKEMEN.
export IKEMEN_CORE="$TEST/gpu/IKEMEN.rbf" IKEMEN_FORCE_CORE=1
export IKEMEN_ENGINE_DIR="$ENGINE" IKEMEN_EXECUTABLE="$TEST/Ikemen_GO.gpu.armv7"
export IKEMEN_GPU=1
export IKEMEN_CPU_MASK="${IKEMEN_CPU_MASK:-3}"
bash "$ENGINE/launch-game.sh" "$@"
