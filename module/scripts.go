package module

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deflinhec/runtimelua/event"
	"github.com/hashicorp/go-multierror"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

type fileCache struct {
	loaded  bool
	Name    string
	Path    string
	Content []byte
}

func (m *fileCache) MD5Sum() string {
	b := md5.Sum(m.Content)
	return hex.EncodeToString(b[:])
}

func (m *fileCache) load() error {
	if len(m.Content) != 0 {
		return nil
	}
	relPath, _ := filepath.Rel(lua.LuaLDir, m.Path)
	name := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	// Make paths Lua friendly.
	m.Name = strings.Replace(name, string(os.PathSeparator), ".", -1)
	return m.reload()
}

func (m *fileCache) reload() error {
	var err error
	if m.Content, err = ioutil.ReadFile(m.Path); err != nil {
		return err
	}
	return nil
}

func (m *fileCache) hotfix(l *lua.LState) error {
	content, err := ioutil.ReadFile(m.Path)
	if err != nil {
		return err
	}
	lfunc, err := l.Load(bytes.NewReader(m.Content), m.Path)
	if err != nil {
		return err
	}
	l.Push(lfunc)
	if err := l.PCall(0, lua.MultRet, nil); err != nil {
		return err
	}
	m.Content = content
	return nil
}

type ScriptModule struct {
	RuntimeModule
	modules map[string]*fileCache
	vm      *lua.LState
}

func NewScriptModule(runtime Runtime) *ScriptModule {
	return &ScriptModule{
		RuntimeModule: RuntimeModule{
			Module: Module{
				name: "script",
			},
			runtime: runtime,
		},
		modules: make(map[string]*fileCache),
	}
}

func (m *ScriptModule) Initialize(logger *zap.Logger) {
	m.logger = logger.With(zap.String("module", m.name))
	m.logger.Debug("config", zap.String("LuaPath", lua.LuaLDir))
	m.logger.Debug("load", zap.Int("count", len(m.load())))
}

func (m *ScriptModule) load() []string {
	scanned := make([]string, 0)
	for _, path := range scan(lua.LuaLDir) {
		if strings.ToLower(filepath.Ext(path)) != ".lua" {
			continue
		}

		module := &fileCache{
			Path: path,
		}
		if err := module.load(); err != nil {
			continue
		}
		if _, ok := m.modules[module.Name]; !ok {
			m.modules[module.Name] = module
			scanned = append(scanned, module.Name)
			m.logger.Debug("find", zap.String("file", module.Name))
		} else if m.modules[module.Name].MD5Sum() != module.MD5Sum() {
			m.modules[module.Name] = module
			scanned = append(scanned, module.Name)
			m.logger.Debug("find", zap.String("file", module.Name))
		}
	}
	return scanned
}

func (m *ScriptModule) MD5Sum() map[string]string {
	md5sum := make(map[string]string)
	for name, cache := range m.modules {
		md5sum[name] = cache.MD5Sum()
	}
	return md5sum
}

func (m *ScriptModule) Reload() error {
	scanned := m.load()
	modules := make([]*fileCache, 0, len(m.modules))
	for _, module := range m.modules {
		modules = append(modules, module)
	}
	result := make(chan error)
	e := &hotFixEvent{
		Modules: modules,
		logger: m.logger.With(
			zap.String("operate", "reload"),
			zap.Int("sum_files", len(m.modules)),
			zap.Int("new_files", len(scanned)),
		),
		ReturnCh: result,
		VM:       m.vm,
	}
	m.runtime.EventQueue() <- e
	return <-result
}

func (m *ScriptModule) Hotfix(name string) error {
	if module, ok := m.modules[name]; ok {
		result := make(chan error)
		e := &hotFixEvent{
			Modules: []*fileCache{module},
			logger: m.logger.With(
				zap.String("operate", "hotfix"),
				zap.String("script", name),
			),
			ReturnCh: result,
			VM:       m.vm,
		}
		m.runtime.EventQueue() <- e
		return <-result
	}
	return errors.New("file not found")
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

func (m *ScriptModule) OpenPackage() lua.LGFunction {
	return func(L *lua.LState) int {
		loLoaderCache := func(L *lua.LState) int {
			name := L.CheckString(1)
			// Make paths Lua friendly.
			name = strings.Replace(name, string(os.PathSeparator), ".", -1)
			if module, ok := m.modules[name]; ok {
				lfunc, err := L.Load(bytes.NewReader(module.Content), module.Path)
				if err != nil {
					L.RaiseError(err.Error())
				}
				L.Push(lfunc)
			} else {
				L.Push(lua.LString(fmt.Sprintf("no cached module '%s'", name)))
			}
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
		m.vm = L
		return 1
	}
}

type hotFixEvent struct {
	event.StateEvent
	logger   *zap.Logger
	VM       *lua.LState
	Modules  []*fileCache
	ReturnCh chan error
}

func (e *hotFixEvent) Update(time.Duration) error {
	switch event.State(e.Load()) {
	case event.INITIALIZE:
		var errs error
		defer e.Store(uint32(event.COMPLETE))
		for _, module := range e.Modules {
			if err := module.hotfix(e.VM); err != nil {
				e.logger.Warn("cannot perform hotfix",
					zap.String("module", module.Name),
					zap.String("path", module.Path),
				)
				errs = multierror.Append(errs, err)
				continue
			}
			e.logger.Info("success",
				zap.String("module", module.Name),
			)
		}
		e.ReturnCh <- errs
	}
	return nil
}
