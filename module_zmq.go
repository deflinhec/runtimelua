//go:build zmq
// +build zmq

package runtimelua

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
type localZmqModule struct {
	localRuntimeModule
}

func (m *localZmqModule) Open() lua.LGFunction {
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
func (m *localZmqModule) dealer(l *lua.LState) int {
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

	stub := l.CreateTable(0, len(localZmqEventFuncs))
	stub = l.SetFuncs(stub, localZmqEventFuncs)
	e := &localZmqEvent{
		Channeler: goczmq.NewDealerChanneler(
			u.String(),
		),
		bind: stub, fn: fn,
	}
	stub.Metatable = &lua.LUserData{Value: e}

	m.runtime.eventQueue <- e
	l.Push(stub)
	return 1
}

// Starts a router channeler
func (m *localZmqModule) router(l *lua.LState) int {
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

	stub := l.CreateTable(0, len(localZmqEventFuncs))
	stub = l.SetFuncs(stub, localZmqEventFuncs)
	e := &localZmqEvent{
		Channeler: goczmq.NewRouterChanneler(
			fmt.Sprintf("tcp://*:%v", port),
		),
		bind: stub,
		fn:   fn,
	}
	stub.Metatable = &lua.LUserData{Value: e}

	m.runtime.eventQueue <- e
	l.Push(stub)
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

type localZmqEvent struct {
	*goczmq.Channeler
	event.StateEvent
	sync.Mutex
	queue []*zmqPacket
	bind  *lua.LTable
	fn    *lua.LFunction
}

// Initialize an goroutine which process received request,
// and handle ZMQPacket within PROGRESS state.
func (e *localZmqEvent) Update(d time.Duration, l *lua.LState) error {
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
			l.Push(e.fn)
			l.Push(e.bind)
			l.Push(luaconv.Value(l, packet.Payload))
			l.Push(e.Sender(l, packet.sender))
			if err := l.PCall(3, 0, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// Overwrite Destory function which inherit from goczmq.Channeler,
// making sure ZMQEvent will be stop when it's called.
func (e *localZmqEvent) Destory() {
	e.Store(uint32(event.COMPLETE))
	e.Channeler.Destroy()
}

// Stop ZMQEvent and destory it.
func (e *localZmqEvent) Stop() {
	e.Destory()
}

// Extracting ZMQEvent from lua table.
func checkZMQEvent(l *lua.LState, n int) *localZmqEvent {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*localZmqEvent); ok {
				return e
			}
		}
	}
	l.ArgError(n, "expect ZMQEvent")
	return nil
}

var localZmqEventFuncs = map[string]lua.LGFunction{
	"send": local_zmq_event_send,
	"stop": local_zmq_event_stop,
}

// Function binding which send ZMQPacket to another.
func local_zmq_event_send(l *lua.LState) int {
	e := checkZMQEvent(l, 1)
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
func local_zmq_event_stop(l *lua.LState) int {
	e := checkZMQEvent(l, 1)
	if e == nil {
		return 0
	}
	e.Stop()
	return 0
}

// Specific key to store frame info on lua table.
const (
	zmqEventSender = "__frame"
)

// Extracting ZMQEvent and frame info from lua table.
func CheckZMQEventSender(l *lua.LState, n int) (*localZmqEvent, []byte) {
	if self := l.CheckTable(1); self != nil {
		if data, ok := self.Metatable.(*lua.LUserData); ok {
			if e, ok := data.Value.(*localZmqEvent); ok {
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
func (e *localZmqEvent) Sender(l *lua.LState, frame []byte) lua.LValue {
	switch {
	case frame == nil:
		fallthrough
	case len(frame) == 0:
		return lua.LNil
	}
	funcs := zmqEventSenderFuncs
	sender := l.SetFuncs(l.CreateTable(0, len(funcs)+1), funcs)
	sender.RawSetString(zmqEventSender, luaconv.Value(l, frame))
	sender.Metatable = &lua.LUserData{Value: e}
	return sender
}
