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
	"time"

	lua "github.com/yuin/gopher-lua"
)

type Event interface {
	Valid() bool

	Update(time.Duration, *lua.LState) error

	Continue() bool

	Priority() int
}

type EventType struct{}

func (p *EventType) Valid() bool {
	return true
}

func (p *EventType) Priority() int {
	return 1
}

func (p *EventType) Update(time.Duration, *lua.LState) error {
	return nil
}

func (p *EventType) Continue() bool {
	return false
}

type EventSequence []Event

func (s EventSequence) Len() int {
	return len(s)
}

func (s EventSequence) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EventSequence) Less(i, j int) bool {
	return s[i].Priority() < s[j].Priority()
}
