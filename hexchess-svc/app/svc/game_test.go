package svc

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
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

func assertChessStateRdb(t *testing.T, rdb *redis.Pool, expState ChessState) {
	ctx := context.WithValue(context.Background(), TraceKey, "assert-chess-state")

	state, err := GetChessState(ctx, rdb, expState.ID)
	assert.NoError(t, err)
	assertStatesEqual(t, expState, state)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-join-game")

	stateID := "test123" + uuid.NewString()

	state := MakeStartChessState(stateID, RealTime)
	state.FirstColor = White

	_, err := SetChessState(ctx, rdb, stateID, state)
	assert.NoError(t, err)

	player := PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	stores := &GameStores{Stores: Stores{Rdb: rdb}}

	updated, err := JoinGame(ctx, stores, stateID, player)
	assert.NotNil(t, updated)
	assert.NoError(t, err)
	assert.Equal(t, player, *updated.WhitePlayer)

	assertChessStateRdb(t, rdb, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-join-game-both-players")

	stateID := "test123" + uuid.NewString()

	state := MakeStartChessState(stateID, RealTime)
	state.WhitePlayer = &PlayerState{ID: 1, Name: "white"}
	state.BlackPlayer = &PlayerState{ID: 2, Name: "black"}

	_, err := SetChessState(ctx, rdb, stateID, state)
	assert.NoError(t, err)

	stores := &GameStores{Stores: Stores{Rdb: rdb}}

	result, err := JoinGame(ctx, stores, stateID, PlayerState{ID: 3, Name: "extra"})
	assert.NoError(t, err)

	assertStatesEqual(t, state, result)
	assertChessStateRdb(t, rdb, result)
}

func TestMakeMove(t *testing.T) {
	stores, closer := beforeStoreTests(t)
	defer closer()

	stateID := "test123" + uuid.NewString()

	state := MakeStartChessState(stateID, RealTime)
	state.WhitePlayer = &PlayerState{ID: 1}
	state.BlackPlayer = &PlayerState{ID: 2}

	tests := []struct {
		pm     chess.PieceMove
		player PlayerState
		expErr error
	}{
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
	}

	gameStores := &GameStores{Stores: stores} // we aren't mocking here, since handleFinishGame won't be called.

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx := context.WithValue(context.Background(), TraceKey, "test-make-move")

			_, err := SetChessState(ctx, gameStores.Rdb, stateID, state)
			assert.NoError(t, err)

			result, err := MakeGameMove(ctx, gameStores, stateID, test.player, test.pm)
			if err == nil {
				assert.Equal(t, test.pm, result.Move)
			} else {
				assert.Equal(t, test.expErr, err)
			}
		})
	}
}

func TestHandleFinishGame(t *testing.T) {
	stores, closer := beforeStoreTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-finish-game")

	stateID := "test123" + uuid.NewString()

	state := MakeStartChessState(stateID, RealTime)
	state.WhitePlayer = &PlayerState{ID: 1}
	state.BlackPlayer = &PlayerState{ID: 2}
	state.MoveList = []chess.PieceMove{{Piece: 1, To: chess.Hex{Rank: 1}}}

	gameStores := &GameStores{Stores: stores}

	assert.NoError(t, handleFinishGame(ctx, gameStores, state, true, Checkmate))
}

func TestForfeit_BlackForfeits(t *testing.T) {
	stores, closer := beforeStoreTests(t)
	defer closer()

	stateID := "test123" + uuid.NewString()

	ctx := context.WithValue(context.Background(), TraceKey, "test-forfeit")

	state := MakeStartChessState(stateID, RealTime)
	state.WhitePlayer = &PlayerState{ID: 1}
	state.BlackPlayer = &PlayerState{ID: 2}

	_, err := SetChessState(ctx, stores.Rdb, stateID, state)
	assert.NoError(t, err)

	gameStores := &GameStores{Stores: stores}

	assert.NoError(t, ForfeitGame(ctx, gameStores, stateID, *state.BlackPlayer))

	state.IsEnded = true
	assertChessStateRdb(t, stores.Rdb, state)
}

func TestUpdateGameResultTx(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	testUsers := createTestUsers(t, pgDB,
		UserInst{Username: "user1-" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1000},
		UserInst{Username: "user2-" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1000})

	ctx := context.WithValue(context.Background(), TraceKey, "test-update-stats")

	cs, err := UpdateGameResultTx(ctx, pgDB, GRParams{WhiteID: testUsers[0].ID, BlackID: testUsers[1].ID, Cause: Checkmate, IsWhiteWin: true, MoveList: nil})
	assert.NoError(t, err)

	u1, err := GetUserById(ctx, pgDB.Q, testUsers[0].ID)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, pgDB.Q, testUsers[1].ID)
	assert.NoError(t, err)
	r1, err := GetReplay(ctx, pgDB.Q, cs.ReplayID)
	assert.NoError(t, err)

	// assert a value relatively close to the actual value
	cs.WinEloDiff = math.Round(cs.WinEloDiff)
	cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
	u1.Elo = math.Round(u1.Elo)
	u2.Elo = math.Round(u2.Elo)

	assert.Equal(t, float64(1015), u1.Elo)
	assert.Equal(t, float64(985), u2.Elo)

	expReplay := ReplayEntity{
		ID:           cs.ReplayID,
		WhiteID:      testUsers[0].ID,
		BlackID:      testUsers[1].ID,
		WhiteName:    testUsers[0].Username,
		BlackName:    testUsers[1].Username,
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

	cs.ReplayID = 0 // we can't assert this and don't care to, since replayID is autogenerated
	expChange := GRChangeSet{WinID: testUsers[0].ID, LoseID: testUsers[1].ID, WinEloDiff: 15, LoseEloDiff: -15}
	assert.Equal(t, expChange, cs)
}
