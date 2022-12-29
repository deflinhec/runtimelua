package auxlib

import (
	lua "github.com/yuin/gopher-lua"
)

var (
	Bit64LibName = "bit64"
)

var bit64Funcs = map[string]lua.LGFunction{
	"brshift": bit64brshift,
	"blshift": bit64blshift,
	"band":    bit64band,
	"bnot":    bit64bnot,
	"bxor":    bit64bxor,
	"bor":     bit64bor,
}

func OpenBit64(l *lua.LState) int {
	mod := l.RegisterModule(Bit64LibName, bit64Funcs)
	l.Push(mod)
	return 1
}

func bit64brshift(l *lua.LState) int {
	lhs, rhs := l.CheckInt64(1), l.CheckInt64(2)
	l.Push(lua.LNumber(uint64(lhs) >> uint64(rhs)))
	return 1
}

func bit64blshift(l *lua.LState) int {
	lhs, rhs := l.CheckInt64(1), l.CheckInt64(2)
	l.Push(lua.LNumber(uint64(lhs) << uint64(rhs)))
	return 1
}

func bit64band(l *lua.LState) int {
	lhs, rhs := l.CheckInt64(1), l.CheckInt64(2)
	l.Push(lua.LNumber(uint64(lhs) & uint64(rhs)))
	return 1
}

func bit64bor(l *lua.LState) int {
	lhs, rhs := l.CheckInt64(1), l.CheckNumber(2)
	l.Push(lua.LNumber(uint64(lhs) | uint64(rhs)))
	return 1
}

func bit64bxor(l *lua.LState) int {
	lhs, rhs := l.CheckInt64(1), l.CheckInt64(2)
	l.Push(lua.LNumber(uint64(lhs) ^ uint64(rhs)))
	return 1
}

func bit64bnot(l *lua.LState) int {
	l.Push(lua.LNumber(^uint64(l.CheckInt64(1))))
	return 1
}
