//go:build zmq
// +build zmq

package runtimelua

import "github.com/deflinhec/runtimelua/module"

// ZMQ Require extra library dependency.
func WithModuleZmq() Option {
	return newOption(func(r *Runtime) {
		mod := module.ZMQModule(r)
		r.preloads[mod.Name()] = mod
	})
}
