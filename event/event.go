package event

import (
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type Event interface {
	Valid() bool

	Update(time.Duration) error

	Continue() bool
}

type TimerEvent struct {
	atomic.Bool
	self          *lua.LTable
	Arguments     []lua.LValue
	Delay, Period time.Duration
	Func          *lua.LFunction
	VM            *lua.LState
}

const (
	STOPPED = "__stopped"
)

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
	e.VM.Push(e.Func)
	if e.self == nil {
		e.VM.Push(e.LuaValue())
	} else {
		e.VM.Push(e.self)
	}
	for _, argument := range e.Arguments {
		e.VM.Push(argument)
	}
	if err := e.VM.PCall(len(e.Arguments)+1, 1, nil); err != nil {
		return err
	}
	defer e.VM.Pop(1)
	if ret, ok := e.VM.Get(-1).(lua.LBool); ok && ret == true {
		e.Delay = e.Period
	} else {
		e.Store(false)
	}
	return nil
}

func (e *TimerEvent) Stop() {
	e.Store(false)
}

func CheckTimerEvent(l *lua.LState, n int) *TimerEvent {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*TimerEvent); ok {
				return e
			}
		}
	}
	l.ArgError(n, "expect TimerEvent")
	return nil
}

var timerFuncs = map[string]lua.LGFunction{
	"stop":  timer_stop,
	"valid": timer_valid,
}

func timer_stop(l *lua.LState) int {
	e := CheckTimerEvent(l, 1)
	if e == nil {
		return 0
	}
	e.Stop()
	return 0
}

func timer_valid(l *lua.LState) int {
	e := CheckTimerEvent(l, 1)
	if e == nil {
		return 0
	}
	l.Push(lua.LBool(e.Valid()))
	return 1
}

func (e *TimerEvent) LuaValue() lua.LValue {
	if e.self != nil {
		return e.self
	}
	funcs := timerFuncs
	e.self = e.VM.SetFuncs(e.VM.CreateTable(0, len(funcs)), funcs)
	e.self.Metatable = &lua.LUserData{Value: e}
	return e.self
}
