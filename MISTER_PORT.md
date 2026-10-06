# MiSTer ARMv7 prototype

Validated on 2026-09-22 against leonkasovan/Ikemen-GO commit
`bf63a10d4b572b1c5f1f49a43121122d9c9b3e9d`, local branch `mister-armv7`.

## Result

The engine compiles and runs natively on the test MiSTer. It renders a 320x240
Kung Fu Man vs Kung Fu Man match using its CPU renderer. This is a **headless
prototype with working HDMI video**. The new Frontier video transport and
companion FPGA core pass simulation and a live match on the MiSTer. The user
confirmed a correct HDMI picture. Physical VGA validation is pending.
Controller and audio integration are implemented and user-confirmed on the
real MiSTer, including lightweight pause/resume. Results are recorded below
separately from the initial video test.
See [FPGA video documentation](fpga/README.md) for scaling and the memory protocol.

The test system reports Linux 6.18.38-MiSTer, ARMv7, Buildroot 2021.02.4,
glibc 2.31, 491 MiB Linux-visible RAM and approximately 407 MiB available before
the test. No existing core, launcher, or Frontier configuration was modified.
Test files are isolated in `/media/fat/games/IkemenFrontier-test-20260922`.

## Changes

- `softwareonly` build tag excludes OpenGL, Vulkan, Direct3D and GTK dialogs.
  Software font loading, renderer selection and sprite queue dispatch remain.
- `minimal` build tag omits FFmpeg video backgrounds and libxmp module music,
  without enabling the Mugen build's embedded-asset extraction behavior.
- The dedicated build script compiles SDL2 without desktop GPU dependencies,
  then builds a static ARMv7 hard-float executable with Go-native network/user
  lookups. The standard upstream build remains available without these tags.
- Staging and headless-run scripts make the test repeatable.

## Build

Run under x86-64 Ubuntu (tested in WSL). Required packages:

```sh
sudo apt-get install gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf \
  libc6-dev-armhf-cross pkg-config curl ca-certificates make cmake
bash tools/build_mister_arm.sh
bash tools/build_mister_arm.sh test
bash tools/stage_mister_test.sh /path/to/Ikemen_GO-Elecbyte-Screenpack
```

The build downloads Go 1.27.1 and SDL2 2.32.10 into
`$HOME/.cache/ikemen-mister` (override with `IKEMEN_BUILD_TOOLS`). Go modules
are pinned by the repository. Script downloads currently use HTTPS but do not
verify pinned archive checksums; tighten this before release packaging.

Outputs live under `build/mister/`:

- `Ikemen_GO.armv7`: final executable, static ELF32 ARM EABI5, VFP hard-float,
  ARMv7/NEON attributes verified using readelf.
- `renderer.test.armv7`: renderer tests executable.
- `ikemen-test.tar.gz`: executable plus standard screenpack and test runner.
- `renderer-tests.log`: 20 passing software-renderer tests from the real board.
- `frame-600.png`: rendered match frame retrieved from the real board.
- `benchmark.log`: final headless match timing and exit status.

Original headless executable SHA-256 (superseded by the video build):
`0bb42921ab88af1d09401292b7313b9dad8404258caa73855e63d69e670ceb1d`

## Validation

1. Executable starts on MiSTer and prints its help text.
2. All 20 `TestSW*` tests pass on the board, covering palette lookup, blending,
   clipping, mirrored/rotated sprites, filtered RGBA, dirty rectangles, font
   coverage and parallel row rendering.
3. A bounded, one-round AI match (10-second round timer) completes with
   `MATCH_EXIT=0`; final run: real 25.39 s, user 25.04 s, system 1.22 s.
   This includes startup, loading and round transitions and is **not an FPS
   measurement**. Audio mixing runs through dummy output; audible output was
   not tested. The first visual-capture run had audio disabled.
4. Frame 600 visually shows both fighters, stage and lifebars at 320x240.
5. Build scripts pass Bash syntax checks; source passes `git diff --check`.

Go 1.27 vet reports existing dynamic-format-string issues in upstream
`char.go`, `main.go` and `script.go`. The test build explicitly uses `-vet=off`
to run functional assertions; a clean vet result is not claimed. The static
linker warns about SDL's optional `dlopen` path; ordinary startup/matches work,
but dynamic SDL overrides and full network functionality were not tested.

On the FAT SD card, extract test archives using `tar --no-same-owner -xzf ...`.
An initial extraction reported ownership warnings; files extracted and were
used successfully. The final archive records owner/group 0.

To repeat in the test directory on MiSTer:

```sh
./renderer.test.armv7 -test.v -test.run TestSW -test.timeout 60s
bash run_mister_headless.sh
```

## Video implementation (2026-09-22)

- `IKEMEN_FRONTIER=1` selects the native video sink. SDL dummy remains in use for
  the window/audio framework; video frames bypass SDL presentation and enter
  the FPGA's acknowledged double buffer at native 320x240.
- Standard MiSTer HDMI scaling, aspect ratio and integer-scale controls are
  retained. The core's VGA Output setting can select scaled VGA or native 240p.
- A changing FPGA heartbeat and vblank acknowledgment gate ARM writes, and a
  process lock excludes multiple publishers. The original OpenBOR core is not
  compatible with this protocol.
- Local GCC/Icarus tests pass: pixel conversion/stride, buffer ownership and
  timeout, 240-line timing, 1080p/720p/480p integer-scale calculation, stalled
  DDR transactions, and two complete per-pixel-checked scanouts with aligned DE.
  The same scanout also passes with Intel's Quartus 17.0.2 `altera_mf.v` FIFO
  simulation model. Neither simulation proves hardware timing closure.
- The rebuilt executable completed another headless MiSTer match with exit 0
  (real 45.82 s, user 45.34 s, sys 1.90 s); this is not an FPS measurement.
  Both the launcher and the engine's video sink rejected the Menu core as expected.
- ARM video build SHA-256:
  `c9ba7f728e154f123534bb3a24581b7a69ca18dbaf75f28ce43ead50350fc2b1`.
- Quartus container digest:
  `ghcr.io/raetro/quartus@sha256:55442b161163dc7723e3d4917ce046fcb682478365fe9320756c9a4ae7d7d4ce`.

## Live HDMI validation

