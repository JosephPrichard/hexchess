//go:build wasm
package main

import "hexchess-svc/wasm"

func main() {
	wasm.RegisterChessModule()
	select {}
}
