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
	value         uint32
	Delay, Period time.Duration
}

func (e *TimerEvent) Load() bool {
	return atomic.LoadUint32(&e.value) == 1
}

func (e *TimerEvent) Store(value bool) {
	if value {
		atomic.StoreUint32(&e.value, uint32(1))
	} else {
		atomic.StoreUint32(&e.value, uint32(0))
	}
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
