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

package runtimelua_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/deflinhec/runtimelua"

	"github.com/google/uuid"
	lua "github.com/yuin/gopher-lua"
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
	context.CancelFunc
	*testing.T
}

func (t *TestingModule) Name() string {
	return "test"
}

func (m *TestingModule) fatal(l *lua.LState) int {
	m.T.Fatal(l.OptString(1, ""))
	return 0
}

func (m *TestingModule) fail(l *lua.LState) int {
	m.T.Fail()
	return 0
}

func (m *TestingModule) failNow(l *lua.LState) int {
	m.T.FailNow()
	return 0
}

func (m *TestingModule) log(l *lua.LState) int {
	m.T.Log(l.OptString(1, ""))
	return 0
}

func (m *TestingModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"fatal":   m.fatal,
			"fail":    m.fail,
			"failNow": m.failNow,
			"log":     m.log,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}
