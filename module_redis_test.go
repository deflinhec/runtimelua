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
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/alicebob/miniredis"
)

func TestRedisModuleGetSet(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Setup shared context
	wg := &sync.WaitGroup{}
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{T: t}

	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
		local test = require 'test'
		local redis = require 'redis'
		redis.set("key", 1)
		local v = redis.get("key")
		if v ~= 1 then
			test.fatal(("%q"):format(v))
		end
		runtime.exit()
		`,
	},
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithContext(ctx),
		runtimelua.WithModule(test),
		runtimelua.WithModuleRedis(logger,
			&redis.Options{
				Addr: s.Addr(),
			}),
	)
	wg.Wait()
}

func TestRedisModulePubSub(t *testing.T) {
	// Setup shared context
	wg := &sync.WaitGroup{}
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{T: t}

	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
		local test = require 'test'
		local redis = require 'redis'
		local event = require 'event'
		redis.subscribe("sub", function(v)
			if v ~= 1 then
				test.fatal(("%q"):format(v))
			end
			runtime.exit()
		end)
		event.delay(5, function(_)
			test.fatal("timeout")
			runtime.exit()
		end)
		`,
	},
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithContext(ctx),
		runtimelua.WithModule(test),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleRedis(logger,
			&redis.Options{
				Addr: "localhost:6379",
			}),
	)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
		local test = require 'test'
		local redis = require 'redis'
		local event = require 'event'
		redis.publish("sub", 1)
		runtime.exit()
		`,
	},
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithContext(ctx),
		runtimelua.WithModule(test),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleRedis(logger,
			&redis.Options{
				Addr: "localhost:6379",
			}),
	)
	wg.Wait()
}