The compiled core was loaded from the isolated test directory. Its reported
identity is IKEMEN. Both protocol magic words matched, and the heartbeat
advanced by approximately 60 counts/second. During the match, control and ACK
words advanced together, demonstrating working ARM-to-FPGA presentation.
The user confirmed both fighters, stage and lifebars looked correct on HDMI.
The 30-second-round AI test completed: `VIDEO_MATCH_EXIT=0`, real 48.35 s,
user 46.89 s, sys 2.09 s. Logs: `build/mister/video-match.log` and
`build/mister/video-handshake.log` (the latter sampled after the match).

Short live samples showed approximately 14–16 published frames/second while
the output heartbeat remained approximately 60 Hz. This is a rough observation,
not a controlled FPS benchmark; performance is not yet ready to call 60-fps
gameplay. This initial test had silent audio and no controller integration. Only HDMI was
connected. Existing MiSTer.ini was read, not changed (`direct_video=0`).

Quartus 17.0.2 completed with 0 errors and 129 warnings. Worst analyzed setup
slack is +0.194 ns, hold +0.215 ns, and TNS is zero. All clocks are constrained;
21 input and 90 output ports remain unconstrained in the inherited framework,
so this is not a claim that every external I/O timing path was closed. Logic
utilization is 21%, RAM bits 8%, DSP blocks 31%.

RBF SHA-256:
`9b59f13eb49e36c0f4b59658c8126f0515c2dc42e32a062835f34c9161c64af7`.
The copy on the MiSTer matched. Output: `fpga/output_files/IKEMEN.rbf`.
No startup configuration was changed. After the bounded match exits, the updated
core retains the last completed picture; it can be tested again from the existing directory
with `bash run_mister_video.sh` while the IKEMEN core is loaded.

## Remaining milestones

Profile and improve CPU rendering/presentation. Verify scaled/native VGA with a
connected compatible display, and audit inherited external I/O timing constraints.
Measure simulation, rendering, presentation and peak memory on representative
characters.

## Controller and audio update

The ARM backend reads four OSD-mapped joystick words at offsets `0x08`, `0x18`,
`0x20`, `0x28`. The existing OSD mapping and core bitstream are retained. Its
six attack positions (currently labeled Attack, Jump, Special, Attack2, Attack3,
Attack4 in the OSD) feed IKEMEN's default A, B, C, X, Y, Z controls. Start remains
Start/taunt. **Start + Attack4** toggles pause in the lightweight match. Release
both buttons before pressing the combination again to resume. Holding the
combination does not repeatedly toggle pause, and suppresses its two gameplay
buttons. In modes with a full graphical menu, the chord maps to Menu instead.
Directions and button releases are translated
directly; the engine retains its normal SOCD handling. Four logical ports are
exposed, without physical connection detection, analog sticks or rumble. The
OSD's Swap Joysticks option remains effective before ARM reads the inputs.

Sound uses MiSTer's existing kernel `/dev/MrAudio` driver, feeding the standard
FPGA ALSA mixer and HDMI/analog audio outputs. The backend converts the Beep
mixer's output into signed 16-bit stereo and resamples the configured engine
rate to 48 kHz. It checks FPGA ring occupancy, submits 512-frame blocks with a
target queue below 32 ms, locks out a second IKEMEN audio publisher, and finishes
on silence on normal exit or SIGTERM. Core changes stop audio production.
No ALSA/SDL package installation or FPGA change is required. Other applications
must not concurrently publish into MiSTer's single global PCM ring.

