#!/bin/bash
# Install in /media/fat/Scripts. Game content is never modified.
set -euo pipefail
export IKEMEN_GAMES_DIR=${IKEMEN_GAMES_DIR:-/media/fat/games/MUGEN}
ENGINE=${IKEMEN_ENGINE_DIR:-/media/fat/games/IkemenFrontier}
exec "$ENGINE/launch-game.sh" "${1:-}"
