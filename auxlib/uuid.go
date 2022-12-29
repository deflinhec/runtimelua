package auxlib

import (
	"github.com/google/uuid"
	lua "github.com/yuin/gopher-lua"
)

var (
	UUIDLibName = "uuid"
)

var uuidFuncs = map[string]lua.LGFunction{
	"gen": uuidGen,
}

func OpenUUID(l *lua.LState) int {
	mod := l.RegisterModule(UUIDLibName, uuidFuncs)
	l.Push(mod)
	return 1
}

func uuidGen(l *lua.LState) int {
	l.Push(lua.LString(uuid.New().String()))
	return 1
}
