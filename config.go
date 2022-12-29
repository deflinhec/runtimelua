package runtimelua

import lua "github.com/yuin/gopher-lua"

type Config interface {
	GetLuaOptions() lua.Options
	// Maximim event queue size.
	GetEventQueueSize() int
	// Lua script entry.
	GetScriptEntry() string
}
