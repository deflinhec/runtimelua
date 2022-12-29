//go:build zmq
// +build zmq

package module

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/deflinhec/runtimelua/event"
	"github.com/deflinhec/runtimelua/luaconv"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/zeromq/goczmq.v4"
)

// ZMQ Require extra library dependency.
type zmqModule struct {
	RuntimeModule
}

func ZMQModule(runtime Runtime) *zmqModule {
	return &zmqModule{
		RuntimeModule: RuntimeModule{
			Module: Module{
				name: "zmq",
			},
			runtime: runtime,
		},
	}
}

func (m *zmqModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"dealer": m.dealer,
			"router": m.router,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

// Starts a dealer channeler
func (m *zmqModule) dealer(l *lua.LState) int {
	address := l.CheckString(1)
	u, err := url.Parse(fmt.Sprintf("tcp://%v", address))
	if err != nil {
		l.ArgError(1, "expect valid url")
		return 0
	}

	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(1, "expect function handler")
		return 0
	}
	e := &zmqEvent{
		Channeler: goczmq.NewDealerChanneler(
			u.String(),
		),
		VM:   l,
		Func: fn,
	}
	m.runtime.EventQueue() <- e
	l.Push(e.Self())
	return 1
}

// Starts a router channeler
func (m *zmqModule) router(l *lua.LState) int {
	port := l.CheckNumber(1)
	if port <= 1000 {
		l.ArgError(1, "invalid port: port under 1000 are usually reserved by system")
		return 0
	}
	if port > math.MaxUint16 {
		l.ArgError(1, "invalid port: port should not exceed 65535")
		return 0
	}
	fn := l.CheckFunction(2)
	if fn == nil {
		l.ArgError(1, "expect function handler")
		return 0
	}
	e := &zmqEvent{
		Channeler: goczmq.NewRouterChanneler(
			fmt.Sprintf("tcp://*:%v", port),
		),
		VM:   l,
		Func: fn,
	}
	m.runtime.EventQueue() <- e
	l.Push(e.Self())
	return 1
}

type zmqPacket struct {
	sender  []byte
	Payload interface{}
}

// Serialize message and send it toward SendChan, if sender frame exist
// and valid, then this message was send from a router as a response to
// the sender.
func (p *zmqPacket) Serialize() ([][]byte, error) {
	packet := make([][]byte, 0, 2)
	if p.sender != nil && len(p.sender) > 0 {
		packet = append(packet, p.sender)
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return append(packet, buf.Bytes()), nil
}

// Deserialize request from RecvChan, the last byte array will be
// the packet payload, so if there are more than one byte array, then
// this message came from a dealer, used the first byte array as the
// sender frame for later on.
func (p *zmqPacket) Deserialize(b [][]byte) error {
	data := b[len(b)-1]
	r := bytes.NewReader(data)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(p); err != nil {
		return err
	} else if len(b) > 1 {
		p.sender = b[0]
	}
	return nil
}

type zmqEvent struct {
	*goczmq.Channeler
	event.StateEvent
	sync.Mutex
	queue []*zmqPacket
	self  *lua.LTable
	Func  *lua.LFunction
	VM    *lua.LState
}

// Initialize an goroutine which process received request,
// and handle ZMQPacket within PROGRESS state.
func (e *zmqEvent) Update(time.Duration) error {
	switch event.State(e.Load()) {
	case event.INITIALIZE:
		e.Store(uint32(event.PROGRESS))
		e.queue = make([]*zmqPacket, 0, 128)
		go func() {
			defer e.Store(uint32(event.COMPLETE))
			for request := range e.RecvChan {
				packet := &zmqPacket{}
				if err := packet.Deserialize(request); err != nil {
					log.Println("malformed packet", err)
					continue
				}
				e.Lock()
				e.queue = append(e.queue, packet)
				e.Unlock()
				log.Println(packet)
			}
		}()
	case event.PROGRESS:
		var packet *zmqPacket
		e.Lock()
		defer e.Unlock()
		for len(e.queue) > 0 {
			packet, e.queue = e.queue[0], e.queue[1:]
			e.VM.Push(e.Func)
			e.VM.Push(e.Self())
			e.VM.Push(luaconv.Value(e.VM, packet.Payload))
			e.VM.Push(e.Sender(packet.sender))
			if err := e.VM.PCall(3, 0, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// Overwrite Destory function which inherit from goczmq.Channeler,
// making sure ZMQEvent will be stop when it's called.
func (e *zmqEvent) Destory() {
	e.Store(uint32(event.COMPLETE))
	e.Channeler.Destroy()
}

// Stop ZMQEvent and destory it.
func (e *zmqEvent) Stop() {
	e.Destory()
}

// Extracting ZMQEvent from lua table.
func CheckZMQEvent(l *lua.LState, n int) *zmqEvent {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*zmqEvent); ok {
				return e
			}
		}
	}
	l.ArgError(n, "expect ZMQEvent")
	return nil
}

var (
	zmqEventFuncs = map[string]lua.LGFunction{
		"send": zmq_event_send,
		"stop": zmq_event_stop,
	}
)

// Function binding which send ZMQPacket to another.
func zmq_event_send(l *lua.LState) int {
	e := CheckZMQEvent(l, 1)
	if e == nil {
		return 0
	}
	packet := &zmqPacket{
		sender:  []byte(l.OptString(3, "")),
		Payload: luaconv.LuaValue(l.CheckAny(2)),
	}
	b, err := packet.Serialize()
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	e.SendChan <- b
	return 0
}

// Function binding for stoping ZMQEvent.
func zmq_event_stop(l *lua.LState) int {
	e := CheckZMQEvent(l, 1)
	if e == nil {
		return 0
	}
	e.Stop()
	return 0
}

// Create a lua table for ZMQEvent, allowing access from lua script.
func (e *zmqEvent) Self() lua.LValue {
	if e.self != nil {
		return e.self
	}
	funcs := zmqEventFuncs
	e.self = e.VM.SetFuncs(e.VM.CreateTable(0, len(funcs)), funcs)
	e.self.Metatable = &lua.LUserData{Value: e}
	return e.self
}

// Specific key to store frame info on lua table.
const (
	zmqEventSender = "__frame"
)

// Extracting ZMQEvent and frame info from lua table.
func CheckZMQEventSender(l *lua.LState, n int) (*zmqEvent, []byte) {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*zmqEvent); ok {
				v := self.RawGetString(zmqEventSender)
				return e, []byte(v.String())
			}
		}
	}
	l.ArgError(n, "expect ZMQEventSender")
	return nil, nil
}

var (
	zmqEventSenderFuncs = map[string]lua.LGFunction{
		"send": zmq_sender_send,
	}
)

// Function bind for sender object
func zmq_sender_send(l *lua.LState) int {
	e, frame := CheckZMQEventSender(l, 1)
	if e == nil {
		return 0
	}
	packet := &zmqPacket{
		sender:  frame,
		Payload: luaconv.LuaValue(l.CheckAny(2)),
	}
	b, err := packet.Serialize()
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	e.SendChan <- b
	return 0
}

// Create a sender object for router to reply to
func (e *zmqEvent) Sender(frame []byte) lua.LValue {
	switch {
	case frame == nil:
		fallthrough
	case len(frame) == 0:
		return lua.LNil
	}
	funcs := zmqEventSenderFuncs
	sender := e.VM.SetFuncs(e.VM.CreateTable(0, len(funcs)+1), funcs)
	sender.RawSetString(zmqEventSender, luaconv.Value(e.VM, frame))
	sender.Metatable = &lua.LUserData{Value: e}
	return sender
}
