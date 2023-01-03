package event

import (
	"sync/atomic"
	"time"
)

type Event interface {
	Valid() bool

	Update(time.Duration) error

	Continue() bool
}

type TimerEvent struct {
	atomic.Value
	Delay, Period time.Duration
}

func (e *TimerEvent) Load() bool {
	return e.Value.Load().(bool)
}

func (e *TimerEvent) Valid() bool {
	return e.Load()
}

func (e *TimerEvent) Continue() bool {
	if !e.Load() {
		return false
	}
	return e.Delay > 0
}

func (e *TimerEvent) Update(elapse time.Duration) error {
	e.Delay -= elapse
	if e.Delay > 0 {
		return nil
	}
	e.Delay = e.Period
	return nil
}

func (e *TimerEvent) Stop() {
	e.Store(false)
}
