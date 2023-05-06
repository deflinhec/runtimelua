package runtimelua

import (
	"encoding/gob"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

type Module interface {
	Name() string
	Open() lua.LGFunction
	Initialize(*zap.Logger)
}

type localModule struct {
	name   string
	logger *zap.Logger
}

func (m *localModule) Name() string {
	return m.name
}

func (m *localModule) Initialize(logger *zap.Logger) {
	m.logger = logger.With(
		zap.String("module", m.name),
	)
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
