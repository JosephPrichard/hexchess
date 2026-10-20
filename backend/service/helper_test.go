package service

import (
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"
)

func assertRedisChess(t *testing.T, redis cache.Redis, wantState *model.ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := t.Context()

	if wantState == nil {
		return
	}
	gameKey := cache.FmtGameKey(wantState.ID)

	bytes, err := redis.PrimaryClient.Get(ctx, gameKey).Bytes()
	if err != nil {
		t.Fatalf("failed to retrieve redis chess state: %v", err)
	}
	actualState, err := model.UnmarshalChessState(bytes)
	if err != nil {
		t.Fatalf("failed to unmarshal redis chess state: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}

func mutateGame(state *model.ChessState, fn func(s *model.ChessState)) *model.ChessState {
	s := state.DeepCopy()
	fn(&s)
	return &s
}

func setChessStates(t *testing.T, rdb cache.Redis, states ...*model.ChessState) {
	t.Helper()

	for _, state := range states {
		bytes, err := model.MarshalChessState(state)
		if err != nil {
			t.Fatalf("failed to marshal redis chess state: %v", err)
		}

		id := state.ID
		gameKey := cache.FmtGameKey(id)
		if err := rdb.PrimaryClient.Set(t.Context(), gameKey, bytes, 0).Err(); err != nil {
			t.Fatalf("failed to set redis chess state: %v", err)
		}

		gameTimersKey := cache.FmtGameTimersZSet(id.Partition())
		gameTimersValue := redis.Z{Score: float64(time.Now().UnixMilli()), Member: id.String()}
		if err := rdb.PrimaryClient.ZAdd(t.Context(), gameTimersKey, gameTimersValue).Err(); err != nil {
			t.Fatalf("failed to set redis chess timer: %v", err)
		}
	}
}
