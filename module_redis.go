package runtimelua

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/deflinhec/runtimelua/luaconv"

	"github.com/go-redis/redis/v8"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

var rdbs sync.Map

type localRedisSubscriber struct {
	sync.Mutex
	*redis.PubSub

	runtime     *Runtime
	vm          *lua.LState
	functions   []*lua.LFunction
	ctx         context.Context
	ctxCancelFn context.CancelFunc
}

func (s *localRedisSubscriber) Startup() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.Channel():
			s.Lock()
			for _, fn := range s.functions {
				e := &pubSubEvent{
					Arguments: []lua.LValue{
						decode(s.vm, msg.Payload),
					},
					Func: fn,
				}
				s.runtime.EventQueue <- e
			}
			s.Unlock()
		}
	}
}

func (s *localRedisSubscriber) Append(fn *lua.LFunction) {
	s.Lock()
	defer s.Unlock()
	s.functions = append(s.functions, fn)
}

func (s *localRedisSubscriber) Close() error {
	s.ctxCancelFn()
	return s.PubSub.Close()
}

type localRedisModule struct {
	localRuntimeModule

	logger *zap.Logger
	pool   *sync.Pool
	pubsub map[string]*localRedisSubscriber
}

func connect_redis(opt *redis.Options) *sync.Pool {
	if opt != nil {
		if pool, ok := rdbs.Load(opt.Addr); ok {
			return pool.(*sync.Pool)
		} else {
			pool = &sync.Pool{
				New: func() interface{} {
					log.Println("[Redis]", "connecting to", opt.Addr)
					return redis.NewClient(opt)
				},
			}
			rdbs.Store(opt.Addr, pool)
			return pool.(*sync.Pool)
		}
	}
	return nil
}

func (m *localRedisModule) Open() lua.LGFunction {
	return func(l *lua.LState) int {
		functions := map[string]lua.LGFunction{
			"exists":       m.exists,
			"ping":         m.ping,
			"get":          m.get,
			"set":          m.set,
			"incr":         m.incr,
			"incrby":       m.incrby,
			"hincrby":      m.hincrby,
			"hkeys":        m.hkeys,
			"hgetall":      m.hgetall,
			"hget":         m.hget,
			"hset":         m.hset,
			"hmset":        m.hmset,
			"publish":      m.publish,
			"subscribe":    m.subscribe,
			"psubscribe":   m.psubscribe,
			"unsubscribe":  m.unsubscribe,
			"punsubscribe": m.punsubscribe,
		}
		l.Push(l.SetFuncs(l.CreateTable(0, len(functions)), functions))
		return 1
	}
}

func (m *localRedisModule) validate() error {
	if m.pool == nil {
		return errors.New("no connection")
	}
	return nil
}

func encode(v interface{}) interface{} {
	switch value := v.(type) {
	case map[string]interface{}:
		v, _ = json.Marshal(value)
	}
	return v
}

func decode(l *lua.LState, v interface{}) lua.LValue {
	switch value := v.(type) {
	case []byte:
		json.Unmarshal(value, &v)
		v = luaconv.Value(l, v)
	case string:
		json.Unmarshal([]byte(value), &v)
		v = luaconv.Value(l, v)
	default:
		v = luaconv.Value(l, v)
	}
	return v.(lua.LValue)
}

func (m *localRedisModule) ping(l *lua.LState) int {
	if err := m.validate(); err != nil {
		return 0
	}
	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	result, err := rdb.Ping(l.Context()).Result()
	if err != nil {
		return 0
	}
	l.Push(lua.LString(result))
	return 1
}

func (m *localRedisModule) exists(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "exists"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	val, err := rdb.Exists(l.Context(), key).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "exists"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	l.Push([]lua.LValue{lua.LFalse, lua.LTrue}[val])
	return 1
}

func (m *localRedisModule) get(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "get"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	val, err := rdb.Get(l.Context(), key).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "get"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	l.Push(decode(l, val))
	return 1
}

func (m *localRedisModule) set(l *lua.LState) int {
	key := l.CheckString(1)
	value := luaconv.LuaValue(l.CheckAny(2))

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "set"),
			zap.String("key", key),
			zap.Any("value", encode(value)),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	err := rdb.Set(l.Context(), key, encode(value), 0).Err()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "set"),
			zap.String("key", key),
			zap.Any("value", encode(value)),
			zap.Error(err),
		)
		return 0
	}

	return 0
}

func (m *localRedisModule) incr(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects hash key")
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	value, err := rdb.Incr(l.Context(), key).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "incr"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	l.Push(lua.LNumber(value))
	return 1
}

func (m *localRedisModule) incrby(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects hash key")
		return 0
	}

	number := l.CheckNumber(2)

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	value, err := rdb.IncrBy(l.Context(), key, int64(number)).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "incrby"),
			zap.String("key", key),
			zap.Int64("number", int64(number)),
			zap.Error(err),
		)
		return 0
	}

	l.Push(lua.LNumber(value))
	return 1
}

