package runtimelua_test

import (
	"context"
	"testing"
	"time"

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
	ctx, ctxCancelFn := context.WithTimeout(
		context.Background(), time.Second*10,
	)
	defer ctxCancelFn()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{cancel: ctxCancelFn}
	defer test.validate(t)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local test = require 'test'
		local redis = require 'redis'
		redis.set("key", 1)
		local v = redis.get("key")
		if v ~= 1 then
			test.fatal(("%q"):format(v))
		end
		test.done()
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleRedis(logger,
			&redis.Options{
				Addr: s.Addr(),
			}),
		runtimelua.WithModule(test),
	).Wait()
}

func TestRedisModulePubSub(t *testing.T) {
	ctx, ctxCancelFn := context.WithTimeout(
		context.Background(), time.Second*10,
	)
	defer ctxCancelFn()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{cancel: ctxCancelFn}
	defer test.validate(t)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local test = require 'test'
		local redis = require 'redis'
		redis.subscribe("sub", function(v)
			if v ~= 1 then
				test.fatal(("%q"):format(v))
			end
			test.done()
		end)
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleRedis(logger,
			&redis.Options{
				Addr: "localhost:6379",
			}),
		runtimelua.WithModule(test),
	)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local test = require 'test'
		local redis = require 'redis'
		local event = require 'event'
		redis.publish("sub", 1)
		event.delay(5, function(_timer)
			test.fatal("timeout")
		end)
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleRedis(logger,
			&redis.Options{
				Addr: "localhost:6379",
			}),
		runtimelua.WithModule(test),
	).Wait()
}
