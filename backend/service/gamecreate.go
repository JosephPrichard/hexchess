package service

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"time"

	"github.com/redis/go-redis/v9"
)

type GameCreateService struct {
	// infra
	redis cache.Redis

	// service
	chessState ChessSetter
}

type ChessSetter interface {
	SetChessStatePiped(ctx context.Context, setter RedisChessSetter, id model.GameID, state *model.ChessState, updtTime time.Time) error
}

func NewGameCreateService(redis cache.Redis, chessState ChessSetter) *GameCreateService {
	return &GameCreateService{redis: redis, chessState: chessState}
}

func (service *GameCreateService) CreateGame(ctx context.Context, args model.ChessStateArgs) (gameID model.GameID, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	if args.ID == "" {
		args.ID = model.NewGameID()
	}

	state := model.NewChessState(args)
	state.Game.InitPieceMoves()

	pipeliner := func(pipe redis.Pipeliner) error {
		now := time.Now()
		if err := service.chessState.SetChessStatePiped(ctx, pipe, args.ID, state, now); err != nil {
			return serrors.New("set chess state", err, "gameID", args.ID)
		}
		return produceUpdtGameMetadata(ctx, pipe, state)
	}

	_, err = service.redis.PrimaryClient.TxPipelined(ctx, pipeliner)
	return args.ID, err
}
