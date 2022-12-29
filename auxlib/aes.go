package auxlib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

var (
	Ase256LibName = "ase256"
)

var ase256Funcs = map[string]lua.LGFunction{
	"encrypt": ase256Encrypt,
	"decrypt": aes256Decrypt,
}

func OpenAes256(l *lua.LState) int {
	mod := l.RegisterModule(Ase256LibName, ase256Funcs)
	l.Push(mod)
	return 1
}

func ase256Encrypt(l *lua.LState) int {
	return aesEncrypt(l, 32)
}

func aes256Decrypt(l *lua.LState) int {
	return aesDecrypt(l, 32)
}

var (
	Ase128LibName = "ase128"
)

var ase128Funcs = map[string]lua.LGFunction{
	"encrypt": ase128Encrypt,
	"decrypt": ase128Decrypt,
}

func OpenAes128(l *lua.LState) int {
	mod := l.RegisterModule(Ase128LibName, ase128Funcs)
	l.Push(mod)
	return 1
}

func ase128Encrypt(l *lua.LState) int {
	return aesEncrypt(l, 16)
}

func ase128Decrypt(l *lua.LState) int {
	return aesDecrypt(l, 16)
}

// Not annotated as not exported and available in the Lua runtime
func aesEncrypt(l *lua.LState, keySize int) int {
	input := l.CheckString(1)
	if input == "" {
		l.ArgError(1, "expects string")
		return 0
	}
	key := l.CheckString(2)
	if len(key) != keySize {
		l.ArgError(2, fmt.Sprintf("expects key %v bytes long", keySize))
		return 0
	}

	// Pad string up to length multiple of 4 if needed.
	if maybePad := len(input) % 4; maybePad != 0 {
		input += strings.Repeat(" ", 4-maybePad)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		l.RaiseError("error creating cipher block: %v", err.Error())
		return 0
	}

	cipherText := make([]byte, aes.BlockSize+len(input))
	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		l.RaiseError("error getting iv: %v", err.Error())
		return 0
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], []byte(input))

	l.Push(lua.LString(cipherText))
	return 1
}

// Not annotated as not exported and available in the Lua runtime
func aesDecrypt(l *lua.LState, keySize int) int {
	input := l.CheckString(1)
	if input == "" {
		l.ArgError(1, "expects string")
		return 0
	}
	key := l.CheckString(2)
	if len(key) != keySize {
		l.ArgError(2, fmt.Sprintf("expects key %v bytes long", keySize))
		return 0
	}

	if len(input) < aes.BlockSize {
		l.RaiseError("input too short")
		return 0
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		l.RaiseError("error creating cipher block: %v", err.Error())
		return 0
	}

	cipherText := []byte(input)
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(cipherText, cipherText)

	l.Push(lua.LString(cipherText))
	return 1
}