func (m *localRedisModule) hincrby(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects hash key")
		return 0
	}

	field := l.CheckString(2)

	if len(field) == 0 {
		l.ArgError(2, "expects field key")
		return 0
	}

	number := l.CheckNumber(3)

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	value, err := rdb.HIncrBy(l.Context(), key, field, int64(number)).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "hincrby"),
			zap.String("key", key),
			zap.Int64("number", int64(number)),
			zap.Error(err),
		)
		return 0
	}

	l.Push(lua.LNumber(value))
	return 1
}

func (m *localRedisModule) hgetall(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects hash key")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "hgetall"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	val, err := rdb.HGetAll(l.Context(), key).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "hgetall"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	table := l.CreateTable(len(val), 0)
	for field, value := range val {
		table.RawSetString(field, decode(l, value))
	}

	l.Push(table)
	return 1
}

func (m *localRedisModule) hkeys(l *lua.LState) int {
	key := l.CheckString(1)

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "hkeys"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	val, err := rdb.HKeys(l.Context(), key).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "hkeys"),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0
	}

	table := l.CreateTable(len(val), 0)
	for i, field := range val {
		table.RawSetInt(i+1, lua.LString(field))
	}

	l.Push(table)
	return 1
}

func (m *localRedisModule) hget(l *lua.LState) int {
	key := l.CheckString(1)
	field := l.CheckString(2)

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	} else if len(field) == 0 {
		l.ArgError(2, "expects field")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "hget"),
			zap.String("key", key),
			zap.String("field", field),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	val, err := rdb.HGet(l.Context(), key, field).Result()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "hget"),
			zap.String("key", key),
			zap.String("field", field),
			zap.Error(err),
		)
		return 0
	}

	l.Push(decode(l, val))
	return 1
}

func (m *localRedisModule) hset(l *lua.LState) int {
	key := l.CheckString(1)
	field := l.CheckString(2)
	value := luaconv.LuaValue(l.CheckAny(3))

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	} else if len(field) == 0 {
		l.ArgError(2, "expects field")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "hset"),
			zap.String("key", key),
			zap.String("field", field),
			zap.Any("value", encode(value)),
			zap.Error(err),
		)
		return 0
	}

	data := map[string]interface{}{field: encode(value)}
	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	err := rdb.HSet(l.Context(), key, data).Err()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "hset"),
			zap.String("key", key),
			zap.String("field", field),
			zap.Any("value", encode(value)),
			zap.Error(err),
		)
		return 0
	}

	return 0
}

func (m *localRedisModule) hmset(l *lua.LState) int {
	key := l.CheckString(1)
	table := l.CheckTable(2)

	if len(key) == 0 {
		l.ArgError(1, "expects key")
		return 0
	}
	values := make([]interface{}, 0)
	table.ForEach(func(l1, l2 lua.LValue) {
		k := luaconv.LuaValue(l1)
		values = append(values, encode(k))
		v := luaconv.LuaValue(l2)
		values = append(values, encode(v))
	})

	handle := func(err error) {
		m.logger.Error("[Redis]",
			zap.String("command", "hmset"),
			zap.String("key", key),
			zap.Any("values", values),
			zap.Error(err),
		)
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	err := rdb.HMSet(l.Context(), key, values...).Err()
	if err != nil {
		handle(err)
		return 0
	}

	return 0
}

func (m *localRedisModule) publish(l *lua.LState) int {
	channel := l.CheckString(1)

	if len(channel) == 0 {
		l.ArgError(1, "expects channel")
		return 0
	}

	value := luaconv.LuaValue(l.CheckAny(2))

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "publish"),
			zap.String("channel", channel),
			zap.Any("value", encode(value)),
			zap.Error(err),
		)
		return 0
	}

	rdb := m.pool.Get().(*redis.Client)
	defer m.pool.Put(rdb)
	err := rdb.Publish(l.Context(), channel, encode(value)).Err()
	if err != nil {
		m.logger.Error("execution",
			zap.String("command", "publish"),
			zap.String("channel", channel),
			zap.Any("value", encode(value)),
			zap.Error(err),
		)
		return 0
	}
	return 0
}

