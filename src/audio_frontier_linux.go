//go:build softwareonly && linux

package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/ikemen-engine/Ikemen-GO/internal/misteraudio"
)

type FrontierSpeaker struct {
	mu              sync.Mutex
	mixer           beep.Mixer
	output          beep.Streamer
	device          *misteraudio.Device
	buf             [512][2]float64
	pcm             [512 * 4]byte
	stop, done      chan struct{}
	once            sync.Once
	signals         chan os.Signal
	failed          bool
	lastCoreCheck   time.Time
	framesWritten   uint64
	nonSilentFrames uint64
}

func (s *FrontierSpeaker) Init(sr beep.SampleRate, _ int) error {
	if sr <= 0 {
		return fmt.Errorf("invalid audio sample rate %d", sr)
	}
	name, err := os.ReadFile("/tmp/CORENAME")
	if err != nil || strings.TrimSpace(string(name)) != "IKEMEN" {
		return fmt.Errorf("IKEMEN core required for audio")
	}
	s.device, err = misteraudio.Open()
	if err != nil {
		return fmt.Errorf("MiSTer audio: %w", err)
	}
	_, nativeAudioDisabled = sys.cmdFlags["-nosound"]
	if nativeAudioDisabled {
		// End on silence, then avoid the mixer/resampler/normalizer worker
		// entirely. Muting volume alone still processed 48,000 samples/sec.
		err = s.device.Write(s.pcm[:])
		closeErr := s.device.Close()
		s.device = nil
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintln(os.Stderr, "[Frontier] Audio disabled: mixer and PCM worker bypassed")
		return nil
	}
	s.output = &s.mixer
	if sr != 48000 {
		s.output = beep.Resample(3, sr, 48000, &s.mixer)
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.signals = make(chan os.Signal, 1)
	signal.Notify(s.signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-s.signals:
			s.Close()
			if v, ok := sig.(syscall.Signal); ok {
				os.Exit(128 + int(v))
			}
			os.Exit(1)
		case <-s.stop:
		}
	}()
	SafeGo(func() {
		defer close(s.done)
		ticker := time.NewTicker(4 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.FillAudio()
				if s.failed {
					return
				}
			}
		}
	})
	fmt.Fprintf(os.Stderr, "[Frontier] Stereo PCM via MiSTer FPGA mixer, %d -> 48000 Hz\n", sr)
	return nil
}

func (s *FrontierSpeaker) FillAudio() {
	if s.device == nil {
		return
	}
	// Single producer. Keep less than 32 ms queued, below FPGA hurry-up thresholds.
	if time.Since(s.lastCoreCheck) > 100*time.Millisecond {
		name, err := os.ReadFile("/tmp/CORENAME")
		if err != nil || strings.TrimSpace(string(name)) != "IKEMEN" {
			s.fail(fmt.Errorf("core changed"))
			return
		}
		s.lastCoreCheck = time.Now()
	}
	pending, err := s.device.Pending()
	if err != nil {
		s.fail(err)
		return
	}
	if pending >= 1024*4 {
		return
	}
	s.mu.Lock()
	n, _ := s.output.Stream(s.buf[:])
	s.mu.Unlock()
	// Beep streamers may end with a partial block. Pad the rest with silence.
	for i := 0; i < len(s.buf); i++ {
		var l, r int16
		if i < n {
			l = floatToS16(s.buf[i][0])
			r = floatToS16(s.buf[i][1])
		}
		s.pcm[i*4] = byte(l)
		s.pcm[i*4+1] = byte(l >> 8)
		s.pcm[i*4+2] = byte(r)
		s.pcm[i*4+3] = byte(r >> 8)
		if l != 0 || r != 0 {
			s.nonSilentFrames++
		}
	}
	if err = s.device.Write(s.pcm[:]); err != nil {
		s.fail(err)
	} else {
		s.framesWritten += uint64(len(s.buf))
	}
}

func (s *FrontierSpeaker) fail(err error) {
	s.failed = true
	fmt.Fprintf(os.Stderr, "[Frontier] Audio stopped: %v\n", err)
}
func (s *FrontierSpeaker) Play(st beep.Streamer) {
	if nativeAudioDisabled {
		return
	}
	s.mu.Lock()
	s.mixer.Add(st)
	s.mu.Unlock()
}
func (s *FrontierSpeaker) Lock()   { s.mu.Lock() }
func (s *FrontierSpeaker) Unlock() { s.mu.Unlock() }
func (s *FrontierSpeaker) Close() {
	s.once.Do(func() {
		if s.stop == nil {
			return
		}
		close(s.stop)
		<-s.done
		signal.Stop(s.signals)
		// The standard FPGA mixer holds its last sample on underrun. End on zero.
		name, err := os.ReadFile("/tmp/CORENAME")
		if err == nil && strings.TrimSpace(string(name)) == "IKEMEN" {
			_ = s.device.Write(make([]byte, 512*4))
		}
		_ = s.device.Close()
		fmt.Fprintf(os.Stderr, "[Frontier] Audio closed: %d frames written, %d non-silent frames\n", s.framesWritten, s.nonSilentFrames)
	})
}
