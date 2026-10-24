package main

import (
	"encoding/base64"
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

type secretBlob struct {
	Size uint32
	Data *byte
}

var secretCrypt = syscall.NewLazyDLL("crypt32.dll")
var secretKernel = syscall.NewLazyDLL("kernel32.dll")

func transformSecret(input []byte, encrypt bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("密钥内容为空")
	}
	in := secretBlob{uint32(len(input)), &input[0]}
	var out secretBlob
	name := "CryptUnprotectData"
	if encrypt {
		name = "CryptProtectData"
	}
	ok, _, _ := secretCrypt.NewProc(name).Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(input)
	if ok == 0 {
		return nil, errors.New("Windows 无法读取或保护密钥，请在当前账号下重新填写")
	}
	defer secretKernel.NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func sealAPIKey(key string) (string, error) {
	b, e := transformSecret([]byte(key), true)
	return base64.StdEncoding.EncodeToString(b), e
}
func unsealAPIKey(sealed string) (string, error) {
	b, e := base64.StdEncoding.DecodeString(sealed)
	if e != nil {
		return "", errors.New("密钥配置无法读取，请重新填写")
	}
	b, e = transformSecret(b, false)
	return string(b), e
}
