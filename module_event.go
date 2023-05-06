package runtimelua

import (
	"time"

	"github.com/deflinhec/runtimelua/event"

	lua "github.com/yuin/gopher-lua"
)

type localEventModule struct {
	localRuntimeModule
}

func (m *localEventModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"loop":  m.loop,
			"delay": m.delay,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

func (m *localEventModule) delay(l *lua.LState) int {
	sec := l.CheckNumber(1)
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(2, "expects function")
		return 0
	}

	timer := l.CreateTable(0, len(localTimerEventFuncs))
	timer = l.SetFuncs(timer, localTimerEventFuncs)
	e := &localTimerEvent{
		TimerEvent: event.TimerEvent{
			Delay: time.Second * time.Duration(sec),
		},
		bind: timer, fn: fn,
	}
	timer.Metatable = &lua.LUserData{Value: e}

	e.Store(true)
	m.runtime.eventQueue <- e
	l.Push(timer)
	return 1
}

func (m *localEventModule) loop(l *lua.LState) int {
	sec := l.CheckNumber(1)
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(1, "expects function")
		return 0
	}

	timer := l.CreateTable(0, len(localTimerEventFuncs))
	timer = l.SetFuncs(timer, localTimerEventFuncs)
	e := &localTimerEvent{
		TimerEvent: event.TimerEvent{
			Period: time.Second * time.Duration(sec),
		},
		bind: timer, fn: fn,
	}
	timer.Metatable = &lua.LUserData{Value: e}

	e.Store(true)
	m.runtime.eventQueue <- e
	l.Push(timer)
	return 1
}

func checkLocalTimerEvent(l *lua.LState, n int) *localTimerEvent {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*localTimerEvent); ok {
				return e
			}
		}
	}
	l.ArgError(n, "expect timerEvent")
	return nil
}

var localTimerEventFuncs = map[string]lua.LGFunction{
	"stop":  local_timer_event_stop,
	"valid": local_timer_event_valid,
}

func local_timer_event_stop(l *lua.LState) int {
	e := checkLocalTimerEvent(l, 1)
	if e == nil {
		return 0
	}
	e.Stop()
	return 0
}

func local_timer_event_valid(l *lua.LState) int {
	e := checkLocalTimerEvent(l, 1)
	if e == nil {
		return 0
	}
	l.Push(lua.LBool(e.Valid()))
	return 1
}

type localTimerEvent struct {
	event.TimerEvent
	bind *lua.LTable
	fn   *lua.LFunction
}

func (e *localTimerEvent) Update(elapse time.Duration, l *lua.LState) error {
	e.Delay -= elapse
	if e.Delay > 0 {
		return nil
	}
	l.Push(e.fn)
	l.Push(e.bind)
	if err := l.PCall(1, 1, nil); err != nil {
		return err
	}
	defer l.Pop(1)
	if ret, ok := l.Get(-1).(lua.LBool); ok && ret == true {
		e.Delay = e.Period
	} else {
		e.Store(false)
	}
	return nil
}

func (e *localTimerEvent) Stop() {
	e.Store(false)
}
