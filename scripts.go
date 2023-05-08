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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dietsche/rfsnotify"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"gopkg.in/fsnotify.v1"
)

const (
	luaDisableHotfixMarker = "---@disable-hotfix"
)

type localFileCache struct {
	sync.RWMutex
	HotfixDisabled *atomic.Bool
	Name           string
	Path           string
	Content        []byte
}

func (m *localFileCache) hotfix(l *lua.LState) error {
	content, err := ioutil.ReadFile(m.Path)
	if err != nil {
		return err
	}
	lfunc, err := l.Load(bytes.NewReader(content), m.Name)
	if err != nil {
		return err
	}
	l.Push(lfunc)
	if err := l.PCall(0, lua.MultRet, nil); err != nil {
		return err
	}
	m.Lock()
	defer m.Unlock()
	m.Content = content
	return nil
}

func (m *localFileCache) patch(l *lua.LState) error {
	m.RLock()
	defer m.RUnlock()
	// Update preload function
	preload := l.GetField(l.GetField(l.Get(lua.EnvironIndex), "package"), "preload")
	f, err := l.Load(bytes.NewReader(m.Content), m.Name)
	if err != nil {
		return err
	}
	l.SetField(preload, m.Name, f)

	// Update module which alreade loaded
	loaded := l.GetField(l.Get(lua.RegistryIndex), "_LOADED")
	lv := l.GetField(loaded, m.Name)
	if !lua.LVAsBool(lv) {
		// Not yet evaluated
		return nil
	}

	l.Push(f)
	return l.PCall(0, -1, nil)
}

type ModuleNames []string

func (a ModuleNames) Len() int {
	return len(a)
}

func (a ModuleNames) Less(i, j int) bool {
	init := strings.HasSuffix(a[i], "init")
	if init && strings.HasSuffix(a[j], "init") {
		return strings.Count(a[i], ".") < strings.Count(a[j], ".")
	} else if init {
		return true
	}
	return strings.Count(a[i], ".") < strings.Count(a[j], ".")
}

func (a ModuleNames) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

type ScriptModule interface {
	OpenPackage() lua.LGFunction

	List() []string
}

type localScriptModule struct {
	sync.RWMutex

	once sync.Once

	runtimes chan *Runtime

	logger  *zap.Logger
	names   ModuleNames
	modules map[string]*localFileCache
}

func NewScriptModule(logger *zap.Logger) *localScriptModule {
	sm := &localScriptModule{
		logger: logger,

		once: sync.Once{},

		names:    make(ModuleNames, 0),
		runtimes: make(chan *Runtime, 1024),
		modules:  make(map[string]*localFileCache),
	}
	return sm
}

func (sm *localScriptModule) add(m *localFileCache) {
	sm.Lock()
	defer sm.Unlock()

	if old, ok := sm.modules[m.Name]; !ok {
		sm.names = append(sm.names, m.Name)
		// Process init scripts with ascending depth order before all other scripts.
		sort.Sort(sm.names)
	} else {
		// Preserve hotfix disabled state.
		m.HotfixDisabled.Swap(old.HotfixDisabled.Load())
	}

	sm.modules[m.Name] = m
}

func (sm *localScriptModule) List() []string {
	sm.RLock()
	defer sm.RUnlock()
	clone := make([]string, len(sm.names))
	copy(clone, sm.names)
	return clone
}

func (sm *localScriptModule) get(name string) (*localFileCache, bool) {
	sm.RLock()
	defer sm.RUnlock()
	if m, ok := sm.modules[name]; ok {
		return m, ok
	}
	return nil, false
}

func scan(path string) []string {
	paths := make([]string, 0, 5)
	fn := func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore directories.
		if !f.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}
	if err := filepath.Walk(path, fn); err != nil {
		return []string{}
	}
	return paths
}

const emptyLString lua.LString = lua.LString("")

func loGetPath(env string, defpath string) string {
	path := os.Getenv(env)
	if len(path) == 0 {
		path = defpath
	}
	path = strings.Replace(path, ";;", ";"+defpath+";", -1)
	if os.PathSeparator != '/' {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			panic(err)
		}
		path = strings.Replace(path, "!", dir, -1)
	}
	return path
}

