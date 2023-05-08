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

type TimerEvent struct {
	EventType
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

func (e *TimerEvent) Update(elapse time.Duration, l *lua.LState) error {
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
