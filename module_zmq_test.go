//go:build zmq
// +build zmq

package runtimelua_test

import (
	"context"
	"testing"
	"time"

	"github.com/deflinhec/runtimelua"
	"go.uber.org/zap"
)

func TestZmqModule(t *testing.T) {
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
		local zmq = require 'zmq'
		local router = zmq.router(5555,
			function(router, payload, sender)
				print("router sender", sender)
				print("router payload", payload)
				assert(type(payload)=='table')
				assert(type(payload.key)=='number')
				sender:send(123)
			end
		)
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleZmq(logger),
		runtimelua.WithModule(test),
	)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local test = require 'test'
		local event = require 'event'
		local zmq = require 'zmq'
		local dealer = zmq.dealer("localhost:5555",
			function(dealer, payload, sender)
				assert(type(payload)=='number')
				assert(type(sender)=='nil')
				print("dealer sender", sender)
				print("dealer payload", payload)
				test.done()
			end
		)
		dealer:send({key=123})
		event.delay(5, function(_timer)
			test.fatal("timeout")
		end)
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleZmq(logger),
		runtimelua.WithModule(test),
	).Wait()
}
