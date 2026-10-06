#!/usr/bin/env bash
# Existing OSD controller mapping drives human P1; P2 is computer controlled.
set -u
cd "$(dirname "$0")"
if [ "$(cat /tmp/CORENAME 2>/dev/null)" != IKEMEN ]; then
  echo 'Load the IKEMEN FPGA core first.' >&2
  exit 1
fi
export SDL_VIDEODRIVER=dummy SDL_AUDIODRIVER=dummy
export IKEMEN_FRONTIER=1 GOMEMLIMIT=256MiB
unset IKEMEN_SW_DUMP_DIR
./Ikemen_GO.armv7 -config test.ini -p1 kfm -p2 kfm -p1.ai 0 -p2.ai 5 \
  -s stage0 -rounds 3 -time 99
