package runtimelua

import (
	"sync/atomic"
	"time"
)

type EventState uint32

const (
	EVENT_STATE_INITIALIZE EventState = iota
	EVENT_STATE_PROGRESS
	EVENT_STATE_FINALIZE
	EVENT_STATE_COMPLETE
)

type StateEvent struct {
	EventType
	value uint32
}

func (e *StateEvent) Load() uint32 {
	return atomic.LoadUint32(&e.value)
}

func (e *StateEvent) Store(value uint32) {
	atomic.StoreUint32(&e.value, value)
}

func (e *StateEvent) Valid() bool {
	return e.Load() != uint32(EVENT_STATE_COMPLETE)
}

func (e *StateEvent) Continue() bool {
	return e.Load() != uint32(EVENT_STATE_COMPLETE)
}

func (e *StateEvent) Update(elapse time.Duration) error {
	switch EventState(e.Load()) {
	case EVENT_STATE_INITIALIZE:
		e.Store(uint32(EVENT_STATE_PROGRESS))
	case EVENT_STATE_PROGRESS:
		e.Store(uint32(EVENT_STATE_FINALIZE))
	case EVENT_STATE_FINALIZE:
		e.Store(uint32(EVENT_STATE_COMPLETE))
	case EVENT_STATE_COMPLETE:

	}
	return nil
}

func (e *StateEvent) Stop() {
	e.Store(uint32(EVENT_STATE_COMPLETE))
}
