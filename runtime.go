package runtimelua

import (
	"context"
	"sync"
	"time"

	"github.com/deflinhec/runtimelua/auxlib"
	"github.com/deflinhec/runtimelua/event"
	"github.com/deflinhec/runtimelua/module"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

type Module interface {
	Name() string
	Open() lua.LGFunction
	Initialize(*zap.Logger)
}

type RuntimeModule interface {
	Module
	// Provide Initialization
	InitializeRuntime(module.Runtime)
}

type ScriptModule interface {
	Reload() error

	Hotfix(name string) error

	MD5Sum() map[string]string
}

type Runtime struct {
	sync.Mutex
	logger *zap.Logger
	vm     *lua.LState
	wg     *sync.WaitGroup

	script      string
	preloads    map[string]Module
	auxlibs     map[string]lua.LGFunction
	ctx         context.Context
	ctxCancelFn context.CancelFunc
	scripts     *module.ScriptModule
	eventQueue  chan event.Event
}

func NewRuntime(options ...Option) *Runtime {
	logger, _ := zap.NewDevelopment()
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
		ctx:         ctx,
		ctxCancelFn: ctxCancelFn,
		wg:          &sync.WaitGroup{},
		preloads:    make(map[string]Module),
		auxlibs:     make(map[string]lua.LGFunction),
		eventQueue:  make(chan event.Event, 128),
	}
	for _, option := range options {
		option.apply(r)
	}
	return r
}

func NewRuntimeWithConfig(config Config, options ...Option) *Runtime {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	ctx, ctxCancelFn := context.WithCancel(context.Background())
	r := &Runtime{
		logger:      logger,
		vm:          lua.NewState(config.GetLuaOptions()),
		script:      config.GetScriptEntry(),
		ctx:         ctx,
		ctxCancelFn: ctxCancelFn,
		wg:          &sync.WaitGroup{},
		preloads:    make(map[string]Module),
		eventQueue:  make(chan event.Event, config.GetEventQueueSize()),
	}
	for _, option := range options {
		option.apply(r)
	}
	return r
}

func (r *Runtime) Script() ScriptModule {
	return r.scripts
}

func (r *Runtime) EventQueue() chan event.Event {
	return r.eventQueue
}

func (r *Runtime) Logger() *zap.Logger {
	return r.logger
}

func (r *Runtime) Wait() {
	r.wg.Wait()
}

func (r *Runtime) Startup() {
	r.scripts = module.NewScriptModule(r)
	r.scripts.Initialize(r.logger)
	for name, lib := range map[string]lua.LGFunction{
		lua.BaseLibName:      lua.OpenBase,
		lua.TabLibName:       lua.OpenTable,
		lua.OsLibName:        auxlib.OpenOs,
		lua.IoLibName:        lua.OpenIo,
		lua.StringLibName:    lua.OpenString,
		lua.MathLibName:      lua.OpenMath,
		lua.CoroutineLibName: lua.OpenCoroutine,
		lua.LoadLibName:      r.scripts.OpenPackage(),
	} {
		r.vm.Push(r.vm.NewFunction(lib))
		r.vm.Push(lua.LString(name))
		r.vm.Call(1, 0)
		r.logger.Debug("load", zap.String("lib", name))
	}
	for name, lib := range map[string]lua.LGFunction{
		auxlib.JsonLibName: auxlib.OpenJson,
	} {
		r.vm.PreloadModule(name, lib)
		r.logger.Debug("preload", zap.String("lib", name))
	}
	for name, module := range r.preloads {
		r.vm.PreloadModule(name, module.Open())
		module.Initialize(r.logger)
		r.logger.Debug("preload", zap.String("module", name))
	}
	r.wg.Add(1)
	go r.process()
	r.vm.SetContext(r.ctx)
	init := lua.LString(r.script)
	req := r.vm.GetGlobal("require").(*lua.LFunction)
	if err := r.vm.GPCall(req.GFunction, init); err != nil {
		r.logger.Error("script", zap.Error(err))
		r.ctxCancelFn()
	}
}

func (r *Runtime) process() {
	var e event.Event
	eventUpdateTime := time.Now()
	eventQueue := make([]event.Event, 0)
	eventSwapQueue := make([]event.Event, 0)

IncommingLoop:
	for {
		select {
		case <-r.ctx.Done():
			break IncommingLoop
		case <-time.After(time.Millisecond * 66):
			r.Lock()
			r.vm.Pop(r.vm.GetTop())
			elpase := time.Since(eventUpdateTime)
			eventUpdateTime = time.Now()
			for len(eventQueue) > 0 {
				e, eventQueue = eventQueue[0], eventQueue[1:]
				if !e.Valid() {
					continue
				} else if err := e.Update(elpase); err != nil {
					r.logger.Error("runtime event", zap.Error(err))
				} else if e.Continue() {
					eventSwapQueue = append(eventSwapQueue, e)
				}
			}
			eventQueue, eventSwapQueue = eventSwapQueue, eventQueue
			r.Unlock()
		case <-time.After(time.Second):
			r.logger.Debug("event queue tick",
				zap.Int("total", len(eventQueue)),
			)
		case e = <-r.eventQueue:
			eventQueue = append(eventQueue, e)
			r.logger.Debug("event queue received",
				zap.Int("total", len(eventQueue)),
			)
		}
	}
	r.wg.Done()
}