References: [MiSTer kernel audio driver](https://github.com/MiSTer-devel/Linux-Kernel_MiSTer/blob/master/sound/drivers/MiSTer-audio-spi.c)
and the vendored `fpga/sys/alsa.sv` / `audio_out.v`.

Unit tests cover logical button mapping, pause chord suppression and release,
four shared-memory controller offsets, PCM ring status validation and wraparound.
The interactive test launcher is `bash run_mister_play.sh` in the staged test
directory: human P1 against CPU P2, three rounds. It removes the video test's
`-nojoy` flag. `IKEMEN_INPUT_TRACE=1` additionally records changes to the logical
controller words. The default stage has no background music; use announcer and
attack effects for sound checks.

### Live verification and memory limit

The user confirmed controller movement/attacks and audible HDMI sound, then
confirmed the final lightweight pause/resume control. The final release build
logs controller transitions and pause changes when `IKEMEN_INPUT_TRACE=1`, plus
audio initialization/errors and submitted/non-silent frame counts on shutdown.
The deployed ARM executable SHA-256 is
`19b2ae6932491de3a7e4efd0143ab9f0a7b5d3a049278515b12ee5adba20287b`.

The graphical-menu experiment (`-loadmotif`) exhausted RAM during full IKEMEN1
screenpack loading. Kernel OOM logs confirmed the process was killed. Its
512 MiB configured heap limit had overridden `GOMEMLIMIT`; reducing the setting
to 256 MiB was insufficient for this full screenpack, so the interactive launcher
retains lightweight Quick VS and direct pause/resume. The final lightweight
test showed about 113 MiB total system memory used in one live sample; this is
not a peak-memory benchmark. See the later memory/library update below for the
replacement loader and full-screenpack testing.

The staging script now sets `Debug.HeapMemoryLimit=256`, and the existing remote
test.ini was updated with a backup at `test-before-audio-menu.ini`. The Go limit
is a soft GC target, not a cap on all process memory. The user's MiSTer.ini,
OSD mapping, FPGA bitstream and startup configuration were unchanged by the
controller/audio update. Analog audio, other controllers and four-player play
remain unverified. No commits or pushes were made; source edits remain local.

## Memory and game library update (2026-09-23)

The DE10-Nano has 1 GiB DDR3, but the tested MiSTer boots with
`mem=511M memmap=513M$511M`: Linux reports 502864 KiB total RAM. The upper DDR3
region contains FPGA buffers, including the standard scaler at `0x20000000`,
the Linux framebuffer, and this port's shared transport at `0x3A000000`.
The additional 128 MiB SDRAM is connected to the FPGA, not mapped as an ordinary
Linux/Go heap. Neither boot arguments nor these hardware allocations are changed.

Software sprite uploads now share immutable decoded pixels instead of allocating
a second image. Texture mutation uses copy-on-write. With `IKEMEN_LOW_MEMORY=1`,
SFFv2 loading retains sprite headers and file offsets, decoding only on use.
Linked sprites and animation copies share the same lazy texture. A 64 MiB LRU
cache is trimmed after the frame's render queue and worker rows finish, so
eviction cannot invalidate queued draws. Evicted sprites are decoded again when
needed. PNG dimensions, palettes, transparency and native sprite coordinates are
preserved; this is not destructive asset resizing.

The cache target is not a total memory cap: active frame data can exceed it until
frame completion, and characters using SFFv1, sounds, fonts, scripts and engine
state use additional memory. SFFv1 retains its existing palette-aware loader.
Cache misses can cost frame time on large animations. The Go heap target remains
256 MiB. `bash tools/build_mister_arm.sh profile` produces a separate debug build
with a localhost-only heap profiler on port 6060; the release has no profiler.

### Choosing games

Build/package using `bash tools/build_mister_arm.sh` followed by
`bash tools/stage_mister_library.sh ../TestContent`. The standard screenpack
checkout supplies the common `data/gofx` effects pack required by the engine.
The resulting `build/mister/ikemen-library.tar.gz` contains the shared runtime
under `games/IkemenFrontier` and `Scripts/MUGEN.sh`. Extract into `/media/fat`
with `tar --no-same-owner -xzf ... -C /media/fat`.

Place each game in `/media/fat/games/MUGEN/<game name>/`, with its `data`, `chars`,
`stages`, `font` and `sound` folders. Choose **Scripts → MUGEN** and select the
game. The launcher reads the motif from `data/mugen.cfg`, falling back to
`data/system.def`, loads the IKEMEN FPGA core when needed, and starts the full
game menu. It forces 320×240 software output; the existing MiSTer HDMI/VGA scaler
remains responsible for display scaling. MUGEN 1.0 screenpacks with wider-than-4:3
local coordinates use the width-based compatibility canvas described below.

Game content remains unchanged. A temporary symlink overlay in `/tmp` merges
engine defaults with game assets (FAT itself cannot store these symlinks).
Settings, saves and the latest log are separate for each game under
`/media/fat/games/IkemenFrontier/profiles/<game name>/`. Temporary overlays are
removed on exit. A per-game lock prevents two sessions writing the same saves.
Folder names containing spaces are supported. CLI launch is also available:
`/media/fat/games/IkemenFrontier/launch-game.sh 'vjdfinal'`.

### Verification

The uploaded `vjdfinal` library is about 737 MiB on disk. The user confirmed that
its intro/title menu and text look correct over HDMI. ARM renderer tests cover
deferred decoding, shared linked sprites, eviction/reload, release accounting,
copy-on-write and the existing rasterizer checks. Large Windows MUGEN games may
still have unsupported content; the port does not execute bundled Windows
executables or plugins. VGA remains physically untested.

The full `vjdfinal` Demitri versus DivineMorrigan test reached gameplay on
`jba_tomb` with correct captured fighters, stage and lifebars. Its observed
resident high-water mark was 221048 KiB (about 216 MiB); the sampled live Go
heap was 86.74 MiB, led by SFFv1 sprite storage (46.42 MiB) and sounds (12.25 MiB).
The previous full IKEMEN1/KFM test, which was killed near 453376 KiB anonymous
RSS before these changes, now reached a match with observed high-water RSS
105400 KiB (about 103 MiB). These are bounded tests of those specific assets,
not maximums for every roster or round. The diagnostic builds were used for
these measurements. The menu selector was tested interactively: it discovered
both `mkp` and `vjdfinal`, and cancellation returned successfully. `mkp` gameplay
has not been tested.

The initial memory-optimized library release SHA-256 was
`2792b17ea14ab22a1ad6c2375f81bd1ab243eb7031440a0ba004a51858d0e3a3`.
Local validation artifacts are in `build/mister`: `heap-match.pprof`,
`vjdfinal-validation.log` and `vjdfinal-match-final.png`.

### Black-frame flicker after intros

The earlier FPGA reader blanked its output after 30 display refreshes without a
new ARM frame. On-demand artwork loading can cross that threshold, producing
black frames that disappear when loading settles. The reader now repeats the
last completed buffer for arbitrarily long publication pauses. Reset-time
suppression of stale DDR, vblank ACK ownership, timing and the MiSTer scaler are
preserved. Exiting or stopping ARM now leaves a frozen picture until the next
publication or core reload. This fixes blanking, not the CPU decoding stalls.

The reader regression holds ARM publication for 35 complete display scanouts
under DDR backpressure, checks every pixel and fails if video readiness drops.
It reproduced the blanking with the previous RTL and passed with the fix using
both the lightweight and Intel FIFO models. The rebuilt core passed compilation
with 0 errors / 129 inherited warnings, minimum setup slack +0.267 ns and hold
slack +0.213 ns, TNS zero. The deployed RBF SHA-256 is
`e1c8a5dff2c9eb4dc1e9e13ad5f7a0e9d55c63d48fcf873029f26fa0975e2c93`.
The previous installed RBF is backed up as `IKEMEN.before-frame-hold.rbf`.

### MUGEN menu coordinate compatibility

The supplied Windows screenshots revealed a separate menu/select cropping issue.
`vjdfinal` declares `localcoord=1680,960`, but its menu begins at y=696 and
continues below y=960. Its PC layout scales uniformly by width; IKEMEN instead
height-fits and center-crops the declared wide canvas. The library launcher now
normalizes the temporary overlay motif to a width-based 4:3 canvas (1680×1260
for this game). This gives backgrounds, fonts, portraits and menu positions the
same scale and fits the full screen at native 320×240. The FPGA then scales this
complete picture for HDMI/VGA. Fighter, stage and lifebar definitions are not
adjusted.

The rule applies only to a MUGEN library game with `mugenversion=1.0`, no
`ikemenversion`, and a wider-than-4:3 motif localcoord. Existing 4:3 and modern
screenpacks retain their definitions. `tools/test_mister_motif.sh` covers these
conditions and key order/quoted versions. The overlay replaces its symlink with
a generated file; the uploaded original remains byte-identical (SHA-256
`1c1c4005f4121df34be25dfe2f4cf825befaab255727dd641ad1723b2b52818e`).

### ARM performance investigation (2026-09-23)

Measure actual ARM publications with `tools/frontier_fps.c`; the roughly 60 Hz
FPGA heartbeat measures display scanout, not newly rendered game frames. This
read-only tool reports a timed publication count and observed longest pause.
Debug builds can also enable `Video.PerfLog` for drawing, presentation and logic
timings. `IKEMEN_EXECUTABLE` selects a diagnostic binary without changing the
installed launcher default.

The library launcher now mixes directly at the FPGA's 48000 Hz, avoiding a
second full-output resampling pass. It streams BGM instead of concurrently
decoding an entire track into RAM (`BGMRAMBuffer=0`). In the first 20-second
Demitri/DivineMorrigan `jba_tomb` profile, that background decode alone consumed
35.8% of sampled CPU time. Streaming also avoids that track-sized allocation.

Rendering now skips fully transparent premultiplied pixels and directly copies
opaque palette pixels. RGB565 conversion uses a cached 150 KiB staging buffer
and an aligned bulk copy to reserved video DDR instead of individual uncached
halfword stores. ACK ownership and publication barriers are preserved. Sampled
frame drawing fell from about 53 ms to 36–48 ms; presentation fell from about
16 ms to 5–7 ms. The same MiSTer HDMI/VGA scaler remains in use.

When AudioDucking is disabled, the normalizer now performs only its existing
filters, volume and clipping. Its unused adaptive gain state is frozen until
ducking is re-enabled. A regression compares 120000 samples bit-for-bit against
the original path, including silence tails and clipped input, at 22050, 44100
and 48000 Hz. The normalizer's sampled CPU share fell from 8.3% to 2.4%.

These reductions do **not** solve heavy-match performance: the same automated
match still averaged 4.85–5.30 rendered fps in 20-second diagnostic samples
(initial estimate 4.6–4.8). AI activity varies between runs; this is not a
deterministic speedup claim. Character bytecode, audio decoding/resampling and
software drawing remain substantial costs. Game logic is kept near its normal
60-update cadence by skipping pictures, rather than slowing the simulation.

Validation: all 22 ARM software-renderer tests passed; the new ARM audio-output
regression passed; host tests of conversion, buffer ownership and ACK timeout
passed; launcher syntax and motif compatibility tests passed. CPU profiles and
bounded test scripts are retained under `build/mister`.

A controlled normal-release title-menu comparison disabled attract matches in
a temporary Lua script only. Over 20 seconds after reaching the menu, the old
release published 360 frames (18.00 fps), and the optimized release published
623 (31.15 fps), about 73% more. Both samples included an initial long pause
(1.84/1.62 seconds); these averages are not steady-state minimum frame rates.
The game's original scripts and definitions were not edited by this benchmark.

The performance release SHA-256 is
`9087ffb0893bdbf7e1db04f893075b48178bbc363234c37db9a35ef72461410c`.
The previous device executable is retained as
`/media/fat/games/IkemenFrontier/Ikemen_GO.before-performance.armv7`.
The installable runtime archive is `build/mister/ikemen-library.tar.gz`.

### Optional disk music cache

MUGEN's `SFXResampleMethod`, `SFXResampleQuality` and `BGMResampleQuality` are
not consumed by IKEMEN's audio pipeline. IKEMEN uses the linked Go Beep decoders
and `Sound.AudioResampleQuality` (1 is its minimum), rather than the original
MUGEN SDL/libresample/plugin configuration. Only the currently playing codec
does decoding work; merely having both MP3 and Ogg files is not itself overhead.

On the ARM board, decoding 30 seconds of `demitri.ogg` and resampling to 48000 Hz
took 4.236 CPU seconds. A separate 16-bit WAV at the original 44100 Hz took
0.936 CPU seconds through the same resampler, about 78% less. These are isolated
audio benchmark CPU times, not an equivalent increase in game frame rate.
Preconverting to 48000 Hz did not improve this benchmark: Beep still executes
its resampler at unity ratio, so the cache retains the original sample rate.

`go run ./tools/mister-music-cache -game <game-folder> -cache <cache-folder>`
creates separate 16-bit PCM WAVs from MP3/Ogg sources. Install that cache as
`profiles/<game>/music-cache` in the shared runtime. The launcher exports
`IKEMEN_AUDIO_CACHE` only when the directory exists. Music is streamed from SD;
the cache is not loaded wholesale into the Go heap. Source-content SHA-256 keys
avoid stale cache hits after a game update. Rate, frame count and sample-based
loop/seek positions are preserved; samples are quantized to 16-bit PCM.

Cache misses retain the original decoder. Failed offline decodes are reported
and skipped, with no partial WAV installed. The supplied Donovan ending MP3
(`chars/Donovan/donend.mp3`) and `chars/KoldRaptor/ZabEND.mp3` trigger Huffman
decode errors and therefore remain uncached. `sound/Egypt.mp3` and
`sound/Runaway-Train.mp3` each decoded 1152 fewer samples than the source decoder
reported; these also remain uncached. The converter now rejects such length
mismatches before publishing a cache entry. The final cache covers 31 of 35
tracks, using about 580 MiB of additional SD space. Every accepted track was
checked against its original rate, channel count and sample count. Unit tests
cover Ogg/MP3 sample counts, rates, seek positions, PCM
precision and invalidation when source bytes change. Uploaded game definitions,
music and the user's `mugen.cfg` edits are preserved.

The cached-music Demitri/DivineMorrigan match published 141 frames in 20.002
seconds (7.05 fps), versus 4.85–5.30 fps in earlier uncached diagnostic runs.
This remains a demanding, non-deterministic AI match and is not yet comfortably
playable. Drawing averaged about 34–36 ms and presentation 4.5–4.8 ms. Its
profile confirmed that the Ogg decoding cost was removed. The final cache-enabled
release SHA-256 is
`3564723517f8b4aadf803674818c379c4400d0f961a8aff2fc2af18839cddc70`.

The startup-sequence comparison with the same render optimizations measured
625 frames / 20.002 s (31.25 fps) using original MP3 playback and 725 frames /
20.002 s (36.25 fps) using the cache. Compared with the initial 18 fps menu
sample, the combined changes roughly doubled this measured average. These
samples still included initial 1.58/1.84-second pauses; asset/loading stalls
and low match frame rates remain unresolved performance limitations. This test
was initially labeled a title-menu comparison, but the marker was subsequently
found to precede logo/intro playback. These numbers do not describe a settled
title menu.

## Experimental FPGA row renderer (2026-09-24)

An opt-in `IKEMEN_GPU=1` backend now encodes flat, indexed and nearest-neighbor
RGBA rows for an FPGA rasterizer. The existing MiSTer scanout, HDMI/VGA scaling,
audio and controller interfaces remain in place. Unsupported frames replay on
the software renderer with previous-frame readback when necessary. This work
is available through the separate experimental `MUGEN_FPGA.sh` launcher.

All blend/raster/DMA simulations pass, including scanout under competing DDR
traffic. ARM tests compare all eight blend modes across the three source types
and verify ordered fallback. The first Quartus fit exposed duplicated framebuffer
storage; sharing the raster/export read port corrects synthesis to one 2457600-bit
framebuffer. Synchronous palette RAM further reduces the design to 10109 ALMs
(24%) and 366 RAM blocks (66%). That version fits but fails timing (-14.385 ns),
so it was not deployed. Texture sampling has since been pipelined and its pixel
simulations pass. Further registering of the memory/blend path closes timing:
system setup slack +0.874 ns and HDMI +0.400 ns, at 24% logic and 67% RAM blocks.
The on-board test matches all 76800 pixels in each of three indexed/RGBA/mirrored
frames (about 23 ms per submission plus readback). Real-game performance is still
under investigation; this is not enabled in the installed runtime.
The build and packaging scripts reject negative timing slack.

Real-game tests now cover the settled title menu and a heavy AI match. Palette
deduplication prevents the title from exhausting its upload arena. The ARM
encoder reuses fallback records and immutable palette lookups, and allows one
GPU frame to overlap game logic while fencing buffer reuse/readback/shutdown.
The full ARM renderer suite passed, and subsequent GPU regressions validate
two CPU-side packets so preparation overlaps FPGA execution. New texture
uploads, packet reuse, arena recycling, fallback, readback and shutdown retain
explicit ordering. Latest samples are 5.45 fps in the title and 9.45 fps in the
match, versus refreshed software samples of 5.00 and 5.40 fps. Two preceding
hardware match runs measured 9.85 and 8.95 fps; AI variability limits precision.
This remains experimental; FPGA throughput and ARM tile preparation still need
work. `IKEMEN_GPU_ASYNC=0` selects synchronous submission for comparisons.

The 4 KiB texture-cache core passes timing (HDMI +0.559 ns, system +0.593 ns)
and all four board pixel comparisons, including changed data at a reused
texture address. The cache alone has not shown a clear performance gain;
overlapping preparation/execution is the main improvement. Return to MiSTer's
menu and run `Scripts/MUGEN_FPGA.sh` to use the shared game chooser with this
build. Normal `MUGEN.sh` and its stable runtime remain unchanged.

See `../DOCS/IKEMEN_HARDWARE_RENDERING.md` for reference research, memory layout,
command protocol and validation details. `tools/test_frontier_gpu_board.c`
provides the first bounded pixel comparison on actual hardware; it requires an
IKV2 core and never replaces the installed runtime or core file.

## Performance testing policy (user request, 2026-09-24)

Skip logos and game intros using the menu override or direct match launch.
Default to one accelerated run: one second settling, six seconds measuring FPS,
and stop AI matches within ten seconds. An independent cutoff stops the engine
at eight seconds after readiness detection (polled once per second).
Do not collect profiles or repeat software baselines routinely. If a short sample
shows no clear FPS gain, stop testing that change and target the next optimization.
Profile only to investigate a specific bottleneck, within the same match budget.
Compare equivalent scenes and measurement windows; older 20-second samples are
context, not precise comparisons with the new short samples.
## Launcher and graphics-only testing (2026-09-25)

The FPGA wrapper now passes its core path to the shared launcher, which waits
until after game selection before loading the core. Previously the early core
load hid the console chooser. The renderer options display the launcher-selected
MiSTer mode and no longer offer desktop OpenGL/Vulkan choices on Frontier.
Those desktop backends are excluded from the softwareonly ARM build; FPGA work
uses the custom row renderer, with ARM fallback, and the MiSTer display scaler.

Sound effects and music are temporarily disabled for both launchers via -nosound.
Set IKEMEN_NO_SOUND=0 to restore audio for subsequent launches. Establish a fresh
short no-sound baseline before attributing FPS differences to graphics changes.
Launcher shell syntax and options Lua syntax were checked; no game was launched
for this update. Game chooser interaction still needs a user display check.

### Latest experimental graphics build (2026-09-25)

MUGEN_FPGA now uses rectangular UV hoisting, exact transparent/opaque FPGA blend
shortcuts, and a profile-guided ARM build from existing CPU captures. With sound
disabled and intros skipped, six-second menu samples rose from 5.83 to 6.66 fps
across the combined changes. This is modest progress, not playable-speed
performance; gameplay FPS was not remeasured. ARM pixel/fence/UV tests, focused
FPGA raster/full-frame DMA simulations, FPGA timing and four board frame pixel
comparisons passed. HDMI/VGA scaler and clock settings are unchanged. Stable
MUGEN engine/core files are unchanged. See the hardware-rendering notes for
artifact hashes, exact measurements and the opt-in PGO build command.

### CPU drawing update (2026-09-25)

The experimental ARM runtime now caches unchanged transforms/last-used palettes,
avoids double-precision rectangle bounds and repeated pixel-range calculations,
and reuses immediate drawing state with less copying/clearing of GPU fallback
records. ARM drawing/fallback/cache-invalidation tests pass. Six-second menu
samples improved from 6.66 to 7.00, then 8.33 fps with state reuse (about 25%
combined). No gameplay FPS claim: no match was needed for this comparison.
Installed behind MUGEN_FPGA, with sound off and the same FPGA/scaler settings.
Details and artifact hashes are in the hardware-rendering notes.

### Latest CPU preparation update (2026-09-25)

MUGEN_FPGA now also reuses the last GPU texture/palette lookup, avoids copying
whole matrices for each vertex, and uses a cheaper exact pixel-boundary rounding
path. Six-second menu samples improved from 8.33 to 9.66 fps across this round
(about 16%); the rounding change provided most of the measured gain. ARM TestSW
passed with new coordinate, boundary and resource-cache invalidation checks.
Audio is still disabled; no AI match or FPGA rebuild was used for this round.
See hardware-rendering notes for measurements, hashes and the saved binary.

### Latest gameplay check (2026-09-25)

Two intro-free vjdfinal AI samples lasted six seconds each. Shared tile setup
measured 16.16 fps; disabling it measured 18.00 fps with similar FPGA work
(28.7 ms/frame). The change remains opt-in (IKEMEN_SW_TILE_BATCH=1), as it did
not demonstrate a gain. Grouped-row commands likewise remain opt-in
(IKEMEN_GPU_GROUP_ROWS=1) after a slower menu sample. Both defaults retain the
faster measured paths. The IKG2 core passed timing and full-frame hardware pixel
checks and supports the original command format. User retested the black-screen
report successfully, and both AI tests launched; no black-screen fix is claimed.
Audio remains off. These gameplay FPS values must not be compared directly with
the earlier menu measurements.

### SvCEX GO missing menu text (2026-09-25)

The launcher selected data/system.def, whose font references (font2.FNT,
sfz2a_system.fnt, sfz2a_bfont.fnt, sfzbigname.fnt) could not resolve. The actual
assets were already present under data/SvCEX/font; the bundled screenpack
 data/SvCEX/system.def references them correctly. Added an optional one-line
profiles/<game>/motif.txt override to launch-game.sh, set to
 data/SvCEX/system.def for SvCEX GO. Both stable and FPGA launchers share this
selection. Original game assets remain unchanged; the prior launcher and game
log were backed up. Shell syntax checked. An intro-free brief menu check and
readback of the actual FPGA framebuffer confirmed visible ARCADE, SURVIVAL,
VERSUS, VS CPU and NETWORK text. Saved image: build/mister/svc-menu.bmp.
The previous log also contained a gameplay GPU timeout; this font-selection
correction does not claim to fix that separate error.

### Actual no-sound fast path (2026-09-25)

MUGEN_FPGA now skips the audio worker and effect stream creation when -nosound
is enabled. Previously silent effects still consumed mixing, resampling and
normalization CPU time. One intro-free six-second vjdfinal AI test reached
25.81 fps versus the preceding baseline's 18.00 fps (about 43% higher in these
short samples); longest publication pause dropped from 215.9 to 73.2 ms. FPGA
work stayed near 29 ms/frame. ARM disabled-audio and renderer checks passed.
Audio-enabled behavior, FPGA/scaler and stable engine are unchanged. The gain
applies to the current sound-disabled testing mode. See hardware-rendering notes
for hashes, evidence and the rejected character-state experiment.

