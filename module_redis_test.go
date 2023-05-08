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
