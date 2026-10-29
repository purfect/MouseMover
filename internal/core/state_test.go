package core

import (
	"testing"
	"time"
)

func TestState(t *testing.T) {
	now := time.Now()
	s := State{Enabled: true, LastPhysical: now.Add(-time.Minute)}
	if !s.ShouldMove(now, time.Minute, 30*time.Second) {
		t.Fatal("expected move")
	}
	s.LastMove = now
	if s.ShouldMove(now, time.Minute, 30*time.Second) {
		t.Fatal("interval ignored")
	}
	s.LastMove = now.Add(-time.Minute)
	s.Locked = true
	if s.ShouldMove(now, time.Minute, 30*time.Second) {
		t.Fatal("lock ignored")
	}
	s.Locked = false
	s.PausedUntil = now.Add(time.Minute)
	if s.ShouldMove(now, time.Minute, 30*time.Second) {
		t.Fatal("pause ignored")
	}
}
