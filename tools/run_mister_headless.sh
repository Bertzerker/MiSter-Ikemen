#!/usr/bin/env bash
# Run from the staged test directory on MiSTer. No FPGA registers are touched.
set -u
export SDL_VIDEODRIVER=dummy SDL_AUDIODRIVER=dummy
export GOMEMLIMIT=240MiB
unset IKEMEN_SW_DUMP_DIR
unset IKEMEN_FRONTIER
time -p timeout 60 ./Ikemen_GO.armv7 -config test.ini \
  -p1 kfm -p2 kfm -p1.ai 8 -p2.ai 8 -s stage0 \
  -rounds 1 -time 10 -nojoy
result=$?
echo "MATCH_EXIT=$result"
exit "$result"
