package runtimelua

import (
	"encoding/gob"

	lua "github.com/yuin/gopher-lua"
)

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

type Module interface {
	Name() string
	Open() lua.LGFunction
}

type localModule struct {
	name string
}

func (m *localModule) Name() string {
	return m.name
}

type RuntimeModule interface {
	Module
	// Provide Initialization
	InitializeRuntime(*Runtime)
}

type localRuntimeModule struct {
	localModule
	runtime *Runtime
}
