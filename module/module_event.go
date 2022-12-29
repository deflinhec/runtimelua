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
	e := &event.TimerEvent{
		Arguments: []lua.LValue{},
		Delay:     time.Second * time.Duration(sec),
		Func:      fn,
		VM:        l,
	}
	e.Store(true)
	m.runtime.EventQueue() <- e
	l.Push(e.LuaValue())
	return 1
}

func (m *eventModule) loop(l *lua.LState) int {
	sec := l.CheckNumber(1)
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(1, "expects function")
		return 0
	}
	e := &event.TimerEvent{
		Period:    time.Second * time.Duration(sec),
		Arguments: []lua.LValue{},
		Func:      fn,
		VM:        l,
	}
	e.Store(true)
	m.runtime.EventQueue() <- e
	l.Push(e.LuaValue())
	return 1
}
