package runtimelua

import (
	"context"

	"github.com/deflinhec/runtimelua/module"
	"go.uber.org/zap"
)

type Option interface {
	apply(s *Runtime)
}

type funcOption struct {
	f func(*Runtime)
}

func (fdo *funcOption) apply(do *Runtime) {
	fdo.f(do)
}

func newOption(f func(*Runtime)) *funcOption {
	return &funcOption{
		f: f,
	}
}

func WithLogger(logger *zap.Logger) Option {
	return newOption(func(r *Runtime) {
		r.logger = logger
	})
}

func WithContext(ctx context.Context) Option {
	return newOption(func(r *Runtime) {
		r.ctx, r.ctxCancelFn = context.WithCancel(ctx)
	})
}

func WithModuleRedis(config module.RedisConfig) Option {
	return newOption(func(r *Runtime) {
		mod := module.RedisModule(r, config)
		r.preloads[mod.Name()] = mod
	})
}

func WithModuleEvent() Option {
	return newOption(func(r *Runtime) {
		mod := module.EventModule(r)
		r.preloads[mod.Name()] = mod
	})
}

func WithModuleLogger() Option {
	return newOption(func(r *Runtime) {
		mod := module.LoggerModule(r)
		r.preloads[mod.Name()] = mod
	})
}

func WithModuleHttp() Option {
	return newOption(func(r *Runtime) {
		mod := module.HttpModule(r)
		r.preloads[mod.Name()] = mod
	})
}

func WithModule(mod Module) Option {
	return newOption(func(r *Runtime) {
		r.preloads[mod.Name()] = mod
	})
}

func WithRuntimeModule(mod RuntimeModule) Option {
	return newOption(func(r *Runtime) {
		mod.Initialize(r.eventQueue, r.vm)
		r.preloads[mod.Name()] = mod
	})
}
