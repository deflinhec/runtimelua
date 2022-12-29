package runtimelua

import (
	"context"
	"log"
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
}

type RuntimeModule interface {
	Module
	// Provide Initialization
	Initialize(chan event.Event, *lua.LState)
}

type Runtime struct {
	sync.Mutex
	logger *zap.Logger
	vm     *lua.LState
	wg     *sync.WaitGroup

	script      string
	preloads    map[string]Module
	entry       *lua.LTable
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
	r.scripts = module.NewScriptModule(r.logger)
	for name, lib := range map[string]lua.LGFunction{
		lua.BaseLibName:      lua.OpenBase,
		lua.TabLibName:       lua.OpenTable,
		lua.OsLibName:        auxlib.OpenOs,
		lua.IoLibName:        lua.OpenIo,
		lua.StringLibName:    lua.OpenString,
		lua.MathLibName:      lua.OpenMath,
		lua.CoroutineLibName: lua.OpenCoroutine,
		auxlib.Bit32LibName:  auxlib.OpenBit32,
		auxlib.JsonLibName:   auxlib.OpenJson,
		auxlib.Bit64LibName:  auxlib.OpenBit64,
		auxlib.Base64LibName: auxlib.OpenBase64,
		auxlib.UUIDLibName:   auxlib.OpenUUID,
		auxlib.MD5LibName:    auxlib.OpenMD5,
		auxlib.Ase128LibName: auxlib.OpenAes128,
		auxlib.Ase256LibName: auxlib.OpenAes256,
		lua.LoadLibName:      r.scripts.OpenPackage(),
	} {
		r.vm.Push(r.vm.NewFunction(lib))
		r.vm.Push(lua.LString(name))
		r.vm.Call(1, 0)
		log.Println("load lib", name)
	}
	for name, module := range r.preloads {
		r.vm.PreloadModule(name, module.Open())
		log.Println("preload module", name)
	}
	r.wg.Add(1)
	go r.process()
	r.vm.SetContext(r.ctx)
	init := lua.LString(r.script)
	req := r.vm.GetGlobal("require").(*lua.LFunction)
	if err := r.vm.GPCall(req.GFunction, init); err != nil {
		r.logger.Error("script", zap.Error(err))
		r.ctxCancelFn()
	} else if entry, ok := r.vm.Get(-1).(*lua.LTable); ok {
		r.entry = entry
	} else {
		r.entry = r.vm.CreateTable(0, 0)
		r.vm.SetGlobal(r.script, r.entry)
	}
	r.logger.Info("runtime", zap.String("entry", r.script))
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
		case e = <-r.eventQueue:
			eventQueue = append(eventQueue, e)
		}
	}
	r.wg.Done()
}
