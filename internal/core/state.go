package core

import "time"

type State struct {
	Enabled      bool
	PausedUntil  time.Time
	Locked       bool
	LastPhysical time.Time
	LastMove     time.Time
	Activated    time.Time
	Cycles       uint64
}

func (s State) Paused(now time.Time) bool { return s.Locked || now.Before(s.PausedUntil) }
func (s State) Idle(now time.Time) time.Duration {
	d := now.Sub(s.LastPhysical)
	if d < 0 {
		return 0
	}
	return d
}
func (s State) ShouldMove(now time.Time, idle, interval time.Duration) bool {
	return s.Enabled && !s.Paused(now) && s.Idle(now) >= idle && (s.LastMove.IsZero() || now.Sub(s.LastMove) >= interval)
}
