package svc

import "context"

func StartStreamReaders(ctx context.Context, svc *Services) {
	gameFinishStreamer := &GameFinishStreamer{
		Context:     ctx,
		Services:    svc,
		Concurrency: 8,
	}
	go gameFinishStreamer.ReadGameFinishEvents()
}
