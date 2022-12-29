package luaconv

import (
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func LuaValue(lv lua.LValue) interface{} {
	// Taken from: https://github.com/yuin/gluamapper/blob/master/gluamapper.go#L79
	switch v := lv.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LString:
		return string(v)
	case lua.LNumber:
		vf := float64(v)
		vi := int64(v)
		if vf == float64(vi) {
			// If it's a whole number use an actual integer type.
			return vi
		}
		return vf
	case *lua.LTable:
		maxn := v.MaxN()
		if maxn == 0 {
			// Table.
			ret := make(map[string]interface{})
			v.ForEach(func(key, value lua.LValue) {
				keyStr := fmt.Sprint(LuaValue(key))
				ret[keyStr] = LuaValue(value)
			})
			return ret
		}
		// Array.
		ret := make([]interface{}, 0, maxn)
		for i := 1; i <= maxn; i++ {
			ret = append(ret, LuaValue(v.RawGetInt(i)))
		}
		return ret
	case *lua.LFunction:
		return v.String()
	default:
		return v
	}
}

func MapString(l *lua.LState, data map[string]string) *lua.LTable {
	lt := l.CreateTable(0, len(data))

	for k, v := range data {
		lt.RawSetString(k, Value(l, v))
	}

	return lt
}

func Table(lv *lua.LTable) map[string]interface{} {
	returnData, _ := LuaValue(lv).(map[string]interface{})
	return returnData
}

func Map(l *lua.LState, data map[string]interface{}) *lua.LTable {
	lt := l.CreateTable(0, len(data))

	for k, v := range data {
		lt.RawSetString(k, Value(l, v))
	}

	return lt
}

func MapInt64(l *lua.LState, data map[string]int64) *lua.LTable {
	lt := l.CreateTable(0, len(data))

	for k, v := range data {
		lt.RawSetString(k, Value(l, v))
	}

	return lt
}

func Value(l *lua.LState, val interface{}) lua.LValue {
	if val == nil {
		return lua.LNil
	}

	// Types looked up from:
	// https://golang.org/pkg/encoding/json/#Unmarshal
	// https://developers.google.com/protocol-buffers/docs/proto3#scalar
	// More types added based on observations.
	switch v := val.(type) {
	case bool:
		return lua.LBool(v)
	case string:
		return lua.LString(v)
	case []byte:
		return lua.LString(v)
	case float32:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case int:
		return lua.LNumber(v)
	case int8:
		return lua.LNumber(v)
	case int32:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case uint:
		return lua.LNumber(v)
	case uint8:
		return lua.LNumber(v)
	case uint32:
		return lua.LNumber(v)
	case uint64:
		return lua.LNumber(v)
	case map[string][]string:
		lt := l.CreateTable(0, len(v))
		for k, v := range v {
			lt.RawSetString(k, Value(l, v))
		}
		return lt
	case map[string]string:
		return MapString(l, v)
	case map[string]int64:
		return MapInt64(l, v)
	case map[string]interface{}:
		return Map(l, v)
	case []string:
		lt := l.CreateTable(len(val.([]string)), 0)
		for k, v := range v {
			lt.RawSetInt(k+1, lua.LString(v))
		}
		return lt
	case []interface{}:
		lt := l.CreateTable(len(val.([]interface{})), 0)
		for k, v := range v {
			lt.RawSetInt(k+1, Value(l, v))
		}
		return lt
	case time.Time:
		return lua.LNumber(v.UTC().Unix())
	case nil:
		return lua.LNil
	default:
		// Never return an actual Go `nil` or it will cause nil pointer dereferences inside gopher-lua.
		return lua.LNil
	}
}
