//go:build zmq
// +build zmq

package runtimelua

import "go.uber.org/zap"

// ZMQ Require extra library dependency.
func WithModuleZmq(logger *zap.Logger) Option {
	return newOption(func(r *Runtime) {
		mod := &localZmqModule{
			localRuntimeModule: localRuntimeModule{
				localModule: localModule{
					name: "zmq",
				},
				runtime: r,
			},
			logger: logger,
		}
		r.preloads[mod.Name()] = mod
	})
}
