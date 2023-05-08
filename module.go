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

package runtimelua

import (
	"encoding/gob"

	lua "github.com/yuin/gopher-lua"
)

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

type Module interface {
	Name() string
	Open() lua.LGFunction
}

type localModule struct {
	name string
}

func (m *localModule) Name() string {
	return m.name
}

type RuntimeModule interface {
	Module
	// Provide Initialization
	InitializeRuntime(*Runtime)
}

type localRuntimeModule struct {
	localModule
	runtime *Runtime
}
