package module

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	logger  *zap.Logger
	modules map[string]*fileCache
}

func NewScriptModule(logger *zap.Logger) *ScriptModule {
	m := &ScriptModule{
		logger: logger.With(
			zap.String("module", "scripts"),
		),
		modules: make(map[string]*fileCache),
	}
	return m.load()
}

func (c *ScriptModule) Reload(l *lua.LState) error {
	var result error
	for _, module := range c.load().modules {
		if err := module.hotfix(l); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

func (c *ScriptModule) Hotfix(l *lua.LState, name string) error {
	if module, ok := c.modules[name]; ok {
		return module.hotfix(l)
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

func (c *ScriptModule) load() *ScriptModule {
	log.Println("[LuaPath]", lua.LuaLDir)
	for _, path := range scan(lua.LuaLDir) {
		if strings.ToLower(filepath.Ext(path)) != ".lua" {
			continue
		}
		if _, ok := c.modules[path]; !ok {
			module := &fileCache{
				Path: path,
			}
			if err := module.load(); err != nil {
				continue
			}
			c.modules[module.Name] = module
			log.Println("[LuaScript]", module.Name)
		}
	}
	return c
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

func (c *ScriptModule) OpenPackage() lua.LGFunction {
	return func(L *lua.LState) int {
		loLoaderCache := func(L *lua.LState) int {
			name := L.CheckString(1)
			// Make paths Lua friendly.
			name = strings.Replace(name, string(os.PathSeparator), ".", -1)
			if module, ok := c.modules[name]; ok {
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
		return 1
	}
}
