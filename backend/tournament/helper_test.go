package tournament

import (
	"hexchess-svc/cache"
	"hexchess-svc/game"
	"hexchess-svc/pubsub"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func assertRedisChess(t *testing.T, redis cache.Redis, wantState *game.ChessState, options ...cmp.Option) {
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
	actualState, err := game.UnmarshalChessState(bytes)
	if err != nil {
		t.Fatalf("failed to unmarshal redis chess state: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}

func ExpectBroadcastTournamentOutput(t *testing.T, rdb cache.Redis, tournamentKey string, wantOutputs []TournamentOutput) func() {
	cmpOptsFullMatch := cmpopts.IgnoreFields(FullMatch{}, "GameID", "CreatedOn")

	return pubsub.ExpectBroadcast(
		t, rdb, wantOutputs,
		func(lb *subscribers.LocalBroadcasters, ch chan []byte) { lb.Tournament.Subscribe(tournamentKey, ch) },
		func(bytes []byte) TournamentOutput {
			var o TournamentOutput
			require.NoError(t, sonic.Unmarshal(bytes, &o))
			return o
		},
		cmpOptsFullMatch,
	)
}
