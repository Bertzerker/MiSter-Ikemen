//go:build softwareonly && linux

package main

import "testing"

func TestFrontierDisabledAudio(t *testing.T) {
	old := nativeAudioDisabled
	nativeAudioDisabled = true
	defer func() { nativeAudioDisabled = old }()
	s := &FrontierSpeaker{}
	s.Play(nil)
	s.FillAudio()
	s.Close()
	s.Close()
	if s.stop != nil || s.done != nil || s.device != nil || s.framesWritten != 0 {
		t.Fatal("disabled speaker started work")
	}
	channel := &SoundChannel{}
	// An undecoded sound must never be accessed in the disabled path.
	channel.Play(&Sound{}, 0, 0, -1, 1, 0, 0, 0)
	if channel.IsPlaying() || channel.ctrl != nil || channel.streamer != nil {
		t.Fatal("disabled effects created a playback stream")
	}
	channels := &SoundChannels{}
	if channels.Play(&Sound{}, 0, 0, 100, 0, 0, 0, 0) {
		t.Fatal("disabled effect claimed a channel")
	}
	(&Char{}).playSound(nil)
}
