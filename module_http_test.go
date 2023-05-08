package runtimelua_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/deflinhec/runtimelua"
	"go.uber.org/zap"
)

func TestHttpModuleRequest(t *testing.T) {
	// Start a local http server
	svr := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer svr.Close()

	wg := &sync.WaitGroup{}
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{T: t}

	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
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
		runtime.exit()
		`,
	},
		runtimelua.WithContext(ctx),
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithModuleHttp(logger),
		runtimelua.WithModule(test),
	)
	wg.Wait()
}

func TestHttpModuleAsyncRequest(t *testing.T) {
	// Start a local http server
	svr := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer svr.Close()

	// Setup shared context
	wg := &sync.WaitGroup{}
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	test := &TestingModule{T: t}

	newRuntimeWithModules(t, map[string]string{
		"main": `
		local runtime = require 'runtime'
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
		local url = "` + svr.URL + `"
		local success, err = pcall(http.request_async,
			function(code, headers, body)
				if code ~= 200 then
					test.fatal(body)
				end
				runtime.exit()
			end, 
			url, "GET", const.headers, content, const.timeout
		)
		if not success then
			test.fatal("request failed"..err)
		elseif err ~= nil then
			test.fatal(err)
		end
		`,
	},
		runtimelua.WithWaitGroup(wg),
		runtimelua.WithContext(ctx),
		runtimelua.WithModule(test),
		runtimelua.WithModuleEvent(logger),
		runtimelua.WithModuleHttp(logger),
	)
	wg.Wait()
}
