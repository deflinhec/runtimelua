//go:build zmq
// +build zmq

package runtimelua_test

import (
	"context"
	"sync"
	"testing"

	"github.com/deflinhec/runtimelua"
	"go.uber.org/zap"
)

func TestZmqModule(t *testing.T) {
	// Setup shared context
	wg := &sync.WaitGroup{}
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{T: t}

	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
		local event = require 'event'
		local test = require 'test'
		local zmq = require 'zmq'
		local router = zmq.router(5555,
			function(router, payload, sender)
				print("router, payload, sender", 
					router, payload, sender)
				if type(payload) ~= 'table' then
					test.fatal("payload is not table")
				elseif type(payload.key) ~= 'number' then
					test.fatal("payload.key is not number")
				end
				sender:send(123)
				event.delay(1, function(_timer)
					runtime.exit()
				end)
			end
		)
		`,
	},
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithContext(ctx),
		runtimelua.WithModule(test),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleZmq(logger),
	)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
		local test = require 'test'
		local event = require 'event'
		local zmq = require 'zmq'
		local dealer = zmq.dealer("localhost:5555",
			function(dealer, payload, sender)
				print("dealer, payload, sender", 
					dealer, payload, sender)
				if type(payload) ~= 'number' then
					test.fatal("payload is not number")
				elseif type(sender) ~= 'nil' then
					test.fatal("sender should be nil")
				end
				runtime.exit()
			end
		)
		dealer:send({key=123})
		event.delay(5, function(_timer)
			test.fatal("timeout")
			runtime.exit()
		end)
		`,
	},
		runtimelua.WithContext(ctx),
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithModule(test),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleZmq(logger),
	)
	wg.Wait()
}
