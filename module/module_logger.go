package module

import (
	"log"
	"strconv"

	"github.com/deflinhec/runtimelua/luaconv"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

type loggerModule struct {
	Module
}

func LoggerModule(runtime Runtime) *loggerModule {
	return &loggerModule{
		Module: Module{
			name: "logger",
		},
	}
}

func (m *loggerModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"log":   m.log,
			"debug": m.logDebug,
			"info":  m.logInfo,
			"warn":  m.logWarn,
			"error": m.logError,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

func (m *loggerModule) log(l *lua.LState) int {
	message := l.CheckString(1)
	if message == "" {
		return 0
	}
	log.Println("[LUA]", message)
	return 0
}

func collapse_to_log(table *lua.LTable) []zap.Field {
	fields := make([]zap.Field, 0)
	table.ForEach(func(l1, l2 lua.LValue) {
		field := luaconv.LuaValue(l1)
		value := luaconv.LuaValue(l2)
		switch f := field.(type) {
		case int64:
			field = strconv.FormatInt(f, 10)
		case []byte:
			field = string(f)
		case string:
			field = f
		default:
			log.Printf("unsupport key type %T", f)
		}
		switch v := value.(type) {
		case int64:
			fields = append(fields, zap.Int64(field.(string), v))
		case []byte:
			fields = append(fields, zap.String(field.(string), string(v)))
		case string:
			fields = append(fields, zap.String(field.(string), v))
		case float64:
			fields = append(fields, zap.Float64(field.(string), v))
		case map[string]interface{}:
			fields = append(fields, zap.Any(field.(string), v))
		case []interface{}:
			fields = append(fields, zap.Any(field.(string), v))
		case bool:
			fields = append(fields, zap.Bool(field.(string), v))
		default:
			log.Printf("unsupport value type %T", v)
		}
	})
	return fields
}

func (m *loggerModule) logInfo(l *lua.LState) int {
	msg := l.CheckString(1)
	value := l.Get(2)
	switch value := value.(type) {
	case *lua.LTable:
		m.logger.Info(msg, collapse_to_log(value)...)
	case lua.LString:
		m.logger.Info(msg, zap.String("content", value.String()))
	default:
		m.logger.Info(msg)
	}
	return 0
}

func (m *loggerModule) logWarn(l *lua.LState) int {
	msg := l.CheckString(1)
	value := l.Get(2)
	switch value := value.(type) {
	case *lua.LTable:
		m.logger.Warn(msg, collapse_to_log(value)...)
	case lua.LString:
		m.logger.Warn(msg, zap.String("content", value.String()))
	default:
		m.logger.Warn(msg)
	}
	return 0
}

func (m *loggerModule) logError(l *lua.LState) int {
	msg := l.CheckString(1)
	value := l.Get(2)
	switch value := value.(type) {
	case *lua.LTable:
		m.logger.Error(msg, collapse_to_log(value)...)
	case lua.LString:
		m.logger.Error(msg, zap.String("content", value.String()))
	default:
		m.logger.Error(msg)
	}
	return 0
}

func (m *loggerModule) logDebug(l *lua.LState) int {
	msg := l.CheckString(1)
	value := l.Get(2)
	switch value := value.(type) {
	case *lua.LTable:
		m.logger.Debug(msg, collapse_to_log(value)...)
	case lua.LString:
		m.logger.Debug(msg, zap.String("content", value.String()))
	default:
		m.logger.Debug(msg)
	}
	return 0
}