### Faster FPGA texture sampling (2026-09-26)

MUGEN_FPGA now uses a validated core that reuses texture-row address calculations
and reads texture-cache hits one stage earlier. All six board pixel comparisons
pass. A single intro-free six-second vjdfinal AI sample improved from 25.81 to
29.31 fps (about 14%); FPGA work dropped from 28.96 to 23.33 ms/frame (19%).
These short AI samples are not deterministic full-match benchmarks. Testing
stopped after the bounded sample and the board returned to MENU.

An equivalent scaler line-end comparison removes a timing bottleneck without
changing any counter decision, cycle, clock or scaling setting. Exhaustive
comparison covers all 16,777,216 uint12 pairs. Final seed-5 timing passes:
HDMI +0.277 ns, system +0.606 ns. The prior experimental core is backed up;
stable runtime/core and ARM executable are unchanged. Audio remains off.
See hardware-rendering notes for build hashes, rejected placements and evidence.

### Display-wait overlap and small rotated effects (2026-09-26)

MUGEN_FPGA now uses compatible IKG3 core/engine builds. The FPGA draws before
the prior frame's display acknowledgement, but checks exact ownership before
export. Simulation, production transport tests on ARM, six full-frame board
pixel comparisons and unchanged timing requirements pass. Older IKG1/2 cores
retain the original ARM-side wait. Previous core/engine pairs are backed up.

