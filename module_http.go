package runtimelua

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deflinhec/runtimelua/luaconv"
	"go.uber.org/zap"

	lua "github.com/yuin/gopher-lua"
)

type localHTTPModule struct {
	localRuntimeModule
	http.Client

	logger *zap.Logger
}

func (m *localHTTPModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"request":       m.request,
			"request_async": m.request_async,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

func (m *localHTTPModule) request(l *lua.LState) int {
	url := l.CheckString(1)
	method := strings.ToUpper(l.CheckString(2))
	headers := l.CheckTable(3)
	body := l.OptString(4, "")

	if url == "" {
		l.ArgError(1, "expects URL string")
		return 0
	}

	switch method {
	case http.MethodGet:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	case http.MethodHead:
	default:
		l.ArgError(2, "expects method to be one of: 'get', 'post', 'put', 'patch', 'delete', 'head'")
		return 0
	}

	// Set a custom timeout if one is provided, or use the default.
	timeoutMs := l.OptInt64(5, 5000)
	if timeoutMs <= 0 {
		timeoutMs = 5_000
	}

	// Prepare request body, if any.
	var requestBody io.Reader
	if body != "" {
		requestBody = strings.NewReader(body)
	}

	ctx, ctxCancelFn := context.WithTimeout(l.Context(), time.Duration(timeoutMs)*time.Millisecond)
	defer ctxCancelFn()

	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		l.RaiseError("HTTP request error: %v", err.Error())
		return 0
	}
	// Apply any request headers.
	httpHeaders := luaconv.Table(headers)
	for k, v := range httpHeaders {
		vs, ok := v.(string)
		if !ok {
			l.RaiseError("HTTP header values must be strings")
			return 0
		}
		req.Header.Add(k, vs)
	}
	// Execute the request.
	resp, err := m.Do(req)
	if err != nil {
		l.RaiseError("HTTP request error: %v", err.Error())
		return 0
	}
	// Read the response body.
	responseBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		l.RaiseError("HTTP response body error: %v", err.Error())
		return 0
	}
	// Read the response headers.
	responseHeaders := make(map[string]interface{}, len(resp.Header))
	for k, vs := range resp.Header {
		// TODO accept multiple values per header
		for _, v := range vs {
			responseHeaders[k] = v
			break
		}
	}

	l.Push(lua.LNumber(resp.StatusCode))
	l.Push(luaconv.Map(l, responseHeaders))
	l.Push(lua.LString(string(responseBody)))
	return 3
}

func (m *localHTTPModule) request_async(l *lua.LState) int {
	fn := l.CheckFunction(1)
	url := l.CheckString(2)
	method := strings.ToUpper(l.CheckString(3))
	headers := l.CheckTable(4)
	body := l.OptString(5, "")

	if fn == nil {
		l.ArgError(1, "expects function")
		return 0
	}

	if url == "" {
		l.ArgError(2, "expects URL string")
		return 0
	}

	switch method {
	case http.MethodGet:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	case http.MethodHead:
	default:
		l.ArgError(3, "expects method to be one of: 'get', 'post', 'put', 'patch', 'delete', 'head'")
		return 0
	}

	// Set a custom timeout if one is provided, or use the default.
	timeoutMs := l.OptInt64(6, 5000)
	if timeoutMs <= 0 {
		timeoutMs = 5_000
	}

	// Prepare request body, if any.
	var requestBody io.Reader
	if body != "" {
		requestBody = strings.NewReader(body)
	}

	ctx, ctxCancelFn := context.WithTimeout(l.Context(), time.Duration(timeoutMs)*time.Millisecond)

	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		ctxCancelFn()
		l.RaiseError("HTTP request error: %v", err.Error())
		return 0
	}
	// Apply any request headers.
	httpHeaders := luaconv.Table(headers)
	for k, v := range httpHeaders {
		vs, ok := v.(string)
		if !ok {
			ctxCancelFn()
			l.RaiseError("HTTP header values must be strings")
			return 0
		}
		req.Header.Add(k, vs)
	}
	e := &localHTTPRequestEvent{
		ctxCancelFn: ctxCancelFn,
		Client:      m.Client,
		Request:     req,
		Func:        fn,
	}
	m.runtime.EventQueue <- e
	return 0
}

type localHTTPRequestEvent struct {
	StateEvent
	*http.Request
	http.Client
	ctxCancelFn context.CancelFunc
	Func        *lua.LFunction
	resp        *http.Response
	err         error
}

func (e *localHTTPRequestEvent) Update(d time.Duration, l *lua.LState) error {
	switch EventState(e.Load()) {
	case EVENT_STATE_INITIALIZE:
		e.Store(uint32(EVENT_STATE_PROGRESS))
		go func() {
			defer e.Store(uint32(EVENT_STATE_FINALIZE))
			if resp, err := e.Do(e.Request); err != nil {
				e.err = fmt.Errorf("HTTP request error: %v", err.Error())
			} else {
				e.resp = resp
			}
		}()
	case EVENT_STATE_FINALIZE:
		resp := e.resp
		var body []byte
		var headers map[string]interface{}
		defer e.Store(uint32(EVENT_STATE_COMPLETE))
		defer e.ctxCancelFn()
		for headers != nil {
			if e.err != nil {
				break
			}
			// Read the response body.
			body, e.err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if e.err != nil {
				e.err = fmt.Errorf("HTTP response body error: %v", e.err.Error())
				break
			}
			// Read the response headers.
			headers = make(map[string]interface{}, len(resp.Header))
			for k, vs := range resp.Header {
				// TODO accept multiple values per header
				for _, v := range vs {
					headers[k] = v
					break
				}
			}
		}
		l.Push(e.Func)
		if e.err != nil {
			l.Push(lua.LBool(false))
			l.Push(lua.LString(e.err.Error()))
			l.Push(lua.LNil)
			l.Push(lua.LNil)
			l.Push(lua.LNil)
		} else {
			l.Push(lua.LBool(true))
			l.Push(lua.LNil)
			l.Push(lua.LNumber(resp.StatusCode))
			l.Push(luaconv.Map(l, headers))
			l.Push(lua.LString(string(body)))
		}
		return l.PCall(5, 0, nil)
	}
	return nil
}
