package gameplay

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/model"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/utils/serrors"
	"time"

	"github.com/redis/go-redis/v9"
)

type GameCreateService struct {
	redis      cache.Redis
	chessState ChessSetter
}

type ChessSetter interface {
	SetChessStatePiped(ctx context.Context, setter gamestate.RedisChessSetter, id model.GameID, state *model.ChessState, updtTime time.Time) error
}

func NewGameCreateService(redis cache.Redis, chessState ChessSetter) *GameCreateService {
	return &GameCreateService{redis: redis, chessState: chessState}
}

func (services *GameCreateService) CreateGame(ctx context.Context, color model.GameColor, mode model.GameMode, initialBoard *chess.Board) (model.GameID, error) {
	gameID := model.NewGameID()
	err := services.SetupGame(ctx, model.StateSetup{ID: gameID, Mode: mode, FirstColor: color, InitialBoard: initialBoard})
	return gameID, err
}

func (services *GameCreateService) SetupGame(ctx context.Context, setup model.StateSetup) error {
	gameID := setup.ID

	state := model.NewChessState(setup)
	state.Game.InitPieceMoves()

	pipeliner := func(pipe redis.Pipeliner) error {
		now := time.Now()
		if err := services.chessState.SetChessStatePiped(ctx, pipe, gameID, state, now); err != nil {
			return serrors.New("set chess state", err, "gameID", gameID)
		}
		return produceUpdtGameMetadata(ctx, pipe, state)
	}

	_, err := services.redis.PrimaryClient.TxPipelined(ctx, pipeliner)
	return err
}
