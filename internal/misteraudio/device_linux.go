// Package misteraudio uses MiSTer's kernel-owned PCM ring and standard FPGA
// ALSA mixer path. Samples are 48 kHz, signed 16-bit little-endian stereo.
package misteraudio

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

const RingBytes = 512 * 1024
const DevicePath = "/dev/MrAudio"

type Device struct{ file, lock *os.File }

func Open() (*Device, error) {
	lock, err := os.OpenFile("/tmp/ikemen-frontier-audio.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, err
	}
	f, err := os.OpenFile(DevicePath, os.O_WRONLY, 0)
	if err != nil {
		lock.Close()
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		f.Close()
		lock.Close()
		return nil, fmt.Errorf("%s is not an available character device", DevicePath)
	}
	d := &Device{f, lock}
	// The FPGA intentionally discards its first write-pointer update after reset.
	// Prime that update with silence; never clear the kernel-owned ring.
	if err = d.Write(make([]byte, 8)); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func ParseStatus(text string) (int, error) {
	var read, write, pending, comp int
	n, err := fmt.Sscanf(text, "rptr: %d, wptr: %d, len: %d, comp: %d", &read, &write, &pending, &comp)
	if err != nil || n != 4 || read < 0 || read >= RingBytes || write < 0 || write >= RingBytes ||
		pending < 0 || pending >= RingBytes || read%8 != 0 || write%4 != 0 || comp < 0 || comp > 7 ||
		pending != (write-read+RingBytes)%RingBytes {
		return 0, fmt.Errorf("invalid MiSTer audio status: %q", text)
	}
	return pending, nil
}

func (d *Device) Pending() (int, error) {
	// Opening the device snapshots the FPGA's read pointer over SPI.
	f, err := os.Open(DevicePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 256))
	if err != nil {
		return 0, err
	}
	return ParseStatus(string(b))
}

func (d *Device) Write(pcm []byte) error {
	// The FPGA fetches two stereo frames per 64-bit beat.
	if len(pcm) == 0 || len(pcm)%8 != 0 || len(pcm) > RingBytes-8 {
		return fmt.Errorf("invalid stereo PCM block length %d", len(pcm))
	}
	n, err := d.file.Write(pcm)
	if err == nil && n != len(pcm) {
		err = io.ErrShortWrite
	}
	return err
}

func (d *Device) Close() error {
	err := d.file.Close()
	d.lock.Close()
	return err
}
