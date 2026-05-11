//go:build browser
package main

import "hexchess-svc/browser"

var version = "test"

func main() {
	browser.RegisterChessModule(version)
	select {}
}