The first bounded AI sample was 30.82 fps versus 29.31 previously, a modest
change with workload variation. Small indexed rotated/trapezoid effects can
now be encoded as exact flat FPGA spans instead of forcing whole-frame CPU
replay. ARM tests cover coverage, blend modes, replay after source mutation and
capacity rollback. The next six-second sample reached 34.83 fps, but did not
exercise this path (generic-spans=0), so it proves no speedup from span encoding.
A larger generic draw still fell back; the longest observed pause was 104.1 ms.
No longer tests or repeats were run. Device returned to MENU, audio remains off,
and MiSTer scaling, clock rates and memory allocation are preserved. See the
hardware-rendering notes for exact hashes, logs and per-core CPU observations.

### Larger indexed effects and launcher CPU affinity (2026-09-26)

The experimental engine now samples generic indexed effects into compact index
images instead of flat-color spans. This supports up to the full viewport while
bounding command count by covered rows, preserving exact coverage, sampling and
all eight blend modes. ARM TestSW checks pass, including original-state replay
after texture/palette mutation. The existing IKG3 core needs no changes.

One six-second intro-free AI sample measured 29.83 fps, 88.0 ms maximum observed
publication pause, and 23.27 ms average GPU completion (including display wait).
The log recorded 239 hardware frames, one software frame and zero generic-image
draws at its last counter report. This sample establishes no speedup from the
new path; the previous 34.83 fps sample had a different AI workload. No repeat
was run to seek a better number. The candidate remains installed for its wider
supported draw coverage; performance on those effects remains unmeasured.

