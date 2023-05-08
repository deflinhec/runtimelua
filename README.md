# Lua runtime Go framework

> This project implement various optional modules for use within lua runtime.

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

* **script hot reload** - Support for lua hot reload when files changed.

## Getting Started

A quick demostration of this framework can be found under exmample directory. 

[main.go](./example/main.go) and it's [main.lua](./example/script/main.lua).

```shell
cd example && go run main.go
```

When launched you'll see each runtime informations in console output.

> 2023-05-08T14:54:20.449+0200    INFO    runtimelua/scripts.go:364       Watching runtime directory      {"vm": 0, "path": "/Users/user/runtimelua/example/script"}
> 2023-05-08T14:54:20.450+0200    DEBUG   runtimelua/runtime.go:120       Runtime information     {"vm": 0, "load": ["table", "os", "io", "string", "math", "coroutine", "package"]}
> 2023-05-08T14:54:20.450+0200    DEBUG   runtimelua/runtime.go:123       Runtime information     {"vm": 0, "preload": ["runtime", "json", "bit32", "base64", "event", "logger"]}
> 2023-05-08T14:54:20.451+0200    INFO    runtimelua/scripts.go:364       Watching runtime directory      {"vm": 1, "path": "/Users/user/runtimelua/example/script"}
> 2023-05-08T14:54:20.451+0200    DEBUG   runtimelua/runtime.go:120       Runtime information     {"vm": 1, "load": ["math", "coroutine", "package", "table", "os", "io", "string"]}
> 2023-05-08T14:54:20.451+0200    DEBUG   runtimelua/runtime.go:123       Runtime information     {"vm": 1, "preload": ["runtime", "json", "bit32", "base64", "event", "logger"]}

## Building with ZeroMQ support

ZeroMQ require extra libraries dependency, so it's optional.

```shell
go build -tags zmq
```

## Contribute

Contributions are always welcome.

## License

This project is licensed under the [Apache-2 License](./LICENSE).

## :coffee: Donation

If you like my project and also appericate for the effort. Don't hesitate to [buy me a coffee](https://ko-fi.com/deflinhec)😊.