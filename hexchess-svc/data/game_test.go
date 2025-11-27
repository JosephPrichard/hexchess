package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/chess"
	"hexchess-svc/util"
	"math"
	"strconv"
	"testing"
	"time"
)

func assertStateRdb(t *testing.T, rdb *Redis, expState ChessState) {
	ctx := context.WithValue(context.Background(), util.Trace, "assert-chess-states")
	actualState, err := GetChessState(ctx, rdb, expState.ID)
	if err != nil {
		t.Fatalf("failed to get chess for assert: %v", err)
	}
	util.AssertEqualIgnoring(t, expState, actualState, ChessMetaCmpOpts)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-join-game")

	gameID := "test123"
	inState := MakeState(gameID, TcRealTime, TcRandom, nil)
	inState.FirstColor = TcWhite
	player := PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	if _, err := SetChessState(ctx, rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	updated, err := JoinGame(ctx, rdb, gameID, &player)
	assert.NoError(t, err)

	expState := inState.DeepCopy()
	expState.WhitePlayer = &player

	util.AssertEqualIgnoring(t, expState, updated, ChessMetaCmpOpts)
	assertStateRdb(t, rdb, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-join-game-both-players")

	gameID := "test123"
	inState := MakeState(gameID, TcRealTime, TcRandom, nil)
	inState.WhitePlayer = &PlayerState{ID: 1, Name: "white"}
	inState.BlackPlayer = &PlayerState{ID: 2, Name: "black"}

	if _, err := SetChessState(ctx, rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	result, err := JoinGame(ctx, rdb, gameID, &PlayerState{ID: 3, Name: "test"})

	assert.NoError(t, err)
	util.AssertEqualIgnoring(t, inState, result, ChessMetaCmpOpts)
	assertStateRdb(t, rdb, inState)
}

func TestMakeMove(t *testing.T) {
	dbs, closer := BeforeDatabasesTests(t, true)
	defer closer()

	s1 := MakeState("test1", TcRealTime, TcRandom, nil)
	s1.WhitePlayer = &PlayerState{ID: 1}
	s1.BlackPlayer = &PlayerState{ID: 2}

	s2 := ChessState{
		Game: chess.MakeEmptyGame(),
		ChessMeta: ChessMeta{
			ID:          "test2",
			FirstColor:  TcRandom,
			TimeControl: TcRealTime,
			Touch:       time.UnixMilli(0),
			WhitePlayer: &PlayerState{ID: 3},
			BlackPlayer: &PlayerState{ID: 4},
		},
	}
	s2.Game.Board.IsWhiteTurn = false
	s2.Game.SetPieces(
		chess.NotMove{Not: "f1", Piece: chess.WhiteKing},
		chess.NotMove{Not: "a2", Piece: chess.BlackQueen},
		chess.NotMove{Not: "h1", Piece: chess.BlackRook},
		chess.NotMove{Not: "f3", Piece: chess.BlackRook},
		chess.NotMove{Not: "f9", Piece: chess.BlackKing})

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
			if _, err := MakeGameMove(ctx, &dbs, test.state.ID, test.player, test.pm); err != nil {
				assert.Equal(t, test.expErr, err)
			}
		})
	}
}

func TestForfeit_BlackForfeits(t *testing.T) {
	dbs, closer := BeforeDatabasesTests(t, true)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-forfeit")

	gameID := "test123"
	inState := MakeState(gameID, TcRealTime, TcRandom, nil)
	inState.WhitePlayer = &PlayerState{ID: 1}
	inState.BlackPlayer = &PlayerState{ID: 2}
	inState.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	if _, err := SetChessState(ctx, dbs.Rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	assert.NoError(t, ForfeitGame(ctx, &dbs, gameID, *inState.BlackPlayer))

	expState := inState.DeepCopy()
	expState.IsEnded = true

	assertStateRdb(t, dbs.Rdb, expState)
}

func TestUpdateGameResultTx(t *testing.T) {
	pdb, closer := BeforeDbTests(t, true)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-update-stats")

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]

	cs, err := UpdateGameResultTx(ctx, pdb, GRParams{WhiteID: testUser0.ID, BlackID: testUser1.ID, Cause: Checkmate, IsWhiteWin: true, MoveHistoryProto: []byte{}})
	assert.NoError(t, err)

	u1, err := GetUserByID(ctx, pdb.Query, testUser0.ID)
	assert.NoError(t, err)
	u2, err := GetUserByID(ctx, pdb.Query, testUser1.ID)
	assert.NoError(t, err)
	r1, err := GetReplay(ctx, pdb.Query, cs.ReplayID)
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
		WhiteID:      testUser0.ID,
		BlackID:      testUser1.ID,
		WhiteName:    testUser0.Username,
		BlackName:    testUser1.Username,
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinElo:       15,
		LoseElo:      -15,
		WhiteEloDiff: 15,
		BlackEloDiff: -15,
		WhiteElo:     1015,
		BlackElo:     985,
		PlayedOn:     time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	util.AssertEqualIgnoring(t, expReplay, r1, ReplayEntityCmpOpts)

	cs.ReplayID = 0 // we can't assert this and don't care to, since replayID is autogenerated
	expChange := GRChangeSet{WinID: testUser0.ID, LoseID: testUser1.ID, WinEloDiff: 15, LoseEloDiff: -15}
	assert.Equal(t, expChange, cs)
}
