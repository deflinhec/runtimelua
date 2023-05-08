# Lua runtime Go framework

> This project implemnt various optional modules for use within lua runtime.

## Features

* **aes256** - AES-256 encryption and decryption.
* **base64** - Base64 encoding and decoding.
* **json** - JSON encoding and decoding.
* **md5** - MD5 hash funtion.
* **uuid** - UUID generator.
* **bit32** - 32 bit shifting.
* **bit64** - 64 bit shifting.

* **event** - Event module for subscribing loop、delay events.
* **http** - HTTP module for sending Http Request.
* **logger** - Logger module for logging.
* **redis** - Redis module to comminucate with redis sever, support pub/sub.
* **zmq** (Optional build) - ZeroMQ module to create router and dealers.

## Getting Started

```go
package main

import (
    "github.com/deflinhec/runtimelua"
)

const (
    Parallel = 8
)

func init() {
    lua.LuaLDir = "script"
}

func main() {
    logger, _ := zap.NewDevelopment()
    ctx, ctxCancelFn := context.WithCancel(context.Background())
	defer ctxCancelFn()
    for i := 0; i < Parallel; i++ {
        runtimelua.NewRuntime(scripts,
            runtimelua.WithLibJson(),
            runtimelua.WithLibBit32(),
            runtimelua.WithLibBase64(),
            runtimelua.WithContext(ctx),
            runtimelua.WithLogger(logger),
            runtimelua.WithModuleEvent(logger),
            runtimelua.WithModuleLogger(logger),
            runtimelua.WithGlobal("GLOBAL_VARS", []string{"var1","var2"}),
        ).Startup()
    }
    <-ctx.Done()
}
```