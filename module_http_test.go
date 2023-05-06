package runtimelua_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deflinhec/runtimelua"
	"go.uber.org/zap"
)

func TestHttpModuleRequest(t *testing.T) {
	svr := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer svr.Close()
	ctx, ctxCancelFn := context.WithTimeout(
		context.Background(), time.Second*10,
	)
	defer ctxCancelFn()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{cancel: ctxCancelFn}
	defer test.validate(t)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local http = require 'http'
		local test = require 'test'
		local const = {
			timeout = 1000, -- 1 seconds timeout
			headers = {
				["Content-Type"] = "application/json",
				["Accept"] = "application/json"
			},
		}
		local url = "` + svr.URL + `"
		local success, code, headers, body = pcall(http.request,
			url, "GET", const.headers, content, const.timeout
		)
		if not success then
			test.fatal("request failed")
		elseif code >= 400 then
			test.fatal(("http code %q"):format(code))
		end
		test.done()
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleHttp(logger),
		runtimelua.WithModule(test),
	).Wait()
}

func TestHttpModuleAsyncRequest(t *testing.T) {
	svr := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer svr.Close()
	ctx, ctxCancelFn := context.WithTimeout(
		context.Background(), time.Second*10,
	)
	defer ctxCancelFn()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{cancel: ctxCancelFn}
	defer test.validate(t)
	newRuntimeWithModules(t, map[string]string{
		"main": `
		local event = require 'event'
		local http = require 'http'
		local test = require 'test'
		local const = {
			timeout = 1000, -- 1 seconds timeout
			headers = {
				["Content-Type"] = "application/json",
				["Accept"] = "application/json"
			},
		}
		local function async(success, err, code, headers, body)
			if not success then
				test.fatal(err)
			elseif code >= 400 then
				test.fatal(("http code %q"):format(code))
			end
			test.done()
		end
		local url = "` + svr.URL + `"
		local success, err = pcall(http.request_async,
			async, url, "GET", const.headers, content, const.timeout
		)
		if not success then
			test.fatal("request failed"..err)
		elseif err ~= nil then
			test.fatal(err)
		end
		event.delay(5, function(_timer)
			test.fatal("timeout")
		end)
		`,
	}, runtimelua.WithContext(ctx),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleHttp(logger),
		runtimelua.WithModule(test),
	).Wait()
}
