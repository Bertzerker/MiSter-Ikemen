#!/usr/bin/env bash
# Run from the staged MiSTer test directory after loading IKEMEN.rbf.
set -u
if [ "$(cat /tmp/CORENAME 2>/dev/null)" != IKEMEN ]; then
  echo 'Load the IKEMEN FPGA core first.' >&2
  exit 1
fi
export SDL_VIDEODRIVER=dummy SDL_AUDIODRIVER=dummy
export IKEMEN_FRONTIER=1 GOMEMLIMIT=240MiB
unset IKEMEN_SW_DUMP_DIR
time -p timeout 90 ./Ikemen_GO.armv7 -config test.ini \
  -p1 kfm -p2 kfm -p1.ai 8 -p2.ai 8 -s stage0 \
  -rounds 1 -time 30 -nojoy
result=$?
echo "VIDEO_MATCH_EXIT=$result"
exit "$result"
