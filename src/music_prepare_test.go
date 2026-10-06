package main

import (
	"context"
	"testing"
)

type closeProbe struct{ closed bool }

func (*closeProbe) Stream([][2]float64) (int, bool) { return 0, false }
func (*closeProbe) Err() error                      { return nil }
func (*closeProbe) Len() int                        { return 0 }
func (*closeProbe) Position() int                   { return 0 }
func (*closeProbe) Seek(int) error                  { return nil }
func (p *closeProbe) Close() error                  { p.closed = true; return nil }

func TestMusicPreparationOwnership(t *testing.T) {
	for _, when := range []string{"before", "during", "queued", "accept"} {
		t.Run(when, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if when == "before" {
				cancel()
			}
			probe := &closeProbe{}
			prepared, accepted := false, false
			tasks := make(chan func(), 1)
			queuePreparedMusic(ctx, tasks, func() *preparedMusic {
				prepared = true
				if when == "during" {
					cancel()
				}
				return &preparedMusic{stream: probe}
			}, func(p *preparedMusic) {
				accepted = true
				if p.stream != probe {
					t.Fatal("wrong decoder")
				}
			})
			if when == "queued" {
				cancel()
			}
			select {
			case task := <-tasks:
				task()
			default:
			}
			if when == "before" && prepared {
				t.Fatal("prepared cancelled request")
			}
			if accepted != (when == "accept") {
				t.Fatal("stale track accepted")
			}
			if prepared && probe.closed != (when != "accept") {
				t.Fatal("decoder ownership error")
			}
		})
	}
}
