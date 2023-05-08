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
		luaLogger := logger.With(zap.Int("vm", i))
		runtimelua.NewRuntime(
			runtimelua.NewScriptModule(luaLogger),
			runtimelua.WithLibJson(),
			runtimelua.WithLibBit32(),
			runtimelua.WithLibBase64(),
			runtimelua.WithWaitGroup(wg),
			runtimelua.WithContext(ctx),
			runtimelua.WithLogger(luaLogger),
			runtimelua.WithModuleEvent(luaLogger),
			runtimelua.WithModuleLogger(luaLogger),
			runtimelua.WithGlobal("GLOBAL_VARS", []string{"var1", "var2"}),
		).Startup()
	}
	wg.Wait()
}
