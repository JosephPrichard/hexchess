package service

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/opt"
)

func produceUpdtGameMetadata(ctx context.Context, pipe producers.RedisXAdder, state *model.ChessState) error {
	return producers.ProduceUpdtGameMetadata(ctx, pipe, model.UpdtGameMetadataEvent{
		GameID:      state.ID,
		WhitePlayer: opt.Option[int64]{Value: state.WhitePlayer.ID, Present: state.WhitePlayer.Present},
		BlackPlayer: opt.Option[int64]{Value: state.BlackPlayer.ID, Present: state.BlackPlayer.Present},
		Mode:        state.Mode,
	})
}
