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

	"go.uber.org/zap"

	lua "github.com/yuin/gopher-lua"
)

type localEventModule struct {
	localRuntimeModule

	logger *zap.Logger
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

	e := &localTimerEvent{
		TimerEvent: TimerEvent{
			Delay: time.Second * time.Duration(sec),
		},
		fn: fn,
	}

	e.Store(true)
	m.runtime.EventQueue <- e
	l.Push(e.LuaValue(l))
	return 1
}

func (m *localEventModule) loop(l *lua.LState) int {
	sec := l.CheckNumber(1)
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(1, "expects function")
		return 0
	}

	e := &localTimerEvent{
		TimerEvent: TimerEvent{
			Period: time.Second * time.Duration(sec),
		},
		fn: fn,
	}

	e.Store(true)
	m.runtime.EventQueue <- e
	l.Push(e.LuaValue(l))
	return 1
}

type localTimerEvent struct {
	TimerEvent
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
	if ret, ok := l.Get(-1).(lua.LBool); ok && ret == lua.LTrue {
		e.Delay = e.Period
	} else {
		e.Store(false)
	}
	return nil
}

func (e *localTimerEvent) Stop() {
	e.Store(false)
}

func (e *localTimerEvent) stop(l *lua.LState) int {
	l.CheckTable(1)
	e.Stop()
	return 0
}

func (e *localTimerEvent) valid(l *lua.LState) int {
	l.CheckTable(1)
	l.Push(lua.LBool(e.Valid()))
	return 1
}

func (e *localTimerEvent) LuaValue(l *lua.LState) lua.LValue {
	if e.bind != nil {
		return e.bind
	}
	functions := map[string]lua.LGFunction{
		"stop":  e.stop,
		"valid": e.valid,
	}
	e.bind = l.SetFuncs(l.CreateTable(0, len(functions)), functions)
	return e.bind
}
