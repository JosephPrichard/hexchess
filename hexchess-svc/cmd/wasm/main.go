//go:build wasm
package main

import "hexchess-svc/wasm"

var version = "test"

func main() {
	wasm.RegisterChessModule(version)
	select {}
}
