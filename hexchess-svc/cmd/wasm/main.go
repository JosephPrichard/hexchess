// go:build wasm
package main

import (
	"syscall/js"
	"hexchess-svc/out"
)

func main() {
	global := js.Global()
	out.RegisterWasmModule(global)
	select {}
}
