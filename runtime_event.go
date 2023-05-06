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

type EventQueue []Event

func (s EventQueue) Len() int {
	return len(s)
}

func (s EventQueue) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EventQueue) Less(i, j int) bool {
	return s[i].Priority() < s[j].Priority()
}
