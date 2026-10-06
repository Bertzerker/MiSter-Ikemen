# IKEMEN Frontier video core

Experimental 320x240 RGB565 display core for the ARM CPU renderer in
`leonkasovan/Ikemen-GO`. Source derives from
[MiSTer OpenBOR](https://github.com/MiSTerOrganize/MiSTer_OpenBOR_7533), commit
`dd210a6082da2b82d89e01ab824b6cb7f356083b`. The inherited FPGA files retain their
copyright notices and GPL-3.0 license. The independent ARM transport is MIT.

## Output scaling

ARM renders and transfers 320x240. The core feeds native RGB and timing to the
standard MiSTer `sys` video pipeline, including `video_freak` aspect and integer
size calculation and the HDMI scaler. The source is not squeezed to 224 lines.

- **Aspect Ratio**: Original (4:3), Full Screen, ARC1, ARC2.
- **Scale**: Normal, V-Integer, Narrower HV-Integer, Wider HV-Integer.
- **VGA Output**: Native selects approximately 15.7 kHz / 59.92 Hz 240p for a
  compatible CRT. Scaled asserts `VGA_SCALER`, selecting MiSTer's scaler output
  at the configured output resolution. Global `vga_scaler=1` also selects this
  path. With `direct_video=0`, HDMI uses the standard scaler regardless of this
  VGA selector. The test MiSTer already has `direct_video=0`.
- CRT horizontal/vertical position controls adjust sync porches.

VGA Native does not currently implement a separate forced-scandoubler path;
use VGA Scaled for a monitor requiring a scaled/higher scan-rate signal.
The user confirmed correct HDMI output during a real match. Analog output still
needs verification with a connected compatible display.

## Protocol v1

The experimental hardware-rendering build identifies its scanout as `IKV2`
(`0x494B5632`). The ARM transport accepts either version for CPU-rendered
frames. Hardware rendering additionally requires a live `IKG1`, `IKG2` or `IKG3` rasterizer
mailbox and the explicit `IKEMEN_GPU=1` setting. Its indexed/RGBA sprite
rasterizer writes the same 320x240 buffers, retaining the HDMI/VGA scaling,
controller and audio paths described above. Unsupported draws cause an ordered
CPU replay of the whole frame.

`IKG2` (`0x494B4732`) adds exact four-row packets. Each packet remains 64 bytes;
word 0 bits 5:4 hold the additional row count (0–3), and words 13–15 hold the
exact fixed-point V coordinates for the additional rows. Each row restarts U
and advances the destination by 320 pixels. Grouped rows are limited to 320
pixels each, cannot clear, and must fit completely in the framebuffer. The
ARM encoder groups rectangular UV draws and flat fills; other draws retain
single-row packets. New engines detect the capability and retain the original
format on IKG1 cores. Grouping is disabled by default after a slower menu test;
`IKEMEN_GPU_GROUP_ROWS=1` opts into the experimental format.

`IKG3` (`0x494B4733`) retains those commands and adds deferred export ownership.
Mailbox +8 bit 0 selects the output buffer; bit 1 requests deferred checking,
and +12 contains the full prior published scanout word. The FPGA draws into
private block RAM immediately, then checks scanout ACK and IKV2 magic before
exporting any output. The target must be opposite the expected displayed buffer.
A missing ACK returns error 5 after approximately 250 ms without publishing a
new frame. Legacy options 0/1 with +12 zero remain supported. New ARM transport
uses the original wait-before-submit path on IKG1/2. Deploy IKG3 with a compatible
engine: older engines do not recognize its capability magic. Reported IKG3 GPU
cycles include any deferred ownership wait. No clocks, scaling or DDR layout
change. Seed-5 timing passes at +0.111 ns HDMI and +0.357 ns system setup slack.

The rasterizer now has a 4 KiB texture cache with frame invalidation. ARM can
prepare the next command packet while the FPGA executes, fencing before
transport writes or buffer reuse. This backend is available in the separate
experimental `MUGEN_FPGA.sh` launcher; the installed software-rendering runtime
remains the stable version. Protocol, reserved DDR layout, timing
results and board validation are documented in
[`IKEMEN_HARDWARE_RENDERING.md`](../../DOCS/IKEMEN_HARDWARE_RENDERING.md).

Constant-V rows reuse their clamped texture-row product, and texture-cache hits
read one stage earlier. Refills retain a synchronous read-after-write bubble.
The scaler's horizontal line-end comparison is algebraically shortened to
remove its increment carry chain from the vertical counter enable path; all
16,777,216 uint12 input pairs are equivalent (`tools/test_scaler_counter.py`).
No cycle, clock, scaling mode or output timing is changed. The preceding IKG2 seed-5 build
passes timing with +0.277 ns HDMI and +0.606 ns system setup slack. Six board
pixel comparisons pass; a six-second vjdfinal AI sample improved from 25.81 to
29.31 fps, with FPGA work falling from 28.96 to 23.33 ms/frame. These are short,
non-deterministic gameplay samples, not a full-game performance guarantee.

Indexed affine/trapezoid effects up to the full 320x240 viewport can be sampled
exactly on ARM into compact indexed images, with one FPGA command per covered
row segment. Blending remains on FPGA; uncovered pixels are not submitted.
Unsupported sources retain the correctness fallback. The encoder passes ARM
pixel/replay tests, including large and crossing triangles; the short gameplay
sample did not exercise it, so its live performance benefit is not yet measured.

The raster pipeline reads indexed palette entries directly from the selected
texture-cache byte during lookup, and feeds the blend pipeline from the stable
synchronous framebuffer output. This removes one cycle for cached indexed
replacement and two for indexed general blending, without changing arithmetic
or command layout. The deterministic warm-cache 256-pixel replacement test
drops from 1,798 to 1,542 cycles. Regression checks enforce the cycle budget as
well as exact pixels, including cache refills and adjacent palette indices.
RGBA premultiplication uses an equivalent byte-carry rounding expression to
shorten the multiply/round path; the raster test checks all 65,536 input pairs.

Physical DDR base is `0x3A000000`, matching Frontier's reserved region. The ARM
driver requires `/tmp/CORENAME=IKEMEN`, a live heartbeat, and a matching ACK
before writing. It locks out a second ARM publisher. It must not be used with
the original OpenBOR RBF.

| Offset | Owner | Meaning |
|---|---|---|
| `0x00000` | ARM | Frame sequence in bits 31:2, buffer index in bit 0; bit 1 zero |
| `0x00040` | ARM | Buffer 0, 153,600 bytes, row-major little-endian RGB565 |
| `0x40040` | ARM | Buffer 1, same layout |
| `0xE0000` | FPGA | Last control word latched at vblank |
| `0xE0004` | FPGA | ACK magic `0x494B5631` (`IKV1`) |
| `0xE0008` | FPGA | Vblank heartbeat counter |
| `0xE000C` | FPGA | Heartbeat magic `0x494B5631` |

At reset, the FPGA acknowledges the existing control word as a baseline without
displaying stale RAM. ARM fills the opposite buffer, issues a memory barrier,
then publishes its new control word. It waits for that word's ACK before reusing
the other buffer. FPGA acknowledges only when selecting a buffer at vblank,
before preloading its scanlines. Two changing heartbeats prevent stale memory
from being mistaken for a running core. A missing ACK times out after 250 ms.

After the first completed frame, the reader keeps scanning the acknowledged
buffer until ARM publishes another. There is no half-second inactivity blanking:
asset decoding, GC, paused games and a stopped process retain the last picture.
Reset/core reload still hides old DDR contents until a fresh frame arrives.
The display timing and MiSTer HDMI/VGA scaler path are unchanged.

The ARM input backend reads the existing OSD-mapped joystick words for four
logical players. The seven-button OSD mapping is retained. Audio uses the
kernel `/dev/MrAudio` PCM ring and this core's standard `sys/alsa.sv` mixer path;
the inherited OpenBOR audio ring remains disabled. No FPGA rebuild or controller
remapping is required for the ARM controller/audio update. OpenBOR replay
UI/status writeback is disabled.

## Build and test

From the engine root on Windows with Docker running:

```powershell
docker pull ghcr.io/raetro/quartus:mister
./tools/build_mister_fpga.ps1
```

The container is the Quartus MiSTer image referenced by the
[official ao486 build instructions](https://github.com/MiSTer-devel/ao486_MiSTer).
Record the downloaded image digest with the build results. Inspect
`output_files/IKEMEN.sta.rpt` before deploying `output_files/IKEMEN.rbf`.

On Linux/WSL with GCC and Icarus Verilog installed:

```sh
bash tools/test_frontier_video.sh
```

Tests cover RGB565 conversion/stride, buffer ownership and timeout, 240-line
timing, integer-scale dimensions, vblank ACK and heartbeat, DDR backpressure,
and complete scanouts with unique per-pixel patterns. The test FIFO is a
behavioral model. Setting `QUARTUS_SIM_LIB` to Quartus's `eda/sim_lib` directory
also runs the scanout test against Intel's `altera_mf.v` FIFO model. Both models
passed the 35-frame publication-pause regression on 2026-09-23; the previous
reader failed by blanking after 30 refreshes. These tests do not replace Quartus timing analysis or
physical HDMI/VGA checks. The user's test display is HDMI only.

Changes to inherited scanout: preload readiness before FIFO overflow, correct
first-pixel advancement, no extra next-line prefetch at the final column,
matching sync/DE pipeline delay, and holding the last completed image while ARM
is loading instead of blanking after 30 refreshes. The scaler divider's unused remainder
port is explicitly open; its arithmetic FSM block is named for simulation.
