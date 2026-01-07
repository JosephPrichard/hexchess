//go:build wasm
package main

import (
	"syscall/js"
	"hexchess-svc/out"
)

func main() {
	wasm.RegisterChessModule()
	select {}
}
