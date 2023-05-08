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