func (m *localRedisModule) subscribe(l *lua.LState) int {
	channel := l.CheckString(1)
	fn := l.CheckFunction(2)

	if len(channel) == 0 {
		l.ArgError(1, "expects channel")
		return 0
	} else if fn == nil {
		l.ArgError(2, "expects function")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "subscribe"),
			zap.String("channel", channel),
			zap.Error(err),
		)
		return 0
	}

	if sub, ok := m.pubsub[channel]; ok {
		sub.Append(fn)
	} else {
		rdb := m.pool.Get().(*redis.Client)
		defer m.pool.Put(rdb)
		sub := rdb.Subscribe(l.Context(), channel)
		if sub == nil {
			m.logger.Error("execution",
				zap.String("command", "subscribe"),
				zap.String("channel", channel),
				zap.Error(fmt.Errorf("unknown")),
			)
			return 0
		}
		ctx, ctxCancelFn := context.WithCancel(l.Context())
		subscriber := &localRedisSubscriber{
			vm:          l,
			PubSub:      sub,
			ctx:         ctx,
			ctxCancelFn: ctxCancelFn,
			runtime:     m.runtime,
			functions:   []*lua.LFunction{fn},
		}
		go subscriber.Startup()
		m.pubsub[channel] = subscriber
	}
	return 0
}

func (m *localRedisModule) unsubscribe(l *lua.LState) int {
	channel := l.CheckString(1)

	if len(channel) == 0 {
		l.ArgError(1, "expects channel")
		return 0
	}
	var ok = false
	var sub *localRedisSubscriber
	if sub, ok = m.pubsub[channel]; !ok {
		return 0
	}
	delete(m.pubsub, channel)

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "unsubscribe"),
			zap.String("channel", channel),
			zap.Error(err),
		)
		return 0
	}

	if err := sub.Unsubscribe(l.Context(), channel); err != nil {
		m.logger.Error("execution",
			zap.String("command", "unsubscribe"),
			zap.String("channel", channel),
			zap.Error(err),
		)
		return 0
	}

	if err := sub.Close(); err != nil {
		m.logger.Error("close",
			zap.String("command", "unsubscribe"),
			zap.String("channel", channel),
			zap.Error(err),
		)
		return 0
	}

	return 0
}

func (m *localRedisModule) psubscribe(l *lua.LState) int {
	pattern := l.CheckString(1)
	fn := l.CheckFunction(2)

	if len(pattern) == 0 {
		l.ArgError(1, "expects pattern")
		return 0
	} else if fn == nil {
		l.ArgError(2, "expects function")
		return 0
	}

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "psubscribe"),
			zap.String("pattern", pattern),
			zap.Error(err),
		)
		return 0
	}

	if sub, ok := m.pubsub[pattern]; ok {
		sub.Append(fn)
	} else {
		rdb := m.pool.Get().(*redis.Client)
		defer m.pool.Put(rdb)
		sub := rdb.PSubscribe(l.Context(), pattern)
		if sub == nil {
			m.logger.Error("execution",
				zap.String("command", "subscribe"),
				zap.String("pattern", pattern),
				zap.Error(fmt.Errorf("unknown")),
			)
			return 0
		}
		ctx, ctxCancelFn := context.WithCancel(l.Context())
		subscriber := &localRedisSubscriber{
			vm:          l,
			PubSub:      sub,
			ctx:         ctx,
			ctxCancelFn: ctxCancelFn,
			runtime:     m.runtime,
			functions:   []*lua.LFunction{fn},
		}
		go subscriber.Startup()
		m.pubsub[pattern] = subscriber
	}

	return 0
}

func (m *localRedisModule) punsubscribe(l *lua.LState) int {
	pattern := l.CheckString(1)

	if len(pattern) == 0 {
		l.ArgError(1, "expects pattern")
		return 0
	}

	var ok = false
	var sub *localRedisSubscriber
	if sub, ok = m.pubsub[pattern]; !ok {
		return 0
	}
	delete(m.pubsub, pattern)

	if err := m.validate(); err != nil {
		m.logger.Error("invalid",
			zap.String("command", "punsubscribe"),
			zap.String("pattern", pattern),
			zap.Error(err),
		)
		return 0
	}

	if err := sub.PUnsubscribe(l.Context(), pattern); err != nil {
		m.logger.Error("execution",
			zap.String("command", "punsubscribe"),
			zap.String("pattern", pattern),
			zap.Error(err),
		)
		return 0
	}

	if err := sub.Close(); err != nil {
		m.logger.Error("close",
			zap.String("command", "punsubscribe"),
			zap.String("pattern", pattern),
			zap.Error(err),
		)
		return 0
	}

	return 0
}

type pubSubEvent struct {
	StateEvent
	Func      *lua.LFunction
	Arguments []lua.LValue
}

func (e *pubSubEvent) Update(elapse time.Duration, l *lua.LState) error {
	switch EventState(e.Load()) {
	case EVENT_STATE_INITIALIZE:
		l.Push(e.Func)
		defer e.Store(uint32(EVENT_STATE_COMPLETE))
		for _, argument := range e.Arguments {
			l.Push(argument)
		}
		if err := l.PCall(len(e.Arguments), 0, nil); err != nil {
			return err
		}
	}
	return nil
}
