package module

import (
	"time"

	"github.com/deflinhec/runtimelua/event"

	lua "github.com/yuin/gopher-lua"
)

type eventModule struct {
	RuntimeModule
}

func EventModule(runtime Runtime) *eventModule {
	return &eventModule{
		RuntimeModule: RuntimeModule{
			Module: Module{
				name: "event",
			},
			runtime: runtime,
		},
	}
}

func (m *eventModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"loop":  m.loop,
			"delay": m.delay,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

func (m *eventModule) delay(l *lua.LState) int {
	sec := l.CheckNumber(1)
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(2, "expects function")
		return 0
	}
	e := &timerEvent{
		TimerEvent: event.TimerEvent{
			Delay: time.Second * time.Duration(sec),
		},
		Func: fn,
		VM:   l,
	}
	e.Store(true)
	m.runtime.EventQueue() <- e
	l.Push(e.Self())
	return 1
}

func (m *eventModule) loop(l *lua.LState) int {
	sec := l.CheckNumber(1)
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(1, "expects function")
		return 0
	}
	e := &timerEvent{
		TimerEvent: event.TimerEvent{
			Period: time.Second * time.Duration(sec),
		},
		Func: fn,
		VM:   l,
	}
	e.Store(true)
	m.runtime.EventQueue() <- e
	l.Push(e.Self())
	return 1
}

func CheckTimerEvent(l *lua.LState, n int) *timerEvent {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*timerEvent); ok {
				return e
			}
		}
	}
	l.ArgError(n, "expect timerEvent")
	return nil
}

type timerEvent struct {
	event.TimerEvent
	self *lua.LTable
	Func *lua.LFunction
	VM   *lua.LState
}

func (e *timerEvent) Update(elapse time.Duration) error {
	e.Delay -= elapse
	if e.Delay > 0 {
		return nil
	}
	e.VM.Push(e.Func)
	e.VM.Push(e.Self())
	if err := e.VM.PCall(1, 1, nil); err != nil {
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

func (e *timerEvent) Stop() {
	e.Store(false)
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

func (e *timerEvent) Self() lua.LValue {
	if e.self != nil {
		return e.self
	}
	funcs := timerFuncs
	e.self = e.VM.SetFuncs(e.VM.CreateTable(0, len(funcs)), funcs)
	e.self.Metatable = &lua.LUserData{Value: e}
	return e.self
}
