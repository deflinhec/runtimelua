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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/deflinhec/runtimelua/auxlib"
	"github.com/deflinhec/runtimelua/luaconv"
	"github.com/go-redis/redis/v8"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

type Option interface {
	apply(s *Runtime)
}

type funcOption struct {
	f func(*Runtime)
}

func (fdo *funcOption) apply(do *Runtime) {
	fdo.f(do)
}

func newOption(f func(*Runtime)) *funcOption {
	return &funcOption{
		f: f,
	}
}

func WithLogger(logger *zap.Logger) Option {
	return newOption(func(r *Runtime) {
		r.logger = logger
	})
}

func WithContext(ctx context.Context) Option {
	return newOption(func(r *Runtime) {
		r.ctx, r.ctxCancelFn = context.WithCancel(ctx)
	})
}

func WithWaitGroup(wg *sync.WaitGroup) Option {
	return newOption(func(r *Runtime) {
		r.wg = wg
	})
}

func WithModuleRedis(logger *zap.Logger, opts *redis.Options) Option {
	return newOption(func(r *Runtime) {
		mod := &localRedisModule{
			localRuntimeModule: localRuntimeModule{
				localModule: localModule{
					name: "redis",
				},
				runtime: r,
			},
			logger: logger,
			pubsub: make(map[string]*localRedisSubscriber),
			pool:   connect_redis(opts),
		}
		r.preloads[mod.Name()] = mod
	})
}

func WithModuleEvent(logger *zap.Logger) Option {
	return newOption(func(r *Runtime) {
		mod := &localEventModule{
			localRuntimeModule: localRuntimeModule{
				localModule: localModule{
					name: "event",
				},
				runtime: r,
			},
			logger: logger,
		}
		r.preloads[mod.Name()] = mod
	})
}

func WithModuleLogger(logger *zap.Logger) Option {
	return newOption(func(r *Runtime) {
		mod := &localLoggerModule{
			localModule: localModule{
				name: "logger",
			},
			logger: logger,
		}
		r.preloads[mod.Name()] = mod
	})
}

func WithModuleHttp(logger *zap.Logger) Option {
	return newOption(func(r *Runtime) {
		mod := &localHTTPModule{
			localRuntimeModule: localRuntimeModule{
				localModule: localModule{
					name: "http",
				},
				runtime: r,
			},
			logger: logger,
		}
		r.preloads[mod.Name()] = mod
	})
}

func WithModule(mod Module) Option {
	return newOption(func(r *Runtime) {
		r.preloads[mod.Name()] = mod
	})
}

func WithRuntimeModule(mod RuntimeModule) Option {
	return newOption(func(r *Runtime) {
		r.preloads[mod.Name()] = mod
		mod.InitializeRuntime(r)
	})
}

func WithLibJson() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.JsonLibName
		lib := auxlib.OpenJson
		r.auxlibs[name] = lib
	})
}

func WithLibAes256() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.Ase256LibName
		lib := auxlib.OpenAes256
		r.auxlibs[name] = lib
	})
}

func WithLibAes128() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.Ase128LibName
		lib := auxlib.OpenAes128
		r.auxlibs[name] = lib
	})
}

func WithLibMD5() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.MD5LibName
		lib := auxlib.OpenMD5
		r.auxlibs[name] = lib
	})
}

func WithLibUUID() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.UUIDLibName
		lib := auxlib.OpenUUID
		r.auxlibs[name] = lib
	})
}

func WithLibBase64() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.Base64LibName
		lib := auxlib.OpenBase64
		r.auxlibs[name] = lib
	})
}

func WithLibBit32() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.Bit32LibName
		lib := auxlib.OpenBit32
		r.auxlibs[name] = lib
	})
}

func WithLibBit64() Option {
	return newOption(func(r *Runtime) {
		name := auxlib.Bit64LibName
		lib := auxlib.OpenBit64
		r.auxlibs[name] = lib
	})
}

// The script path for the runtime entrypoint. Default is main.
func WithScriptEntry(path string) Option {
	return newOption(func(r *Runtime) {
		relPath, _ := filepath.Rel(lua.LuaLDir, path)
		name := strings.TrimSuffix(relPath, filepath.Ext(relPath))
		// Make paths Lua friendly.
		r.script = strings.Replace(name, string(os.PathSeparator), ".", -1)
	})
}

// Global variable which can be access within Lua runtime.
func WithGlobal(key string, value interface{}) Option {
	return newOption(func(r *Runtime) {
		r.vm.SetGlobal(key, luaconv.Value(r.vm, value))
	})
}

// Evaluation handler which handle the return value of script entry.
func WithEvaluation(evaluation func(interface{})) Option {
	return newOption(func(r *Runtime) {
		r.evaluate = evaluation
	})
}
