package auxlib

import (
	"crypto/md5"
	"encoding/hex"

	lua "github.com/yuin/gopher-lua"
)

var (
	MD5LibName = "base64"
)

var md5Funcs = map[string]lua.LGFunction{
	"sum": md5Sum,
}

func OpenMD5(l *lua.LState) int {
	mod := l.RegisterModule(MD5LibName, md5Funcs)
	l.Push(mod)
	return 1
}

func md5Sum(l *lua.LState) int {
	ctx := l.CheckString(1)
	hash := md5.Sum([]byte(ctx))
	l.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}
