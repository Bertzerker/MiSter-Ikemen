//go:build linux && cgo

// Package frontiervideo publishes native-resolution frames to the IKEMEN FPGA core.
package frontiervideo

/*
#include "transport.h"
*/
import "C"

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

type Video struct{ gpu bool }

func Open(width, height int) (*Video, error) {
	if width != 320 || height != 240 {
		return nil, fmt.Errorf("Frontier requires 320x240, got %dx%d", width, height)
	}
	name, err := os.ReadFile("/tmp/CORENAME")
	if err != nil || strings.TrimSpace(string(name)) != "IKEMEN" {
		return nil, fmt.Errorf("load the IKEMEN FPGA core before enabling Frontier video")
	}
	if code := C.frontier_open(); code != 0 {
		return nil, fmt.Errorf("Frontier video handshake: %w", syscall.Errno(code))
	}
	v := &Video{}
	if os.Getenv("IKEMEN_GPU") == "1" {
		if code := C.frontier_gpu_open(); code != 0 {
			C.frontier_close()
			return nil, fmt.Errorf("Frontier GPU arena/handshake: %w", syscall.Errno(code))
		}
		v.gpu = true
	}
	return v, nil
}

func (v *Video) Present(pixels []byte, width, height, pitch int) error {
	// Fixed dimensions also bound multiplication and C integer conversion.
	if width != 320 || height != 240 || pitch != 320*4 || len(pixels) != pitch*height {
		return fmt.Errorf("invalid Frontier RGBA frame dimensions or length")
	}
	if code := C.frontier_present((*C.uint8_t)(unsafe.Pointer(&pixels[0])), C.int(width), C.int(height), C.int(pitch)); code != 0 {
		return fmt.Errorf("Frontier present: %w", syscall.Errno(code))
	}
	return nil
}

func (v *Video) Close() { C.frontier_close() }

func (v *Video) GPUAvailable() bool   { return v.gpu }
func (v *Video) GPUGroupedRows() bool { return v.gpu && C.frontier_gpu_grouped_rows() != 0 }
func bytePointer(data []byte) *C.uint8_t {
	if len(data) == 0 {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(&data[0]))
}
func (v *Video) GPUUpload(address uint32, data []byte) error {
	if code := C.frontier_gpu_upload(C.uint32_t(address), bytePointer(data), C.size_t(len(data))); code != 0 {
		return syscall.Errno(code)
	}
	return nil
}
func (v *Video) GPUFrame(commands, palettes []byte) error {
	if code := C.frontier_gpu_frame(bytePointer(commands), C.size_t(len(commands)), bytePointer(palettes), C.size_t(len(palettes))); code != 0 {
		return syscall.Errno(code)
	}
	return nil
}
func (v *Video) GPUReadback(pixels []byte) error {
	if code := C.frontier_gpu_readback(bytePointer(pixels), C.size_t(len(pixels))); code != 0 {
		return syscall.Errno(code)
	}
	return nil
}

// Buttons returns the MiSTer OSD-mapped digital bits for a logical player.
func Buttons(player int) uint32 { return uint32(C.frontier_joystick(C.int(player))) }
