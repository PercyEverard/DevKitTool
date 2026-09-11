package main

import _ "embed"

// pack 命令会把完整环境写进 embed/ 目录，再调用 go build 重新编译本程序，
// 得到的就是一个自包含的可迁移单文件工具。

//go:embed embed/payload.tar.gz
var payloadGz []byte

//go:embed embed/manifest.json
var manifestJSON []byte

// payloadReady 判断当前二进制里是否已经有真正的打包内容（而不是占位文件）。
func payloadReady() bool {
	return len(payloadGz) > 0
}
