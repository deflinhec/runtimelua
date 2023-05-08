// Copyright 2023 Deflinhec
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
