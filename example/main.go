package main

import (
	"context"
	"sync"

	"github.com/deflinhec/runtimelua"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

const (
	Parallel = 2
)

func init() {
	lua.LuaLDir = "script"
}

func main() {
	wg := &sync.WaitGroup{}
	logger, _ := zap.NewDevelopment()
	ctx, ctxCancelFn := context.WithCancel(context.Background())
	defer ctxCancelFn()
	for i := 0; i < Parallel; i++ {
		runtimelua.NewRuntime(
			runtimelua.NewScriptModule(logger),
			runtimelua.WithLibJson(),
			runtimelua.WithLibBit32(),
			runtimelua.WithLibBase64(),
			runtimelua.WithWaitGroup(wg),
			runtimelua.WithContext(ctx),
			runtimelua.WithLogger(logger),
			runtimelua.WithModuleEvent(logger),
			runtimelua.WithModuleLogger(logger),
			runtimelua.WithGlobal("GLOBAL_VARS", []string{"var1", "var2"}),
		).Startup()
	}
	wg.Wait()
}
