package svc

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/app/chess"
	"math"
	"testing"
	"time"
)

func assertStatesEqual(t *testing.T, expState ChessState, actualState ChessState) {
	// empty fields we do not want to assert
	expState.Touch = time.Time{}
	actualState.Touch = time.Time{}
	assert.Equal(t, expState, actualState)
}

func assertChessStateRdb(t *testing.T, rdb *redis.Client, expState ChessState) {
	ctx := context.WithValue(context.Background(), TraceKey, "assert-chess-state")
	rdbState, err := GetChessState(ctx, rdb, "abc123")
	assert.NoError(t, err)
	assertStatesEqual(t, expState, rdbState)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	rdb := beforeRedisTests(t)

	ctx := context.WithValue(context.Background(), TraceKey, "test-join-game")

	state := MakeStartChessState("abc123", RealTime)
	state.FirstColor = White

	_, err := SetChessState(ctx, rdb, "abc123", state)
	assert.NoError(t, err)

	player := PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	updated, err := JoinGame(ctx, rdb, "abc123", player)
	assert.NotNil(t, updated)
	assert.NoError(t, err)
	assert.Equal(t, player, *updated.WhitePlayer)

	assertChessStateRdb(t, rdb, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	rdb := beforeRedisTests(t)

	ctx := context.WithValue(context.Background(), TraceKey, "test-join-game-both-players")

	state := MakeStartChessState("abc123", RealTime)
	state.WhitePlayer = &PlayerState{ID: 1, Name: "white"}
	state.BlackPlayer = &PlayerState{ID: 2, Name: "black"}

	_, err := SetChessState(ctx, rdb, "abc123", state)
	assert.NoError(t, err)

	result, err := JoinGame(ctx, rdb, "abc123", PlayerState{ID: 3, Name: "extra"})
	assert.NoError(t, err)

	assertStatesEqual(t, state, result)
	assertChessStateRdb(t, rdb, result)
}

func TestMakeMove(t *testing.T) {
	dbs, closer := beforeDatabaseTests(t)
	defer closer()

	state := MakeStartChessState("abc123", RealTime)
	state.WhitePlayer = &PlayerState{ID: 1}
	state.BlackPlayer = &PlayerState{ID: 2}

	type Test struct {
		pm     chess.PieceMove
		player PlayerState
		expErr error
	}

	for i, test := range []Test{
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid turn
			player: *state.BlackPlayer,
			expErr: ErrTurn,
		},
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid move
			player: *state.WhitePlayer,
			expErr: ErrInvalidMove,
		},
		{
			pm:     chess.PieceMove{Piece: chess.WhitePawn, From: chess.Hex{File: 1, Rank: 0}, To: chess.Hex{File: 1, Rank: 1}}, // valid move
			player: *state.WhitePlayer,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx := context.WithValue(context.Background(), TraceKey, "test-make-move")

			_, err := SetChessState(ctx, dbs.Rdb, "abc123", state)
			assert.NoError(t, err)

			result, err := MakeGameMove(ctx, dbs, "abc123", test.player, test.pm)
			if err == nil {
				assert.Equal(t, test.pm, result.Move)
			} else {
				assert.Equal(t, test.expErr, err)
			}
		})
	}
}

func TestHandleFinishGame(t *testing.T) {
	dbs, closer := beforeDatabaseTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-finish-game")

	state := MakeStartChessState("abc123", RealTime)
	state.WhitePlayer = &PlayerState{ID: 1}
	state.BlackPlayer = &PlayerState{ID: 2}
	state.MoveList = []chess.PieceMove{{Piece: 1, To: chess.Hex{Rank: 1}}}

	createTestUsers(t, dbs.PgDB.Q)

	assert.NoError(t, handleFinishGame(ctx, dbs, state, true, Checkmate))
}

func TestForfeit_BlackForfeits(t *testing.T) {
	dbs, closer := beforeDatabaseTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-forfeit")

	state := MakeStartChessState("abc123", RealTime)
	state.WhitePlayer = &PlayerState{ID: 1}
	state.BlackPlayer = &PlayerState{ID: 2}

	createTestUsers(t, dbs.PgDB.Q)

	_, err := SetChessState(ctx, dbs.Rdb, "abc123", state)
	assert.NoError(t, err)

	assert.NoError(t, ForfeitGame(ctx, dbs, "abc123", *state.BlackPlayer))

	state.IsEnded = true
	assertChessStateRdb(t, dbs.Rdb, state)
}

func TestFinishGameTx(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-update-stats")
	createTestUsers(t, pgDB.Q)

	cs, err := FinishGameTx(ctx, pgDB, FinishGameParams{WhiteID: 1, BlackID: 2, Cause: Checkmate, IsWhiteWin: true, MoveList: nil})
	assert.NoError(t, err)
	u1, err := GetUserById(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, pgDB.Q, 2)
	assert.NoError(t, err)
	r1, err := GetReplay(ctx, pgDB.Q, 1)
	assert.NoError(t, err)

	// assert a value relatively close to the actual value
	cs.WinEloDiff = math.Round(cs.WinEloDiff)
	cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
	u1.Elo = math.Round(u1.Elo)
	u2.Elo = math.Round(u2.Elo)

	expChange := FinishGameChangeSet{WinID: 1, LoseID: 2, WinEloDiff: 15, LoseEloDiff: -15}
	assert.Equal(t, expChange, cs)

	assert.Equal(t, float64(1015), u1.Elo)
	assert.Equal(t, float64(985), u2.Elo)

	expReplay := ReplayEntity{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinElo:       15,
		LoseElo:      -15,
		WhiteElo:     1015,
		BlackElo:     985,
		PlayedOn:     time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, expReplay, r1)
}
