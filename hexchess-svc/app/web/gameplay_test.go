package web

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"hexchess-svc/app/chess"
	"hexchess-svc/app/data"
	"hexchess-svc/app/util"
	"testing"
	"time"
)

func assertStatesEqual(t *testing.T, expState data.ChessState, actualState data.ChessState) {
	// empty fields we do not want to assert
	expState.Touch = time.Time{}
	actualState.Touch = time.Time{}
	assert.Equal(t, expState, actualState)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	d := data.NewMockGameplayDAL(ctrl)

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-join-game")

	gameID := "test123-" + uuid.NewString()

	inpState := data.MakeStartChessState(gameID, data.RealTime)
	inpState.FirstColor = data.White
	inpPlayer := data.PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	retState := inpState.DeepCopy()
	retState.WhitePlayer = &inpPlayer

	d.EXPECT().
		GetChessState(gomock.Any(), gomock.Eq(gameID)).
		Return(inpState, nil)
	d.EXPECT().
		SetChessState(gomock.Any(), gomock.Eq(gameID), gomock.Eq(retState)).
		Return(retState, nil)

	updated, err := JoinGame(ctx, d, gameID, inpPlayer)

	assert.NoError(t, err)
	assert.Equal(t, retState, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := data.NewMockGameplayDAL(ctrl)

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-join-game-both-players")

	gameID := "test123-" + uuid.NewString()

	inpState := data.MakeStartChessState(gameID, data.RealTime)
	inpState.WhitePlayer = &data.PlayerState{ID: 1, Name: "white"}
	inpState.BlackPlayer = &data.PlayerState{ID: 2, Name: "black"}

	d.EXPECT().
		GetChessState(gomock.Any(), gomock.Eq(gameID)).
		Return(inpState, nil)

	result, err := JoinGame(ctx, d, gameID, data.PlayerState{ID: 3, Name: "extra"})
	assert.NoError(t, err)

	assertStatesEqual(t, inpState, result)
}

func TestMakeMove(t *testing.T) {
	gameID := "test123-" + uuid.NewString()

	ss := data.MakeStartChessState(gameID, data.RealTime)
	ss.WhitePlayer = &data.PlayerState{ID: 1}
	ss.BlackPlayer = &data.PlayerState{ID: 2}

	fs := data.ChessState{
		ID:          gameID,
		Game:        chess.MakeEmptyGame(),
		FirstColor:  data.Random,
		TimeControl: data.RealTime,
		Touch:       time.UnixMilli(0),
		WhitePlayer: &data.PlayerState{ID: 1},
		BlackPlayer: &data.PlayerState{ID: 2},
	}
	fs.Game.Board.IsWhiteTurn = false
	fs.Game.
		SetPiece("f1", chess.WhiteKing).
		SetPiece("a2", chess.BlackQueen).
		SetPiece("h1", chess.BlackRook).
		SetPiece("f3", chess.BlackRook).
		SetPiece("f9", chess.BlackKing)

	type Test struct {
		pm             chess.PieceMove
		state          data.ChessState
		player         data.PlayerState
		makesCheckmate bool
		expErr         error
	}

	tests := []Test{
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid turn
			state:  ss,
			player: *ss.BlackPlayer,
			expErr: ErrTurn,
		},
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid move
			state:  ss,
			player: *ss.WhitePlayer,
			expErr: ErrInvalidMove,
		},
		{
			pm:     chess.PieceMove{Piece: chess.WhitePawn, From: chess.Hex{File: 1, Rank: 0}, To: chess.Hex{File: 1, Rank: 1}}, // valid move
			state:  ss,
			player: *ss.WhitePlayer,
		},
		{
			pm:             chess.PieceMove{Piece: chess.BlackQueen, From: chess.Hex{File: 0, Rank: 1}, To: chess.Hex{File: 0, Rank: 0}}, // valid move
			state:          fs,
			makesCheckmate: true,
			player:         *ss.BlackPlayer,
		},
	}

	runTest := func(t *testing.T, test Test) {
		ctrl := gomock.NewController(t)
		d := data.NewMockGameplayDAL(ctrl)

		ctx := context.WithValue(context.Background(), util.TraceKey, "test-make-move")

		// don't assert the chess states, since it is too complex to test that logic here
		d.EXPECT().
			GetChessState(gomock.Any(), gomock.Eq(gameID)).
			Return(test.state, nil)
		if test.expErr == nil {
			d.EXPECT().
				SetChessState(gomock.Any(), gomock.Eq(gameID), gomock.Any()).
				DoAndReturn(func(_ context.Context, gameID string, state data.ChessState) (data.ChessState, error) {
					return state, nil // echo stub
				})
		}
		if test.makesCheckmate {
			d.EXPECT().
				WriteFinishedGame(gomock.Any(), gomock.Any(), false, data.Checkmate).
				Return(nil)
		}

		if result, err := MakeGameMove(ctx, d, gameID, test.player, test.pm); err == nil {
			assert.Equal(t, test.pm, result.Move)
		} else {
			assert.Equal(t, test.expErr, err)
		}
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) { runTest(t, test) })
	}
}

var MockMoveList = []chess.PieceMove{{Piece: 1, To: chess.Hex{Rank: 1}}}

func TestForfeit_BlackForfeits(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := data.NewMockGameplayDAL(ctrl)

	gameID := "test123-" + uuid.NewString()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-forfeit")

	inpState := data.MakeStartChessState(gameID, data.RealTime)
	inpState.WhitePlayer = &data.PlayerState{ID: 1}
	inpState.BlackPlayer = &data.PlayerState{ID: 2}
	inpState.MoveList = MockMoveList

	retState := inpState.DeepCopy()
	retState.IsEnded = true

	d.EXPECT().
		GetChessState(gomock.Any(), gomock.Eq(gameID)).
		Return(inpState, nil)
	d.EXPECT().
		WriteFinishedGame(gomock.Any(), gomock.Eq(retState), gomock.Eq(true), gomock.Eq(data.Forfeit)).
		Return(nil)
	d.EXPECT().
		SetChessState(gomock.Any(), gomock.Eq(gameID), gomock.Eq(retState)).
		Return(retState, nil)

	assert.NoError(t, ForfeitGame(ctx, d, gameID, *inpState.BlackPlayer))
}