MUGEN_FPGA explicitly requests CPU mask 3 through the shared launcher's optional
IKEMEN_CPU_MASK setting. Starting the benchmark from mask 2 still produced engine
Cpus_allowed_list 0-1. This prevents inheritance of a single-core restriction;
it does not parallelize game logic. Prior SSH benchmarks already allowed both
cores. The ordinary launcher leaves this setting unset. The board is back at
MENU, audio remains off, and the display scaler and FPGA bitstream are unchanged.

### FPGA pixel pipeline update (2026-09-26)

MUGEN_FPGA now uses a shorter palette/blend pipeline. A deterministic cached
256-pixel indexed row drops from 1,798 to 1,542 cycles (14.24% less time).
Pixel, blend, transport and arbitration simulations pass, including exhaustive
RGBA rounding equivalence. The first fit missed timing and was not deployed;
shortening that arithmetic path produced a passing build with +0.219 ns system
and +0.339 ns HDMI setup slack. Six board frames match every RGBA pixel.

One six-second AI sample measured 25.16 fps versus the preceding 29.83 fps,
with a 579.8 ms publication stall. Average GPU completion fell from 23.27 to
21.67 ms, but includes display waiting and uses a different AI workload. Thus
the pixel-path saving is proven in simulation, while a gameplay FPS gain is
not. No additional match was run. Six generic indexed images were used without
whole-frame fallback; the stall's cause is unresolved. The next priority is
locating that long stall in ARM game logic, draw preparation or resource loading
rather than assuming it is FPGA raster time. Board returned to MENU.

