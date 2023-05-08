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
