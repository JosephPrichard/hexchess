package svc

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	ctx := context.WithValue(t.Context(), util.Trace, "assert-chess-states")
	actualState, err := GetChessState(ctx, rdb, expState.ID)
	if err != nil {
		t.Fatalf("get chess for assert: %v", err)
	}
	util.AssertEqualIgnoring(t, expState, *actualState, ChessMetaCmpOpts)
}

func assertChessState(t *testing.T, expState ChessState, actualState *ChessState) {
	if actualState == nil {
		t.Fatalf("chess state is nil")
	}
	util.AssertEqualIgnoring(t, expState, *actualState, ChessMetaCmpOpts)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-join-game")

	gameID := "test123"
	inState := MakeState(StateSetup{ID: gameID, TimeControl: TcRealTime, FirstColor: ColorRandom})
	inState.FirstColor = ColorWhite
	player := MakePlayer(1, "name", "us", 0)

	// when
	assert.NoError(t, SetChessState(ctx, rdb, gameID, &inState))

	updatedState, err := JoinGame(ctx, rdb, gameID, player)
	assert.NoError(t, err)

	// then
	expState := inState.DeepCopy()
	expState.WhitePlayer = player

	assertChessState(t, expState, updatedState)
	assertStateRdb(t, rdb, *updatedState)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-join-game-both-players")

	gameID := "test123"
	inState := MakeState(StateSetup{
		ID:          gameID,
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       util.Ptr(MakeNamePlayer(1, "white")),
		Black:       util.Ptr(MakeNamePlayer(2, "name")),
	})

	// when
	assert.NoError(t, SetChessState(ctx, rdb, gameID, &inState))

	resultState, err := JoinGame(ctx, rdb, gameID, MakeNamePlayer(3, "test"))
	assert.NoError(t, err)

	// then
	assertChessState(t, inState, resultState)
	assertStateRdb(t, rdb, inState)
}

func TestAttemptUndo(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-undo")

	gameID := "test123"
	inState := MakeState(StateSetup{
		ID:          gameID,
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       util.Ptr(MakeNamePlayer(1, "white")),
		Black:       util.Ptr(MakeNamePlayer(2, "black")),
	})

	makeExpState := func(fn func(s *ChessState)) ChessState {
		state := inState.DeepCopy()
		fn(&state)
		return state
	}

	if err := SetChessState(ctx, rdb, gameID, &inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	for i, test := range []struct {
		kind     UndoKind
		player   PlayerState
		expState ChessState
		expErr   error
	}{
		{
			kind:   UndoCreate,
			player: MakeIDPlayer(1),
			expState: makeExpState(func(s *ChessState) {
				s.UndoState = UndoState{UndoID: 1}
			}),
		},
		{
			kind:   UndoReject,
			player: MakeIDPlayer(1),
			expState: makeExpState(func(s *ChessState) {
				s.UndoState = UndoState{}
			}),
		},
		{
			kind:   UndoCreate,
			player: MakeIDPlayer(2),
			expState: makeExpState(func(s *ChessState) {
				s.UndoState = UndoState{UndoID: 2}
			}),
		},
		{
			kind:   UndoAccept,
			player: MakeIDPlayer(2),
			expErr: ErrUndoNoop,
			expState: makeExpState(func(s *ChessState) {
				s.UndoState = UndoState{UndoID: 2}
			}),
		},
		{
			kind:   UndoAccept,
			player: MakeIDPlayer(1),
			expErr: ErrNoMoveUndo,
			expState: makeExpState(func(s *ChessState) {
				s.UndoState = UndoState{}
			}),
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// when
			state, err := AttemptGameUndo(ctx, rdb, gameID, test.player, test.kind)

			// then
			if test.expErr != nil {
				assert.Equal(t, test.expErr, err)
			} else {
				assertChessState(t, test.expState, state)
				assertStateRdb(t, rdb, *state)
			}
		})
	}
}

