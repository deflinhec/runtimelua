package runtimelua_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/deflinhec/runtimelua"

	"github.com/google/uuid"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

func newRuntimeWithModules(t *testing.T, modules map[string]string,
	options ...runtimelua.Option) *runtimelua.Runtime {
	dir, err := os.MkdirTemp("",
		fmt.Sprintf("runtime_lua_test_%v", uuid.New().String()),
	)
	if err != nil {
		t.Fatalf("Failed initializing runtime modules tempdir: %s", err.Error())
	}
	defer os.RemoveAll(dir)

	for moduleName, moduleData := range modules {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%v.lua", moduleName)), []byte(moduleData), 0644); err != nil {
			t.Fatalf("Failed initializing runtime modules tempfile: %s", err.Error())
		}
	}
	lua.LuaLDir = dir
	logger, _ := zap.NewDevelopment()
	r := runtimelua.NewRuntime(
		runtimelua.NewScriptModule(logger),
		options...,
	)
	r.Startup()
	return r
}

type TestingModule struct {
	sync.Mutex
	err    error
	cancel context.CancelFunc
}

func (t *TestingModule) Name() string {
	return "test"
}

func (m *TestingModule) fatal(l *lua.LState) int {
	msg := l.CheckString(1)
	err := errors.New(msg)
	m.Lock()
	m.err = multierr.Append(m.err, err)
	m.Unlock()
	go m.cancel()
	return 0
}

func (m *TestingModule) Initialize(*zap.Logger) {

}

func (m *TestingModule) done(l *lua.LState) int {
	go m.cancel()
	return 0
}

func (m *TestingModule) validate(t *testing.T) {
	if m.err != nil {
		t.Fatal(m.err)
	}
}

func (m *TestingModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"fatal": m.fatal,
			"done":  m.done,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}
