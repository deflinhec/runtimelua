package auxlib

import (
	"encoding/base64"

	lua "github.com/yuin/gopher-lua"
)

var (
	Base64LibName = "base64"
)

var base64Funcs = map[string]lua.LGFunction{
	"decode": base64Decode,
	"encode": base64Encode,
}

func OpenBase64(l *lua.LState) int {
	mod := l.RegisterModule(Base64LibName, base64Funcs)
	l.Push(mod)
	return 1
}

func base64Encode(l *lua.LState) int {
	ctx := []byte(l.CheckString(1))
	l.Push(lua.LString(base64.StdEncoding.EncodeToString(ctx)))
	return 1
}

func base64Decode(l *lua.LState) int {
	ctx := l.CheckString(1)
	b, err := base64.StdEncoding.DecodeString(ctx)
	if err != nil {
		l.Push(lua.LNil)
		l.Push(lua.LString(err.Error()))
		return 2
	}
	l.Push(lua.LString(b))
	return 1
}