The validated core remains installed for its shorter exact drawing paths;
gpu/IKEMEN.before-pixel-pipeline.rbf holds the prior core. Engine, clocks,
resolution, MiSTer scaling and audio-off configuration are unchanged. Exact
hashes and evidence are in DOCS/IKEMEN_HARDWARE_RENDERING.md in the parent folder.

### Sound restored and audio CPU work reduced (2026-09-26)

The shared launcher now enables sound by default; IKEMEN_NO_SOUND=1 remains an
explicit diagnostic override. The experimental engine prepares OGG/MP3 music
off the main thread on Frontier when BGMRAMBuffer is disabled (the launcher
default). One worker opens/hashes the source and prepares its cached PCM or
decoder, then hands it to the main thread. New music, Stop and video-music
requests cancel stale results. Playback may begin after preparation completes.
Both ARM cores remain available to the scheduler; no dedicated CPU pinning is
claimed. The PCM mixer already had its own worker.

Stereo resampling shares identical interpolation weights between channels,
preserving their original summation order and quality. Exact reference tests
cover buffer boundaries, EOF, quality 1/3/6/16 and live ratio changes. ARM tests
also cover preparation cancellation and decoder ownership. A 512-sample ARM
benchmark drops from 1.856440 to 1.021445 ms (45.0% less time).

The single six-second sound-on match measured 11.66 fps, with non-silent PCM
confirmed. Background preparation took 1,112 ms but no longer blocked the
game-thread BGM-open operation. This is not an overall FPS improvement: audio
mixing accounted for 13.93% cumulative sampled CPU and state execution 33.68%.
Testing stopped after that sample. The normal experimental launcher retains
sound and these audio optimizations; the board returned to MENU.

Darkstalkers reuses the old vjdfinal PCM cache through its optional profile file
music-cache-path.txt. No music files are copied or recompressed. Stable engine
and FPGA core remain unchanged; the shared sound-on launcher affects both game
launchers. Exact hashes and logs are recorded in the hardware-rendering notes.

## ARM arithmetic pass (2026-09-26)

Audio mixer accumulation now copies the first voice instead of adding it to
zero. Fixed-gain normalizer filter denominators are computed once per audio
block. Quality-1 resampling uses explicit two-point weights with the same
sample arithmetic. Frontier can additionally enable a factored quality-3
polynomial, reducing its nested divisions; this is the same interpolation
curve with different floating-point rounding (reference tolerance 1e-12).
Other callers retain the original quality-3 arithmetic by default.

Graphics bounds use float32 min/max; ordinary filtered-sample floors and hue
wrapping avoid float64 math calls; absolute value clears the float32 sign bit.
Exceptional/extreme floor/mod inputs retain the original math path. This is
an arithmetic optimization, not a complete NEON conversion. Float64 audio
interfaces, adaptive ducking, and some less common graphics helpers remain.

Validation: full beep package tests pass locally. ARM tests passed for the
first pass, including renderer tests, 100,000 randomized helper values,
normalizer output, mixer behavior and quality-3 resampling. ARM benchmarks:
quality 3, 512 stereo samples: original 1.858782 ms, factored 0.628804 ms;
previous stereo-sharing build measured 1.021445 ms in an earlier run.
Hue modulo: original 732.6 ns, float32 122.1 ns. These are microbenchmarks.
The game's actual profile uses quality 1, so quality-3 gains do not apply to it.

A single intro-free, sound-on six-second AI measurement of the first pass
returned 10.66 FPS (64 frames), with a 1465.3 ms maximum publication gap.
No significant overall improvement demonstrated; testing stopped. PCM output
included 407,026 non-silent frames. The CPU profile was empty/unusable; no new
CPU percentage claims. Catch-up frame skips remain frequent in the log.

First pass deployed to the experimental engine, SHA256
98d3d43ef54a967c053374dccd262c51e2fe87a8d0a76d2925e050ca718f1562.
Rollback is Ikemen_GO.before-arithmetic.armv7 in the experimental board folder.
The subsequent quality-1 addition passed local tests and cross-compilation:
build/mister/Ikemen_GO.arithmetic-linear.armv7, BuildID
b1c240fffcc2e6e1704183facc790e0856f93ff5.
Its compressed binary and test executable are staged on the board, but final
ARM benchmarking/deployment was deferred because an active game held the
publisher lock. The quality-1 addition is not installed yet. No second AI
match was run. Sound stays enabled and the FPGA core was not changed.

Evidence: build/mister/arithmetic-arm-tests.txt, arithmetic-match-result.txt,
match-arithmetic.log, arithmetic-linear-arm.txt. Resume the staged final
benchmark/deployment with build/mister/finish-arithmetic.sh once the game exits.

## Unit-rate audio follow-up (2026-09-26)

Completed the previously deferred quality-1 ARM checks and deployment after
user testing ended. Linear resampling benchmark: original 256,738 ns versus
205,666 ns per 512 stereo frames (19.9% less time relative to the original
reference, not relative to every intervening optimization).

Then added Frontier opt-in block copying when the resampling ratio is exactly
one and the current sample position is integral. The copy stops at the existing
lookahead boundary and retains both history buffers, preserving later dynamic
ratio changes. Fractional positions still interpolate. Finite PCM is assumed;
negative zero can be preserved rather than normalized to positive zero.

Full beep package tests passed locally. ARM tests passed for unit-rate/ratio
transitions, EOF, several buffer sizes and qualities, stereo interpolation,
and mixing. Unit-rate ARM benchmark for 512 stereo samples: interpolation
207,289 ns; copy 15,593 ns (13.3x faster, 92.5% less time in this operation).
This is not an overall engine speedup.

Installed experimental engine: build/mister/Ikemen_GO.unit-audio.armv7,
BuildID 4919d5ae3439851bc1457a11034fbea897a22ae6, SHA256
9cb88b3389e67ebeb6ff673053488d258fd71fbc0bd5d1088b77532a4d929db5.
Board rollback: Ikemen_GO.before-unit-audio.armv7. FPGA and stable engine
unchanged. Evidence: build/mister/unit-audio-arm.txt.

