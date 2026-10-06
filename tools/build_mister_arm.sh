#!/usr/bin/env bash
# ARMv7 CPU-rendering build with optional Frontier video, input and audio.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
TOOLS=${IKEMEN_BUILD_TOOLS:-$HOME/.cache/ikemen-mister}
mkdir -p "$TOOLS" "$ROOT/build/mister"
if [ ! -x "$TOOLS/go/bin/go" ]; then
  curl -fL --retry 2 https://go.dev/dl/go1.27.1.linux-amd64.tar.gz -o "$TOOLS/go.tar.gz"
  tar -xzf "$TOOLS/go.tar.gz" -C "$TOOLS"
fi
export PATH="$TOOLS/go/bin:$PATH"
if [ ! -f "$TOOLS/sdl-arm-install/lib/libSDL2.a" ]; then
  curl -fL --retry 2 https://www.libsdl.org/release/SDL2-2.32.10.tar.gz -o "$TOOLS/sdl.tar.gz"
  tar -xzf "$TOOLS/sdl.tar.gz" -C "$TOOLS"
  mkdir -p "$TOOLS/sdl-arm"
  cd "$TOOLS/sdl-arm"
  "$TOOLS/SDL2-2.32.10/configure" --host=arm-linux-gnueabihf \
    --prefix="$TOOLS/sdl-arm-install" --disable-shared --enable-static \
    --disable-video-x11 --disable-video-wayland --disable-video-opengl \
    --disable-video-opengles --disable-video-vulkan --disable-video-kmsdrm \
    --disable-alsa --disable-pulseaudio --disable-pipewire --disable-jack \
    --disable-esd --disable-sndio > configure.log 2>&1
  make -j4 > build.log 2>&1
  make install > install.log 2>&1
fi
cd "$ROOT"
export GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 GOEXPERIMENT=arenas
export CC=arm-linux-gnueabihf-gcc CXX=arm-linux-gnueabihf-g++
export PKG_CONFIG_LIBDIR="$TOOLS/sdl-arm-install/lib/pkgconfig"
export CGO_CFLAGS="-O2 -mcpu=cortex-a9 -mfpu=neon -mfloat-abi=hard"
export CGO_LDFLAGS="$(pkg-config --static --libs sdl2)"
# Explicit opt-in keeps ordinary builds independent of local profiling files.
PGO=${IKEMEN_PGO:-off}
if [ "${1:-}" = test ]; then
  # Upstream currently fails Go 1.27's printf vet checks; run the renderer
  # assertions independently. This does not claim a clean vet result.
  go test -pgo="$PGO" -vet=off -c -tags 'desktop softwareonly minimal netgo osusergo' \
    -ldflags '-linkmode external -extldflags "-static"' \
    -o build/mister/renderer.test.armv7 ./src
  exit
fi
TAGS='desktop softwareonly minimal netgo osusergo'
OUTPUT=build/mister/Ikemen_GO.armv7
if [ "${1:-}" = profile ]; then
  TAGS="$TAGS debug"
  OUTPUT=build/mister/Ikemen_GO.profile.armv7
fi
go build -pgo="$PGO" -tags "$TAGS" -trimpath \
  -ldflags '-linkmode external -extldflags "-static"' \
  -o "$OUTPUT" ./src
file "$OUTPUT"
