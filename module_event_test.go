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

package runtimelua_test

import (
	"context"
	"sync"
	"testing"

	"github.com/deflinhec/runtimelua"
	"go.uber.org/zap"
)

func TestEventModule(t *testing.T) {
	// Setup shared context
	wg := &sync.WaitGroup{}
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{T: t}

	defer newRuntimeWithModules(t, map[string]string{
		"main": `
		local looptimer, delaytimer
		local test = require 'test'
		local event = require 'event'
		local count = {loop=0,delay=0}
		-- Start a loop timer
		looptimer = event.loop(1,
			function (_timer, ...)
				count.loop = count.loop + 1
				return true
			end
		)
		-- Stop loop timer after 5 sec
		delaytimer = event.delay(5,
			function (_timer, ...)
				looptimer:stop()
				count.delay = count.delay + 1
			end
		)
		-- Validate test result
		event.delay(7, function(_)
			if count.loop ~= 5 then
				test.fatal("loop count not equal")
			elseif count.delay ~= 1 then
				test.fatal("delay count not equal")
			elseif not delaytimer:valid() then
				test.fatal("delaytimer valid")
			elseif not looptimer:valid() then
				test.fatal("looptimer valid")
			end
			runtime.exit()
		end)
		`,
	},
		runtimelua.WithContext(ctx),
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithModule(test),
		runtimelua.WithModuleEvent(logger),
	)
	wg.Wait()
}