var loFuncs = map[string]lua.LGFunction{
	"loadlib": loLoadLib,
	"seeall":  loSeeAll,
}

func loLoaderPreload(L *lua.LState) int {
	name := L.CheckString(1)
	preload := L.GetField(L.GetField(L.Get(lua.EnvironIndex), "package"), "preload")
	if _, ok := preload.(*lua.LTable); !ok {
		L.RaiseError("package.preload must be a table")
	}
	lv := L.GetField(preload, name)
	if lv == lua.LNil {
		L.Push(lua.LString(fmt.Sprintf("no field package.preload['%s']", name)))
		return 1
	}
	L.Push(lv)
	return 1
}

func loLoadLib(L *lua.LState) int {
	L.RaiseError("loadlib is not supported")
	return 0
}

func loSeeAll(L *lua.LState) int {
	mod := L.CheckTable(1)
	mt := L.GetMetatable(mod)
	if mt == lua.LNil {
		mt = L.CreateTable(0, 1)
		L.SetMetatable(mod, mt)
	}
	L.SetField(mt, "__index", L.Get(lua.GlobalsIndex))
	return 0
}

func (sm *localScriptModule) OpenPackage() lua.LGFunction {
	sm.once.Do(func() {
		// Load file from file system.
		for _, path := range scan(lua.LuaLDir) {
			if strings.ToLower(filepath.Ext(path)) != ".lua" {
				continue
			}

			var name, relPath string
			relPath, _ = filepath.Rel(lua.LuaLDir, path)
			name = strings.TrimSuffix(relPath, filepath.Ext(relPath))
			// Make paths Lua friendly.
			name = strings.ReplaceAll(name, string(os.PathSeparator), ".")

			content, err := os.ReadFile(path)
			if err != nil {
				sm.logger.Warn("An error occurred while reading lua module", zap.Error(err))
				break
			}
			static := strings.HasPrefix(string(content), luaDisableHotfixMarker)
			m := &localFileCache{
				HotfixDisabled: atomic.NewBool(static),
				Name:           name,
				Path:           path,
				Content:        content,
			}
			sm.add(m)
		}
		watcher, err := rfsnotify.NewWatcher()
		if err != nil {
			sm.logger.Fatal("Failed to create runtime directory watcher", zap.Error(err))
		}
		// Watch for changes in the runtime directory.
		go func() {
			ctx := context.Background()
			for {
				select {
				case <-ctx.Done():
					// Context cancelled
					return
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if strings.ToLower(filepath.Ext(event.Name)) != ".lua" {
						break
					}
					var name, relPath string
					relPath, _ = filepath.Rel(lua.LuaLDir, event.Name)
					name = strings.TrimSuffix(relPath, filepath.Ext(relPath))
					// Make paths Lua friendly.
					name = strings.Replace(name, string(os.PathSeparator), ".", -1)
					switch event.Op {
					case fsnotify.Write:
						// Skip static files.
						if m, ok := sm.get(name); ok && m.HotfixDisabled.Load() {
							break
						}
						fallthrough
					case fsnotify.Create:
						content, err := os.ReadFile(event.Name)
						if err != nil {
							sm.logger.Warn("An error occurred while reading lua module", zap.Error(err))
							break
						}
						static := strings.HasPrefix(string(content), luaDisableHotfixMarker)
						m := &localFileCache{
							HotfixDisabled: atomic.NewBool(static),
							Name:           name,
							Path:           event.Name,
							Content:        content,
						}
						completes := 0
						// Loop through the pool and hotfix each runtime.
					HotfixProcess:
						for {
							select {
							case <-ctx.Done():
								// Context cancelled
								return
							case r := <-sm.runtimes:
								sm.runtimes <- r
								var e Event
								result := make(chan error, 1)
								// Discard hotfix if the first attempt fails.
								if completes == 0 {
									e = &localHotfixEvent{
										module:   m,
										logger:   sm.logger,
										resultCh: result,
									}
								} else {
									e = &localPatchEvent{
										module:   m,
										logger:   sm.logger,
										resultCh: result,
									}
								}
								// Send and wait for the result.
								r.EventQueue <- e
								if err := <-result; err != nil {
									sm.logger.Warn("An error occurred while patching lua module",
										zap.String("module", name), zap.Error(err))
									break HotfixProcess
								}
								completes++
								if completes < len(sm.runtimes) {
									continue
								}
								sm.add(m)
								sm.logger.Info("Lua runtime patched",
									zap.String("module", name),
									zap.Int("runtimes", completes))
								break HotfixProcess
							default:
								time.Sleep(100 * time.Millisecond)
							}
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					sm.logger.Error("An error occurred while watching directory", zap.Error(err))
				}
			}
		}()
		if err = watcher.AddRecursive(lua.LuaLDir); err != nil {
			sm.logger.Fatal("An error occurred while watching directory", zap.Error(err))
		}
		absDir, _ := filepath.Abs(lua.LuaLDir)
		sm.logger.Info("Watching runtime directory", zap.String("path", absDir))
	})
	return func(L *lua.LState) int {
		loLoaderCache := func(L *lua.LState) int {
			name := L.CheckString(1)
			module, ok := sm.get(name)
			if !ok {
				L.Push(lua.LString(fmt.Sprintf("no cached module '%s'", name)))
				return 1
			}
			fn, err := L.Load(bytes.NewReader(module.Content), module.Path)
			if err != nil {
				L.RaiseError(err.Error())
			}
			L.Push(fn)
			return 1
		}

		packagemod := L.RegisterModule(lua.LoadLibName, loFuncs)

		L.SetField(packagemod, "preload", L.NewTable())

		loaders := L.CreateTable(2, 0)
		L.RawSetInt(loaders, 1, L.NewFunction(loLoaderPreload))
		L.RawSetInt(loaders, 2, L.NewFunction(loLoaderCache))
		L.SetField(packagemod, "loaders", loaders)
		L.SetField(L.Get(lua.RegistryIndex), "_LOADERS", loaders)

		loaded := L.NewTable()
		L.SetField(packagemod, "loaded", loaded)
		L.SetField(L.Get(lua.RegistryIndex), "_LOADED", loaded)

		L.SetField(packagemod, "path", lua.LString(loGetPath(lua.LuaPath, lua.LuaPathDefault)))
		L.SetField(packagemod, "cpath", emptyLString)

		L.Push(packagemod)
		return 1
	}
}

type localHotfixEvent struct {
	StateEvent
	logger   *zap.Logger
	module   *localFileCache
	resultCh chan error
}

func (e *localHotfixEvent) Update(d time.Duration, l *lua.LState) error {
	switch EventState(e.Load()) {
	case EVENT_STATE_INITIALIZE:
		var err error
		defer e.Store(uint32(EVENT_STATE_COMPLETE))
		if err = e.module.hotfix(l); err != nil {
			e.logger.Warn("cannot perform hotfix",
				zap.String("module", e.module.Name),
				zap.String("path", e.module.Path),
			)
		} else {
			e.logger.Info("success",
				zap.String("module", e.module.Name),
			)
		}
		e.resultCh <- err
	}
	return nil
}

type localPatchEvent struct {
	StateEvent
	logger   *zap.Logger
	module   *localFileCache
	resultCh chan error
}

func (e *localPatchEvent) Update(d time.Duration, l *lua.LState) error {
	switch EventState(e.Load()) {
	case EVENT_STATE_INITIALIZE:
		var err error
		defer e.Store(uint32(EVENT_STATE_COMPLETE))
		if err = e.module.patch(l); err != nil {
			e.logger.Warn("cannot perform hotfix",
				zap.String("module", e.module.Name),
				zap.String("path", e.module.Path),
			)
		} else {
			e.logger.Info("success",
				zap.String("module", e.module.Name),
			)
		}
		e.resultCh <- err
	}
	return nil
}
