// Copyright 2023 Deflinhec
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtimelua

import (
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"
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

func (e *StateEvent) Update(d time.Duration, l *lua.LState) error {
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
