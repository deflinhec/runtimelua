package module

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deflinhec/runtimelua/event"
	"github.com/deflinhec/runtimelua/luaconv"

	lua "github.com/yuin/gopher-lua"
)

type httpModule struct {
	RuntimeModule
	http.Client
}

func HttpModule(runtime Runtime) *httpModule {
	return &httpModule{
		RuntimeModule: RuntimeModule{
			Module: Module{
				name: "http",
			},
			runtime: runtime,
		},
	}
}

func (m *httpModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"request":       m.request,
			"request_async": m.request_async,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

func (m *httpModule) request(l *lua.LState) int {
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

func (m *httpModule) request_async(l *lua.LState) int {
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
	m.runtime.EventQueue() <- &httpRequestEvent{
		ctxCancelFn: ctxCancelFn,
		Client:      m.Client,
		Request:     req,
		VM:          l,
		Func:        fn,
	}
	return 0
}

type httpRequestEvent struct {
	event.StateEvent
	*http.Request
	http.Client
	ctxCancelFn context.CancelFunc
	VM          *lua.LState
	Func        *lua.LFunction
	resp        *http.Response
	err         error
}

func (e *httpRequestEvent) Update(time.Duration) error {
	switch event.State(e.Load()) {
	case event.INITIALIZE:
		e.Store(uint32(event.PROGRESS))
		go func() {
			defer e.Store(uint32(event.FINALIZE))
			if resp, err := e.Do(e.Request); err != nil {
				e.err = fmt.Errorf("HTTP request error: %v", err.Error())
			} else {
				e.resp = resp
			}
		}()
	case event.FINALIZE:
		resp := e.resp
		var body []byte
		var headers map[string]interface{}
		defer e.Store(uint32(event.COMPLETE))
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
		e.VM.Push(e.Func)
		if e.err != nil {
			e.VM.Push(lua.LBool(false))
			e.VM.Push(lua.LString(e.err.Error()))
			e.VM.Push(lua.LNil)
			e.VM.Push(lua.LNil)
			e.VM.Push(lua.LNil)
		} else {
			e.VM.Push(lua.LBool(true))
			e.VM.Push(lua.LNil)
			e.VM.Push(lua.LNumber(resp.StatusCode))
			e.VM.Push(luaconv.Map(e.VM, headers))
			e.VM.Push(lua.LString(string(body)))
		}
		return e.VM.PCall(5, 0, nil)
	}
	return nil
}
