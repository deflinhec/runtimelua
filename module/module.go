package module

import (
	"encoding/gob"

	"github.com/deflinhec/runtimelua/event"

	"go.uber.org/zap"
)

type Runtime interface {
	EventQueue() chan event.Event
}

type Module struct {
	name   string
	logger *zap.Logger
}

func (m *Module) Initialize(logger *zap.Logger) {
	m.logger = logger.With(
		zap.String("module", m.name),
	)
}

type RuntimeModule struct {
	Module
	runtime Runtime
}

func (m *Module) Name() string {
	return m.name
}

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}
