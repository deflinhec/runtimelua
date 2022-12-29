package auxlib

import (
	"encoding/json"

	"github.com/deflinhec/runtimelua/luaconv"

	lua "github.com/yuin/gopher-lua"
)

var (
	JsonLibName = "json"
)

var jsonFuncs = map[string]lua.LGFunction{
	"decode": jsonDecode,
	"encode": jsonEncode,
}

func OpenJson(l *lua.LState) int {
	mod := l.RegisterModule(JsonLibName, jsonFuncs)
	l.Push(mod)
	return 1
}

func jsonEncode(l *lua.LState) int {
	value := l.Get(1)
	if value == nil {
		l.ArgError(1, "expects a non-nil value to encode")
		return 0
	}

	jsonData := luaconv.LuaValue(value)
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		l.RaiseError("error encoding to JSON: %v", err.Error())
		return 0
	}

	l.Push(lua.LString(string(jsonBytes)))
	return 1
}

func jsonDecode(l *lua.LState) int {
	jsonString := l.CheckString(1)
	if jsonString == "" {
		l.ArgError(1, "expects JSON string")
		return 0
	}

	var jsonData interface{}
	if err := json.Unmarshal([]byte(jsonString), &jsonData); err != nil {
		l.RaiseError("not a valid JSON string: %v", err.Error())
		return 0
	}

	l.Push(luaconv.Value(l, jsonData))
	return 1
}
