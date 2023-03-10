package event

import (
	"sync/atomic"
	"time"
)

type State uint32

const (
	INITIALIZE State = iota
	PROGRESS
	FINALIZE
	COMPLETE
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
	return e.Load() != uint32(COMPLETE)
}

func (e *StateEvent) Continue() bool {
	return e.Load() != uint32(COMPLETE)
}

func (e *StateEvent) Update(elapse time.Duration) error {
	switch State(e.Load()) {
	case INITIALIZE:
		e.Store(uint32(PROGRESS))
	case PROGRESS:
		e.Store(uint32(FINALIZE))
	case FINALIZE:
		e.Store(uint32(COMPLETE))
	case COMPLETE:

	}
	return nil
}

func (e *StateEvent) Stop() {
	e.Store(uint32(COMPLETE))
}
