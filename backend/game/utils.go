package game

import (
	"context"
	"hexchess-svc/queue"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/opt"
	"math"
)

func produceUpdtGameMetadata(ctx context.Context, pipe producers.RedisXAdder, state *ChessState) error {
	return producers.ProduceUpdtGameMetadata(ctx, pipe, queue.UpdtGameMetadataJob{
		GameID:      state.ID,
		WhitePlayer: opt.Option[int64]{Value: state.WhitePlayer.ID, Present: state.WhitePlayer.Present},
		BlackPlayer: opt.Option[int64]{Value: state.BlackPlayer.ID, Present: state.BlackPlayer.Present},
		Mode:        state.Mode.String(),
	})
}

func probabilityWins(elo1, elo2 float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (elo2-elo1)/400.0))
}
