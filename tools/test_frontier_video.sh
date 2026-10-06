#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p build/mister
gcc -std=c11 -Wall -Wextra -Werror tools/test_frontier_transport.c -o build/mister/transport-test
build/mister/transport-test
gcc -std=c11 -O2 -Wall -Wextra -Werror tools/test_frontier_gpu_transport.c -o build/mister/gpu-transport-test
build/mister/gpu-transport-test
iverilog -g2012 -s timing_tb -o build/mister/timing-test fpga/test/timing_tb.sv fpga/rtl/openbor_video_timing.sv
vvp build/mister/timing-test
iverilog -g2012 -s scale_tb -o build/mister/scale-test fpga/test/scale_tb.sv fpga/sys/video_freak.sv fpga/sys/math.sv
vvp build/mister/scale-test
iverilog -g2012 -s reader_tb -o build/mister/reader-test fpga/test/reader_tb.sv fpga/test/fifo_model.sv fpga/rtl/openbor_video_timing.sv fpga/rtl/openbor_video_reader.sv fpga/rtl/openbor_video_top.sv
vvp build/mister/reader-test
iverilog -g2012 -DGPU_ARBITRATION -s reader_tb -o build/mister/reader-arbiter-test fpga/test/reader_tb.sv fpga/test/fifo_model.sv fpga/rtl/openbor_video_timing.sv fpga/rtl/openbor_video_reader.sv fpga/rtl/openbor_video_top.sv fpga/rtl/ikemen_gpu_arbiter.sv
vvp build/mister/reader-arbiter-test
iverilog -g2012 -s gpu_blend_tb -o build/mister/gpu-blend-test fpga/test/gpu_blend_tb.sv fpga/rtl/ikemen_gpu_blend.sv
vvp build/mister/gpu-blend-test
iverilog -g2012 -s gpu_raster_tb -o build/mister/gpu-raster-test fpga/test/gpu_raster_tb.sv fpga/rtl/ikemen_gpu_raster.sv fpga/rtl/ikemen_gpu_blend.sv
vvp build/mister/gpu-raster-test
iverilog -g2012 -s gpu_dma_tb -o build/mister/gpu-dma-test fpga/test/gpu_dma_tb.sv fpga/rtl/ikemen_gpu_dma.sv fpga/rtl/ikemen_gpu_arbiter.sv fpga/rtl/ikemen_gpu_raster.sv fpga/rtl/ikemen_gpu_blend.sv
vvp build/mister/gpu-dma-test
if [[ -n "${QUARTUS_SIM_LIB:-}" ]]; then
  iverilog -g2012 -s reader_tb -o build/mister/reader-vendor-test fpga/test/reader_tb.sv \
    "$QUARTUS_SIM_LIB/altera_mf.v" fpga/rtl/openbor_video_timing.sv \
    fpga/rtl/openbor_video_reader.sv fpga/rtl/openbor_video_top.sv
  vvp build/mister/reader-vendor-test
  iverilog -g2012 -DGPU_ARBITRATION -s reader_tb -o build/mister/reader-arbiter-vendor-test fpga/test/reader_tb.sv \
    "$QUARTUS_SIM_LIB/altera_mf.v" fpga/rtl/openbor_video_timing.sv \
    fpga/rtl/openbor_video_reader.sv fpga/rtl/openbor_video_top.sv fpga/rtl/ikemen_gpu_arbiter.sv
  vvp build/mister/reader-arbiter-vendor-test
fi
