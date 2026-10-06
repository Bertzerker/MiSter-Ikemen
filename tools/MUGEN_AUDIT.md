# MUGEN asset audit

Requires Python 3. Run against an unpacked game folder. Audit is read-only:

```text
python tools/mugen-audit.py /path/to/game --report /path/to/report.json
```

The report lists explicit asset references from `.def`, `.cfg`, and `.ini`,
case mismatches, ambiguous resolutions, unresolved paths, case-colliding
filenames, music-format counts and file hashes for proposed edits.

To repair only unambiguous case mismatches in explicit paths:

```text
python tools/mugen-audit.py /path/to/game --report /path/to/repair.json --fix-case
```

Do this with the game stopped. Every changed text file is backed up under a
timestamped `.mugen-repair-*` directory within the game. Restore those files
to their original relative locations to undo the edits. Files are replaced
individually; a filesystem error can leave a partially applied repair, with
all originals preserved in the backup. The tool does not rename assets.
Report output should be a new file outside the game assets.

An unresolved reference is a review item, not proof the asset is absent:
the engine may use other search roots or a runtime-generated path. Multiple
candidates are never auto-selected. Fallback-directory case findings are
report-only. ZIPs, CNS/Lua expressions and bare `select.def` character entries
are not parsed. Unsupported fonts, malformed archives, scripts and executable
compatibility require separate diagnosis. The engine already has an ASCII
case-insensitive fallback, so case correction alone is not a universal fix.

## Music preparation

The existing `tools/mister-music-cache` converter writes separate 16-bit PCM
WAV files for OGG/MP3 tracks, keyed by source content. It preserves original
sample rates/counts so sample-based loop positions stay valid. Launch with
`IKEMEN_AUDIO_CACHE` pointing at that cache, or use the launcher's per-profile
cache path. It needs no edits to original music references. Example from the
repository root using the configured Go toolchain:

```text
go run ./tools/mister-music-cache -game /path/to/game -cache /path/to/cache
```

MUGEN `.snd` is an indexed sound-effect bank, not a drop-in BGM file for this
engine's music loader. Converting already cached music to it will not remove
the mixer/resampler cost. A future 48 kHz cache would need explicit loop/seek
position conversion; blindly resampling and replacing references is unsafe.

Tests: `python tools/test_mugen_audit.py`.
