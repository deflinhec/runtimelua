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
	"crypto/md5"
	"encoding/hex"

	lua "github.com/yuin/gopher-lua"
)

var (
	MD5LibName = "base64"
)

var md5Funcs = map[string]lua.LGFunction{
	"sum": md5Sum,
}

func OpenMD5(l *lua.LState) int {
	mod := l.RegisterModule(MD5LibName, md5Funcs)
	l.Push(mod)
	return 1
}

func md5Sum(l *lua.LState) int {
	ctx := l.CheckString(1)
	hash := md5.Sum([]byte(ctx))
	l.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}
