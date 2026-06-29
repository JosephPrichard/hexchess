package main

import (
	_ "xk6-hexchess/websocket"

	k6cmd "go.k6.io/k6/v2/cmd"
)

func main() {
	k6cmd.Execute()
}
