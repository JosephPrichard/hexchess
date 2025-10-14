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

	inpState := data.MakeStartChessState(gameID, data.RealTime)
	inpState.WhitePlayer = &data.PlayerState{ID: 1}
	inpState.BlackPlayer = &data.PlayerState{ID: 2}

	tests := []struct {
		pm     chess.PieceMove
		player data.PlayerState
		expErr error
	}{
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid turn
			player: *inpState.BlackPlayer,
			expErr: ErrTurn,
		},
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid move
			player: *inpState.WhitePlayer,
			expErr: ErrInvalidMove,
		},
		{
			pm:     chess.PieceMove{Piece: chess.WhitePawn, From: chess.Hex{File: 1, Rank: 0}, To: chess.Hex{File: 1, Rank: 1}}, // valid move
			player: *inpState.WhitePlayer,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			d := data.NewMockGameplayDAL(ctrl)

			ctx := context.WithValue(context.Background(), util.TraceKey, "test-make-move")

			d.EXPECT().
				GetChessState(gomock.Any(), gomock.Eq(gameID)).
				Return(inpState, nil)
			if test.expErr == nil {
				d.EXPECT().
					// don't assert the chess state, since it is too complex to test that logic here
					SetChessState(gomock.Any(), gomock.Eq(gameID), gomock.Any()).
					Return(inpState, nil)
			}

			result, err := MakeGameMove(ctx, d, gameID, test.player, test.pm)
			if err == nil {
				assert.Equal(t, test.pm, result.Move)
			} else {
				assert.Equal(t, test.expErr, err)
			}
		})
	}
}

var MockMoveList = []chess.PieceMove{{Piece: 1, To: chess.Hex{Rank: 1}}}

func TestHandleFinishGame(t *testing.T) {
	ctrl := gomock.NewController(t)
	d := data.NewMockGameplayDAL(ctrl)

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-finish-game")

	gameID := "test123-" + uuid.NewString()

	inpState := data.MakeStartChessState(gameID, data.RealTime)
	inpState.WhitePlayer = &data.PlayerState{ID: 1}
	inpState.BlackPlayer = &data.PlayerState{ID: 2}
	inpState.MoveList = MockMoveList

	d.EXPECT().
		UpdateGameResult(gomock.Any(), gomock.Eq(data.GRParams{WhiteID: 1, BlackID: 2, Cause: data.Checkmate, IsWhiteWin: true, MoveList: MockMoveList})).
		Return(data.GRChangeSet{ReplayID: 1, WinID: 1, LoseID: 2, WinEloDiff: 30, LoseEloDiff: -30}, nil)
	d.EXPECT().
		UpdateLeaderboard(gomock.Any(), gomock.Eq([]data.UpdtLbChangeSet{{1, 30}, {2, -30}})).
		Return(nil)

	assert.NoError(t, handleFinishGame(ctx, d, inpState, true, data.Checkmate))
}

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
		UpdateGameResult(gomock.Any(), gomock.Eq(data.GRParams{WhiteID: 1, BlackID: 2, Cause: data.Forfeit, IsWhiteWin: true, MoveList: MockMoveList})).
		Return(data.GRChangeSet{ReplayID: 1, WinID: 1, LoseID: 2, WinEloDiff: 30, LoseEloDiff: -30}, nil)
	d.EXPECT().
		UpdateLeaderboard(gomock.Any(), gomock.Eq([]data.UpdtLbChangeSet{{1, 30}, {2, -30}})).
		Return(nil)
	d.EXPECT().
		SetChessState(gomock.Any(), gomock.Eq(gameID), gomock.Eq(retState)).
		Return(retState, nil)

	assert.NoError(t, ForfeitGame(ctx, d, gameID, *inpState.BlackPlayer))
}
