// Package mistermusic provides an optional disk cache of decoded music.
package mistermusic

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

func key(source io.ReadSeeker) (string, error) {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h := sha256.New()
	_, err := io.Copy(h, source)
	_, seekErr := source.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}
	if seekErr != nil {
		return "", seekErr
	}
	return fmt.Sprintf("%x.wav", h.Sum(nil)), nil
}

// OpenCached leaves source rewound, including on a cache miss. Content hashes
// prevent a game update from selecting PCM belonging to an older music file.
func OpenCached(source io.ReadSeeker, dir string) (beep.StreamSeekCloser, beep.Format, bool) {
	if dir == "" {
		return nil, beep.Format{}, false
	}
	name, err := key(source)
	if err != nil {
		return nil, beep.Format{}, false
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, beep.Format{}, false
	}
	s, format, err := wav.Decode(f)
	if err != nil {
		return nil, beep.Format{}, false
	}
	return s, format, true
}

// Convert writes a separate, streamed 16-bit PCM file. Original sample rate and
// sample count are retained so MUGEN's sample-based loop/seek positions still fit.
func Convert(path, dir string) (output string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	name, err := key(f)
	if err != nil {
		return "", err
	}
	var s beep.StreamSeekCloser
	var format beep.Format
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ogg":
		s, format, err = vorbis.Decode(f)
	case ".mp3":
		s, format, err = mp3.Decode(f)
	default:
		return "", fmt.Errorf("unsupported source: %s", path)
	}
	if err != nil {
		return "", err
	}
	defer s.Close()
	expectedSamples := s.Len()
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	output = filepath.Join(dir, name)
	tmp, err := os.CreateTemp(dir, ".music-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	format.Precision = 2
	err = wav.Encode(tmp, s, format)
	if err == nil {
		err = s.Err()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	check, err := os.Open(tmp.Name())
	if err != nil {
		return "", err
	}
	pcm, pcmFormat, err := wav.Decode(check)
	if err != nil {
		return "", err
	}
	valid := pcm.Len() == expectedSamples && pcmFormat.SampleRate == format.SampleRate && pcmFormat.NumChannels == format.NumChannels
	actualSamples := pcm.Len()
	pcm.Close()
	if !valid {
		return "", fmt.Errorf("decoded sample count differs: source %d, PCM %d", expectedSamples, actualSamples)
	}
	if err = os.Rename(tmp.Name(), output); err != nil {
		return "", err
	}
	return output, nil
}
