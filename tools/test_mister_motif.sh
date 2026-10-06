#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
check() {
  local input=$1 expected=$2 actual
  actual=$(printf '%s\n' "$input" | awk -f "$ROOT/tools/mugen-width-coordinates.awk" | sed -n 's/^localcoord = //p' | cut -d ';' -f 1 | sed 's/ *$//')
  [ "$actual" = "$expected" ] || { echo "Expected $expected, got $actual" >&2; exit 1; }
}
check $'[Info]\nmugenversion = 1.0\nlocalcoord = 1680,960' '1680,1260'
check $'[Info]\nlocalcoord = 1680,960\nmugenversion = "1.0" ; comment' '1680,1260'
check $'[Info]\nmugenversion = 1.0\nlocalcoord = 320,240' '320,240'
check $'[Info]\nmugenversion = 1.1\nlocalcoord = 1680,960' '1680,960'
check $'[Info]\nmugenversion = 1.0\nikemenversion = 1.0\nlocalcoord = 1680,960' '1680,960'
echo 'PASS: MUGEN width canvas, key ordering, quoted version, 4:3 and modern-screenpack preservation'
