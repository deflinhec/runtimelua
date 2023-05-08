// Copyright 2023 Deflinhec
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"github.com/deflinhec/runtimelua/luaconv"
	"go.uber.org/zap"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/zeromq/goczmq.v4"
)

// ZMQ Require extra library dependency.
type localZmqModule struct {
	localRuntimeModule

	logger *zap.Logger
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

	e := &localZmqEvent{
		Channeler: goczmq.NewDealerChanneler(
			u.String(),
		),
		fn: fn,
	}

	m.runtime.EventQueue <- e
	l.Push(e.LuaValue(l))
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

	e := &localZmqEvent{
		Channeler: goczmq.NewRouterChanneler(
			fmt.Sprintf("tcp://*:%v", port),
		),
		fn: fn,
	}

	m.runtime.EventQueue <- e
	l.Push(e.LuaValue(l))
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
	StateEvent
	sync.Mutex
	queue []*zmqPacket
	bind  *lua.LTable
	fn    *lua.LFunction
}

// Initialize an goroutine which process received request,
// and handle ZMQPacket within PROGRESS state.
func (e *localZmqEvent) Update(d time.Duration, l *lua.LState) error {
	switch EventState(e.Load()) {
	case EVENT_STATE_INITIALIZE:
		e.Store(uint32(EVENT_STATE_PROGRESS))
		e.queue = make([]*zmqPacket, 0, 128)
		go func() {
			defer e.Store(uint32(EVENT_STATE_COMPLETE))
			for request := range e.RecvChan {
				packet := &zmqPacket{}
				if err := packet.Deserialize(request); err != nil {
					log.Println("malformed packet", err)
					continue
				}
				e.Lock()
				e.queue = append(e.queue, packet)
				e.Unlock()
			}
		}()
	case EVENT_STATE_PROGRESS:
		var packet *zmqPacket
		e.Lock()
		defer e.Unlock()
		for len(e.queue) > 0 {
			packet, e.queue = e.queue[0], e.queue[1:]
			sender := &zmqSender{
				frame:     packet.sender,
				Channeler: e.Channeler,
			}
			l.Push(e.fn)
			l.Push(e.bind)
			l.Push(luaconv.Value(l, packet.Payload))
			l.Push(sender.LuaValue(l))
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
	e.Store(uint32(EVENT_STATE_COMPLETE))
	e.Channeler.Destroy()
}

// Stop ZMQEvent and destory it.
func (e *localZmqEvent) Stop() {
	e.Destory()
}

func (e *localZmqEvent) stop(l *lua.LState) int {
	l.CheckTable(1)
	e.Stop()
	return 0
}

func (e *localZmqEvent) send(l *lua.LState) int {
	l.CheckTable(1)
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

func (e *localZmqEvent) LuaValue(l *lua.LState) lua.LValue {
	if e.bind != nil {
		return e.bind
	}
	functions := map[string]lua.LGFunction{
		"stop": e.stop,
		"send": e.send,
	}
	e.bind = l.SetFuncs(l.CreateTable(0, len(functions)), functions)
	return e.bind
}

type zmqSender struct {
	*goczmq.Channeler
	frame []byte
	bind  *lua.LTable
}

func (s *zmqSender) send(l *lua.LState) int {
	l.CheckTable(1)
	packet := &zmqPacket{
		sender:  s.frame,
		Payload: luaconv.LuaValue(l.CheckAny(2)),
	}
	b, err := packet.Serialize()
	if err != nil {
		l.RaiseError(err.Error())
		return 0
	}
	s.SendChan <- b
	return 0
}

func (s *zmqSender) LuaValue(l *lua.LState) lua.LValue {
	switch {
	case s.frame == nil:
		fallthrough
	case len(s.frame) == 0:
		return lua.LNil
	}
	if s.bind != nil {
		return s.bind
	}
	functions := map[string]lua.LGFunction{
		"send": s.send,
	}
	s.bind = l.CreateTable(0, len(functions)+1)
	s.bind.RawSetString("frame", lua.LString(s.frame))
	s.bind = l.SetFuncs(s.bind, functions)
	return s.bind
}
