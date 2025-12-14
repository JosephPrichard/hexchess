package svc

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func assertStateRdb(t *testing.T, rdb *db.Redis, expState ChessState) {
	ctx := context.WithValue(context.Background(), util.Trace, "assert-chess-states")
	actualState, err := GetChessState(ctx, rdb, expState.ID)
	if err != nil {
		t.Fatalf("get chess for assert: %v", err)
	}
	util.AssertEqualIgnoring(t, expState, actualState, ChessMetaCmpOpts)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-join-game")

	gameID := "test123"
	inState := MakeState(StateSetup{ID: gameID, TimeControl: TcRealTime, FirstColor: ColorRandom})
	inState.FirstColor = ColorWhite
	player := PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	// when
	_, err := SetChessState(ctx, rdb, gameID, inState)
	assert.NoError(t, err)

	updated, err := JoinGame(ctx, rdb, gameID, &player)
	assert.NoError(t, err)

	// then
	expState := inState.DeepCopy()
	expState.WhitePlayer = &player

	util.AssertEqualIgnoring(t, expState, updated, ChessMetaCmpOpts)
	assertStateRdb(t, rdb, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-join-game-both-players")

	gameID := "test123"
	inState := MakeState(StateSetup{ID: gameID, TimeControl: TcRealTime, FirstColor: ColorRandom, White: &PlayerState{ID: 1, Name: "white"}, Black: &PlayerState{ID: 2, Name: "black"}})

	// when
	_, err := SetChessState(ctx, rdb, gameID, inState)
	assert.NoError(t, err)

	result, err := JoinGame(ctx, rdb, gameID, &PlayerState{ID: 3, Name: "test"})
	assert.NoError(t, err)

	// then
	util.AssertEqualIgnoring(t, inState, result, ChessMetaCmpOpts)
	assertStateRdb(t, rdb, inState)
}

func TestMakeMove(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, InsertTestData)
	defer closer()

	s1 := MakeState(StateSetup{
		ID:          "test1",
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       &PlayerState{ID: 1},
		Black:       &PlayerState{ID: 2},
	})
	s2 := MakeState(StateSetup{
		ID:          "test2",
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       &PlayerState{ID: 3},
		Black:       &PlayerState{ID: 4},
		Game: util.PtrOf(chess.MakeEmptyGame(false,
			chess.NotMove{Not: "f1", Piece: chess.WhiteKing},
			chess.NotMove{Not: "a2", Piece: chess.BlackQueen},
			chess.NotMove{Not: "h1", Piece: chess.BlackRook},
			chess.NotMove{Not: "f3", Piece: chess.BlackRook},
			chess.NotMove{Not: "f9", Piece: chess.BlackKing},
		)),
	})

	ctx := context.WithValue(context.Background(), util.Trace, "testing-make-move")

	for _, state := range []ChessState{s1, s2} {
		state.Game.InitPieceMoves()
		if _, err := SetChessState(ctx, dbs.Rdb, state.ID, state); err != nil {
			t.Fatalf("failed initialize test state: %v", err)
		}
	}

	for i, test := range []struct {
		pm     chess.Move
		state  ChessState
		player PlayerState
		expErr error
	}{
		{
			pm:     chess.Move{To: chess.Hex{File: 1}}, // invalid turn
			state:  s1,
			player: *s1.BlackPlayer,
			expErr: ErrTurn,
		},
		{
			pm:     chess.Move{To: chess.Hex{File: 1}}, // invalid move
			state:  s1,
			player: *s1.WhitePlayer,
			expErr: ErrInvalidMove,
		},
		{
			pm:     chess.Move{Promotion: chess.QueenPromotion, From: chess.Hex{File: 1, Rank: 0}, To: chess.Hex{File: 1, Rank: 1}}, // valid move
			state:  s1,
			player: *s1.WhitePlayer,
		},
		{
			pm:     chess.Move{Promotion: chess.QueenPromotion, From: chess.Hex{File: 0, Rank: 1}, To: chess.Hex{File: 0, Rank: 0}}, // valid move
			state:  s2,
			player: *s2.BlackPlayer,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := MakeGameMove(ctx, &dbs, test.state.ID, test.player, test.pm)
			assert.Equal(t, test.expErr, err)
		})
	}
}

func TestForfeit_BlackForfeits(t *testing.T) {
	// given
	dbs, closer := db.BeforeDbTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-forfeit")

	gameID := "test123"
	inState := MakeState(StateSetup{ID: gameID, TimeControl: TcRealTime, FirstColor: ColorRandom, White: &PlayerState{ID: 1}, Black: &PlayerState{ID: 2}})
	inState.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	// when
	_, err := SetChessState(ctx, dbs.Rdb, gameID, inState)
	assert.NoError(t, err)
	assert.NoError(t, ForfeitGame(ctx, &dbs, gameID, *inState.BlackPlayer))

	// then
	expState := inState.DeepCopy()
	expState.IsEnded = true

	assertStateRdb(t, dbs.Rdb, expState)
}

func TestInsertGameResultTx(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-update-stats")

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]

	// when
	cs, err := InsertGameResultTx(ctx, pdb, time.Now(), GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Checkmate, ReplayResult: WhiteWin, ReplayMode: ModeUnlimited, SerializedMoveHist: []byte{}})
	assert.NoError(t, err)

	// then
	e1, err := pdb.Query.GetElosByIds(ctx, []int64{testUser0.ID})
	assert.NoError(t, err)
	e2, err := pdb.Query.GetElosByIds(ctx, []int64{testUser1.ID})
	assert.NoError(t, err)

	assert.Equal(t, e1[0].Elo, float64(1015))
	assert.Equal(t, e2[0].Elo, float64(985))

	r1, err := pdb.Query.GetReplayRowByID(ctx, cs.ReplayID)
	assert.NoError(t, err)

	expReplay := db.Replay{
		ID:          cs.ReplayID,
		WhiteID:     testUser0.ID,
		BlackID:     testUser1.ID,
		Result:      string(WhiteWin),
		Cause:       string(Checkmate),
		Mode:        string(TcUnlimited),
		WinEloDiff:  15,
		LoseEloDiff: -15,
		WhiteElo:    1015,
		BlackElo:    985,
		MoveHistory: []byte{},
	}
	util.AssertEqualIgnoring(t, expReplay, r1, ReplayRowCmpOpts)

	cs.ReplayID = 0
	cs.WinEloDiff = math.Round(cs.WinEloDiff)
	cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
	expChange := GRChangeSet{WinID: testUser0.ID, LoseID: testUser1.ID, WinEloDiff: 15, LoseEloDiff: -15}
	assert.Equal(t, expChange, cs)
}