func TestMakeMove(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, InsertTestData)
	defer closer()

	s1 := MakeState(StateSetup{
		ID:          "test1",
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       util.Ptr(MakeIDPlayer(1)),
		Black:       util.Ptr(MakeIDPlayer(2)),
	})
	s2 := MakeState(StateSetup{
		ID:          "test2",
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       util.Ptr(MakeIDPlayer(3)),
		Black:       util.Ptr(MakeIDPlayer(4)),
		Game: util.Ptr(chess.MakeEmptyGame(false,
			chess.NotMove{Not: "f1", Piece: chess.WhiteKing},
			chess.NotMove{Not: "a2", Piece: chess.BlackQueen},
			chess.NotMove{Not: "h1", Piece: chess.BlackRook},
			chess.NotMove{Not: "f3", Piece: chess.BlackRook},
			chess.NotMove{Not: "f9", Piece: chess.BlackKing},
		)),
	})

	ctx := context.WithValue(t.Context(), util.Trace, "testing-make-move")

	for _, state := range []ChessState{s1, s2} {
		state.Game.InitPieceMoves()
		if err := SetChessState(ctx, dbs.Rdb, state.ID, &state); err != nil {
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
			player: s1.BlackPlayer,
			expErr: ErrTurn,
		},
		{
			pm:     chess.Move{To: chess.Hex{File: 1}}, // invalid move
			state:  s1,
			player: s1.WhitePlayer,
			expErr: ErrInvalidMove,
		},
		{
			pm:     chess.Move{Promotion: chess.QueenPromotion, From: chess.HexStr("b1"), To: chess.HexStr("b2")}, // valid move
			state:  s1,
			player: s1.WhitePlayer,
		},
		{
			pm:     chess.Move{Promotion: chess.QueenPromotion, From: chess.HexStr("a2"), To: chess.HexStr("a1")}, // valid move
			state:  s2,
			player: s2.BlackPlayer,
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

	ctx := context.WithValue(t.Context(), util.Trace, "testing-forfeit")

	gameID := "test123"
	inState := MakeState(StateSetup{
		ID:          gameID,
		TimeControl: TcRealTime,
		FirstColor:  ColorRandom,
		White:       util.Ptr(MakeIDPlayer(1)),
		Black:       util.Ptr(MakeIDPlayer(2)),
	})
	inState.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	// when
	assert.NoError(t, SetChessState(ctx, dbs.Rdb, gameID, &inState))
	assert.NoError(t, ForfeitGame(ctx, &dbs, gameID, inState.BlackPlayer))

	// then
	expState := inState.DeepCopy()
	expState.IsEnded = true

	assertStateRdb(t, dbs.Rdb, expState)
}

func TestInsertGameResult(t *testing.T) {
	ctx := context.WithValue(t.Context(), util.Trace, "testing-insert-game-result")

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]

	for i, test := range []struct {
		result      GameResult
		expWhiteElo float64
		expBlackElo float64
		expReplay   db.Replay
		expChange   GRChangeSet
	}{
		{
			result:      GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Stalemate, ReplayResult: Draw, ReplayMode: ModeRealTime},
			expWhiteElo: 1000,
			expBlackElo: 1000,
			expReplay: db.Replay{
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      string(Draw),
				Cause:       string(Stalemate),
				Mode:        string(ModeRealTime),
				WinEloDiff:  0,
				LoseEloDiff: 0,
				WhiteElo:    1000,
				BlackElo:    1000,
				MoveHistory: []byte{},
			},
			expChange: GRChangeSet{},
		},
		{
			result:      GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Checkmate, ReplayResult: WhiteWin, ReplayMode: ModeUnlimited},
			expWhiteElo: 1015,
			expBlackElo: 985,
			expReplay: db.Replay{
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
			},
			expChange: GRChangeSet{WinID: testUser0.ID, LoseID: testUser1.ID, WinEloDiff: 15, LoseEloDiff: -15},
		},
		{
			result:      GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Forfeit, ReplayResult: BlackWin, ReplayMode: ModeCorrespondence},
			expWhiteElo: 985,
			expBlackElo: 1015,
			expReplay: db.Replay{
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      string(BlackWin),
				Cause:       string(Forfeit),
				Mode:        string(TcCorrespondence),
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    985,
				BlackElo:    1015,
				MoveHistory: []byte{},
			},
			expChange: GRChangeSet{WinID: testUser1.ID, LoseID: testUser0.ID, WinEloDiff: 15, LoseEloDiff: -15},
		},
	} {
		t.Run(fmt.Sprintf("%s-%d", test.result.ReplayResult, i), func(t *testing.T) {
			// given
			pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
			defer closer()

			// when
			cs, err := InsertGameResultTx(ctx, pdb, time.Now(), test.result)
			assert.NoError(t, err)

			// then
			elos, err := pdb.Query.GetElosByIds(ctx, []int64{test.result.WhiteID, test.result.BlackID})
			assert.NoError(t, err)

			assert.Equal(t, elos[0].Elo, test.expWhiteElo)
			assert.Equal(t, elos[1].Elo, test.expBlackElo)

			r1, err := pdb.Query.GetReplayRowByID(ctx, cs.ReplayID)
			assert.NoError(t, err)

			util.AssertEqualIgnoring(t, test.expReplay, r1, cmpopts.IgnoreFields(db.Replay{}, "ID", "PlayedOn"))

			cs.ReplayID = 0
			cs.WinEloDiff = math.Round(cs.WinEloDiff)
			cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
			assert.Equal(t, test.expChange, cs)
		})
	}
}