One six-second sound-on AI match after deployment measured 10.66 FPS
(64 frames / 6.005 seconds), maximum publication gap 1177.8 ms. No significant
FPS gain; no further match tests. Audio wrote 461,289 non-silent frames.
A usable three-second CPU profile recorded StateBlock.Run 36.42% cumulative,
BytecodeExp.run 22.19% cumulative, Resampler.Stream 9.27% cumulative/5.30% flat,
and fixed normalizer 1.99% flat. Nested percentages overlap and this is a short
sample, not a controlled before/after CPU comparison. Character-state work
remains a larger measured bottleneck than resampling. Evidence:
unit-audio-match.txt, match-unit-audio.log, unit-audio-cpu.pprof,
unit-audio-cpu-top.txt in build/mister. Test restored MENU; sound remains on.

## Character-state copy removal (2026-09-27)

StateBlock.Run now has a pointer receiver. Ordinary state blocks no longer
copy their complete structure on every trigger/controller execution. Loop
blocks still run on a private copy so counters cannot leak across executions
or shared character states. Compiler insertion sites take shallow snapshots,
preserving the previous value-interface semantics. Nested hitpause dispatch
and persistent-change correction recognize the new pointer controller type.

ARM tests cover persistent counters, run-once locking, hitpause and explicit
ignorehitpause, false-trigger else/state-change propagation, repeated for loops,
and compiler snapshot independence. A false-trigger microbenchmark measured
458.6 ns for the value-copy reference versus 284.8 ns for pointer execution
(37.9% less time). This is not a whole-state or whole-engine speedup.

Installed experimental binary build/mister/Ikemen_GO.state.armv7:
BuildID bc835041534596046d34f31e831d40bbd39c2d5c;
SHA256 bf4da48ce4cd4be91b227e99463279cadcf53ffb3aa29597f49a5aabcaaeb02c.
Rollback on board: Ikemen_GO.before-state.armv7. Sound remains enabled;
FPGA core unchanged. Evidence: build/mister/state-arm-tests.txt.

Single intro-free, sound-on six-second AI sample: 11.17 FPS (67 frames),
maximum publication gap 269.8 ms. Previous sample was 10.66 FPS; these short
non-deterministic runs do not establish a significant FPS gain. Testing stopped.
The three-second profile shows runtime.duffcopy 1.65% flat (previous 6.95%),
StateBlock.run 32.67% cumulative and BytecodeExp.run 24.42% cumulative.
Nested percentages overlap. Copy overhead is reduced; trigger bytecode remains
the next state-execution target. PCM output includes 544,231 non-silent frames.
Test restored MENU. Evidence: state-match.txt, match-state.log,
state-cpu.pprof, state-cpu-top.txt in build/mister.

## Matched audio and frontend contention audit (2026-09-27)

Added opt-in IKEMEN_WAIT_PROFILE=1 to debug builds for mutex and block profiles.
Default remains off. Diagnostic binary BuildID
92ed92defe6c73879f8babb2dc2baab5b75b65c4, SHA256
dd2bfc13151bcf45213529aff6503251aadfc6ca742b18b059361a788e198d8d
is installed experimentally; rollback Ikemen_GO.before-waits.armv7.

Matched six-second tests of the same binary/characters/stage (AI not seeded):

| Measurement | Muted | Sound on |
| --- | ---: | ---: |
| CPU + mutex + block profile collection | 19.49 FPS | 6.83 FPS |
| No profile requests, wait profiling disabled | 28.99 FPS | 12.83 FPS |

Profile collection materially perturbs this resource-constrained target;
future FPS tests must not fetch profiles in their measurement window.
The clean sound-on run still has a 1112.5 ms publication gap: profiler activity
is not the complete explanation for long gaps. Do not claim that stall solved.
Cached music was confirmed active. The cache retains source sample rates;
not every cached track qualifies for the unit-rate resampler bypass.

Sound-on mutex profile (3.07 seconds): only 0.69 ms attributed to
sync.Mutex.Unlock, about 0.71 ms cumulative under FrontierSpeaker.FillAudio.
No evidence of a major game-thread audio mutex stall in this sample. Block
profile is dominated by timer/channel waits, not proof of active CPU work.
CPU: state execution 32.63% cumulative, bytecode 19.30%, resampling 9.82%.
These overlap and describe the sampled process, not whole-system utilization.

A one-second /proc snapshot while IKEMEN ran recorded IKEMEN 105.8% and
MiSTer frontend 79.2% CPU (100% = one core). The snapshot itself used CPU;
it is evidence of substantial frontend competition, not a steady-state budget.
A separate clean sound-on test with frontend nice=10 measured 18.16 FPS,
versus 12.83 at nice=0 (41.5% higher in this short, non-deterministic sample).
This makes CPU scheduling/frontend polling a stronger next target than
another tiny arithmetic change, but is not a confirmed reproducible gain.
The frontend restarted during core transitions; restoration of the old PID
failed. The new PID was found at nice=10 and explicitly restored to nice=0,
verified in frontend-priority-restored.txt. The test script now resolves the
current frontend PID during cleanup. No persistent priority/affinity change.

Games reside on /media/fat mounted exFAT, which is case-insensitive and
case-preserving (Microsoft exFAT specification). The engine additionally has
case-insensitive lookup fallback. Missing fonts cannot simply be attributed
to Linux filename case. Source music is already decoded to PCM by the optional
cache; SND is an effect bank, not a supported BGM replacement in this loader.

Created tools/mugen-audit.py with read-only JSON reporting and explicit
--fix-case repairs with backups. Tested preservation of byte encodings,
comments/CRLF, ambiguous candidates, metadata labels and changed-source
rejection. Audited all six uploaded games without modifying assets. Reports
are in build/mister/game-audits and on the experimental board directory.
Unresolved references include backup/inactive configs and unsupported lookup
contexts; these are review findings, not a count of broken games. Refer to
tools/MUGEN_AUDIT.md for scope, operation and the existing PCM cache builder.

Evidence: paired-audio-results.txt, paired-{cpu,mutex,block}-top.txt,
clean-audio-results.txt, priority-audio-results.txt, paired-system-cpu.json,
frontend-priority-restored.txt, game-audit-summary.txt in build/mister.
Sound is still enabled by default. No music references or game assets changed.
