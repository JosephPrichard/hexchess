package game

import (
	"hexchess-svc/cache"
	"hexchess-svc/gameid"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func assertRedisChess(t *testing.T, redis cache.Redis, wantState *ChessState, options ...cmp.Option) {
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
	actualState, err := UnmarshalChessState(bytes)
	if err != nil {
		t.Fatalf("failed to unmarshal redis chess state: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}

func mutateGame(state *ChessState, fn func(s *ChessState)) *ChessState {
	s := state.DeepCopy()
	fn(&s)
	return &s
}

func setChessStates(t *testing.T, rdb cache.Redis, states ...*ChessState) {
	t.Helper()

	for _, state := range states {
		bytes, err := MarshalChessState(state)
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

func ExpectBroadcastGames(t *testing.T, rdb cache.Redis, gameID gameid.Identifier, wantOutputs []*pb.GameOutput) func() {
	cmpOptsGameOutputs := []cmp.Option{
		protocmp.Transform(),
		protocmp.IgnoreFields(&pb.ReplayOutput{}, "id", "played_on"),
	}

	return pubsub.ExpectBroadcast(
		t, rdb, wantOutputs,
		func(lb *subscribers.LocalBroadcasters, ch chan []byte) { lb.Games.Subscribe(gameID, ch) },
		func(bytes []byte) *pb.GameOutput {
			o := &pb.GameOutput{}
			require.NoError(t, proto.Unmarshal(bytes, o))
			return o
		},
		cmpOptsGameOutputs...,
	)
}
