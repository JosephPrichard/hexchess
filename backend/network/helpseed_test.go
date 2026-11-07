package network_test

import (
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/game"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/network"
	"hexchess-svc/player"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var TestSessionID1 = "testing-session-id-1"
var TestSessionID2 = "testing-session-id-2"
var TestGameID1 = gameid.NewGameID()

func createTestSessions(t *testing.T, redis cache.Redis) {
	t.Helper()
	ctx := t.Context()

	for _, session := range []struct {
		ID     string
		Player player.PlayerState
		Expiry time.Duration
	}{
		{ID: TestSessionID1, Player: player.NewPlayer(1, "user1", "us"), Expiry: network.SessionMaxAge},
		{ID: TestSessionID2, Player: player.NewPlayer(2, "user2", "us"), Expiry: network.SessionMaxAge},
	} {
		data, err := player.MarshalPlayer(session.Player)
		if err != nil {
			t.Fatalf("marshal session: %v", err)
		}
		sessionKey := fmt.Sprintf("session:{%s}", session.ID)
		if err := redis.PrimaryClient.SetEx(ctx, sessionKey, data, session.Expiry).Err(); err != nil {
			t.Fatalf("set session: %v", err)
		}
	}
}

func createTestChessStates(t *testing.T, redis cache.Redis) {
	t.Helper()

	var testStates = []*game.ChessState{
		game.NewChessState(game.ChessStateArgs{
			ID:         TestGameID1,
			Mode:       gamedata.ModeCorrespondence1,
			FirstColor: gamedata.Random,
			Black:      player.NewPlayer(2, "user2", "us"),
			UndoState:  game.UndoState{UndoID: 2},
		}),
		game.NewChessState(game.ChessStateArgs{ID: "game2", Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random}),
		game.NewChessState(game.ChessStateArgs{ID: "game3", Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random}),
	}
	ctx := t.Context()
	for _, state := range testStates {
		gameKey := fmt.Sprintf("game:%s{%c}", state.ID, state.ID.Partition())

		bytes, err := game.MarshalChessState(state)
		if err != nil {
			t.Fatalf("marshal chess state: %v", err)
		}
		if err := redis.PrimaryClient.Set(ctx, gameKey, bytes, 0).Err(); err != nil {
			t.Fatalf("set chess state: %v", err)
		}
	}
}

type updtLbChangeSet struct {
	Mode    gamedata.GameMode
	ID      int64
	EloDiff float64
}

func createLeaderboard(t *testing.T, rdb cache.Redis, changes ...updtLbChangeSet) {
	ctx := t.Context()
	pipe := rdb.PrimaryClient.Pipeline()
	for _, change := range changes {
		modeLbZSet := fmt.Sprintf("%s/mode:{%s}", cache.Constants.LeaderboardZSet, change.Mode.String())
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.EloDiff, Member: change.ID})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("failed to create leaderboard: %v", err)
	}
}
