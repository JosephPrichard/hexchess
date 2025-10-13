package dal

import "context"

//go:generate mockgen -source gameplay.go -destination gameplay_mock.go -package dal GameplayDAL
type GameplayDAL interface {
	GetChessStateCount(ctx context.Context) (int64, error)
	GetChessState(ctx context.Context, id string) (ChessState, error)
	SetChessState(ctx context.Context, id string, state ChessState) (ChessState, error)
	UpdateGameResult(ctx context.Context, params GRParams) (GRChangeSet, error)
	UpdateLeaderboard(ctx context.Context, csList ...IncrLbChangeSet) error
}

type GameplayDao struct {
	Stores
}

func (s *GameplayDao) GetChessStateCount(ctx context.Context) (int64, error) {
	return GetChessStateCount(ctx, s.Rdb)
}

func (s *GameplayDao) GetChessState(ctx context.Context, id string) (ChessState, error) {
	return GetChessState(ctx, s.Rdb, id)
}

func (s *GameplayDao) SetChessState(ctx context.Context, id string, state ChessState) (ChessState, error) {
	return SetChessState(ctx, s.Rdb, id, state)
}

func (s *GameplayDao) UpdateGameResult(ctx context.Context, params GRParams) (GRChangeSet, error) {
	return UpdateGameResultTx(ctx, s.PgDB, params)
}

func (s *GameplayDao) UpdateLeaderboard(ctx context.Context, csList ...IncrLbChangeSet) error {
	return IncrLeaderboard(ctx, s.Rdb, csList...)
}
