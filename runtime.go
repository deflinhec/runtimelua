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
	"context"
	"sort"
	"sync"
	"time"

	"github.com/deflinhec/runtimelua/auxlib"
	"github.com/deflinhec/runtimelua/luaconv"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

type Runtime struct {
	sync.RWMutex
	logger *zap.Logger
	vm     *lua.LState
	wg     *sync.WaitGroup

	script      string
	evaluate    func(interface{})
	preloads    map[string]Module
	auxlibs     map[string]lua.LGFunction
	ctx         context.Context
	ctxCancelFn context.CancelFunc
	scripts     ScriptModule
	EventQueue  chan Event
}

func NewRuntime(scripts *localScriptModule, options ...Option) *Runtime {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	ctx, ctxCancelFn := context.WithCancel(context.Background())
	r := &Runtime{
		logger: logger,
		vm: lua.NewState(lua.Options{
			CallStackSize:       128,
			RegistryGrowStep:    128,
			RegistryMaxSize:     128,
			SkipOpenLibs:        true,
			IncludeGoStackTrace: true,
		}),
		script:      "main",
		scripts:     scripts,
		ctx:         ctx,
		ctxCancelFn: ctxCancelFn,
		wg:          &sync.WaitGroup{},
		preloads:    make(map[string]Module),
		auxlibs:     make(map[string]lua.LGFunction),
		EventQueue:  make(chan Event, 128),
	}
	for _, option := range options {
		option.apply(r)
	}
	scripts.runtimes <- r
	return r
}

func NewRuntimeWithConfig(scripts ScriptModule, opts lua.Options, options ...Option) *Runtime {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	ctx, ctxCancelFn := context.WithCancel(context.Background())
	r := &Runtime{
		logger:      logger,
		vm:          lua.NewState(opts),
		script:      "main",
		ctx:         ctx,
		ctxCancelFn: ctxCancelFn,
		wg:          &sync.WaitGroup{},
		preloads:    make(map[string]Module),
		auxlibs:     make(map[string]lua.LGFunction),
		EventQueue:  make(chan Event, 128),
	}
	for _, option := range options {
		option.apply(r)
	}
	return r
}

func (r *Runtime) Startup() {
	stdlibs := map[string]lua.LGFunction{
		lua.BaseLibName:      lua.OpenBase,
		lua.TabLibName:       lua.OpenTable,
		lua.OsLibName:        auxlib.OpenOs,
		lua.IoLibName:        lua.OpenIo,
		lua.StringLibName:    lua.OpenString,
		lua.MathLibName:      lua.OpenMath,
		lua.CoroutineLibName: lua.OpenCoroutine,
		lua.LoadLibName:      r.scripts.OpenPackage(),
	}
	loadlibs := make([]string, 0, len(stdlibs))
	for name, lib := range stdlibs {
		r.vm.Push(r.vm.NewFunction(lib))
		r.vm.Push(lua.LString(name))
		r.vm.Call(1, 0)
		if len(name) > 0 {
			loadlibs = append(loadlibs, name)
		}
	}
	preloadlibs := make([]string, 0, len(r.preloads)+1)
	preloadlibs = append(preloadlibs, "runtime")
	module := lua.LGFunction(func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"exit": r.exit,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	})
	r.vm.PreloadModule("runtime", module)
	for name, lib := range r.auxlibs {
		r.vm.PreloadModule(name, lib)
		preloadlibs = append(preloadlibs, name)
	}
	for name, module := range r.preloads {
		r.vm.PreloadModule(name, module.Open())
		preloadlibs = append(preloadlibs, name)
	}
	r.logger.Debug("Runtime information",
		zap.Strings("load", loadlibs),
	)
	r.logger.Debug("Runtime information",
		zap.Strings("preload", preloadlibs),
	)
	r.wg.Add(1)
	go r.process()
	r.vm.SetContext(r.ctx)
	init := lua.LString(r.script)
	req := r.vm.GetGlobal("require").(*lua.LFunction)
	if err := r.vm.GPCall(req.GFunction, init); err != nil {
		defer r.ctxCancelFn()
		if r.evaluate == nil {
			r.logger.Error("script", zap.Error(err))
		} else {
			r.evaluate(err)
		}
	} else if r.evaluate != nil {
		if ret := r.vm.Get(-1); ret != lua.LNil {
			r.evaluate(luaconv.LuaValue(ret))
		}
	}
}

func (r *Runtime) Shutdown() {
	r.ctxCancelFn()
}

type localExitEvent struct {
	context.CancelFunc
	StateEvent
}

func (e *localExitEvent) Update(d time.Duration, l *lua.LState) error {
	e.StateEvent.Update(d, l)
	e.CancelFunc()
	return nil
}

func (r *Runtime) exit(l *lua.LState) int {
	e := &localExitEvent{
		CancelFunc: r.ctxCancelFn,
	}
	r.EventQueue <- e
	return 0
}

func (r *Runtime) fatal() {
	e := &localExitEvent{
		CancelFunc: r.ctxCancelFn,
	}
	r.EventQueue <- e
}

func (r *Runtime) process() {
	var e Event
	eventUpdateTime := time.Now()
	eventQueue := make(EventSequence, 0)
	eventSwapQueue := make(EventSequence, 0)

IncommingLoop:
	for {
		select {
		case <-r.ctx.Done():
			break IncommingLoop
		case <-time.After(time.Millisecond * 66):
			sort.Sort(eventQueue)
			r.Lock()
			r.vm.Pop(r.vm.GetTop())
			elpase := time.Since(eventUpdateTime)
			eventUpdateTime = time.Now()
			for len(eventQueue) > 0 {
				e, eventQueue = eventQueue[0], eventQueue[1:]
				if !e.Valid() {
					continue
				} else if err := e.Update(elpase, r.vm); err != nil {
					if r.evaluate == nil {
						r.logger.Error("runtime event", zap.Error(err))
					} else {
						r.evaluate(err)
					}
					break IncommingLoop
				} else if e.Continue() {
					eventSwapQueue = append(eventSwapQueue, e)
				}
			}
			eventQueue, eventSwapQueue = eventSwapQueue, eventQueue
			r.Unlock()
		case <-time.After(time.Second):
			r.logger.Debug("EventQueue information",
				zap.Int("total", len(eventQueue)),
			)
		case e = <-r.EventQueue:
			eventQueue = append(eventQueue, e)
		}
	}
	r.vm.Close()
	r.wg.Done()
}
