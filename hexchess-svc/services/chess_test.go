package svc

import (
	"context"
	"testing"
	"time"

	"hexchess-svc/chess"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoChessState(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()

	s1 := MakeChess(StateSetup{ID: id1, Mode: ModeCorrespondence1, FirstColor: Random})
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	require.NoError(t, services.SetChessState(ctx, id1, &s1))

	outState1, err := services.GetChessState(ctx, id1)
	require.NoError(t, err)

	_, errBadID := services.GetChessState(ctx, id2)

	// then
	assert.Equal(t, ErrNoChessState, errBadID)
	testutil.Equal(t, s1, *outState1, ChessMetaCmpOpt)
}

func TestGetChessMetas(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()
	id3 := "testing-id3-" + uuid.NewString()

	s1 := MakeChess(StateSetup{ID: id1, Mode: ModeCorrespondence1, FirstColor: Random, White: PlayerState{ID: 1, Present: true}, Black: PlayerState{ID: 2, Present: true}})
	s2 := MakeChess(StateSetup{ID: id2, Mode: ModeCorrespondence1, FirstColor: Random, Black: PlayerState{ID: 1, Present: true}})
	s3 := MakeChess(StateSetup{ID: id3, Mode: ModeCorrespondence1, FirstColor: Random, Black: PlayerState{ID: 1, Present: true}})

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	now := time.Now()

	// when
	// these times must be after now.Add(-GameExpireFinished)
	require.NoError(t, services.SetChessStateAt(ctx, id1, &s1, now.Add(-100*time.Second)))
	require.NoError(t, services.SetChessStateAt(ctx, id2, &s2, now.Add(-50*time.Second)))
	require.NoError(t, services.SetChessStateAt(ctx, id3, &s3, now.Add(-10*time.Second)))

	metaList1, err := services.GetUserChessMetas(ctx, 1)
	require.NoError(t, err)
	metaList2, err := services.GetUserChessMetas(ctx, 2)
	require.NoError(t, err)
	metaList3, err := services.GetUserChessMetas(ctx, 3)
	require.NoError(t, err)
	metaList4, err := services.GetUserChessMetasPaged(ctx, 1, 1, 2)
	require.NoError(t, err)
	metaList5, err := services.GetUserChessMetasPaged(ctx, 1, 2, 2)
	require.NoError(t, err)

	// then
	m1 := ChessMeta{ID: id1, WhitePlayer: PlayerState{ID: 1, Present: true}, BlackPlayer: PlayerState{ID: 2, Present: true}, FirstColor: Random, Mode: ModeCorrespondence1}
	m2 := ChessMeta{ID: id2, BlackPlayer: PlayerState{ID: 1, Present: true}, FirstColor: Random, Mode: ModeCorrespondence1}
	m3 := ChessMeta{ID: id3, BlackPlayer: PlayerState{ID: 1, Present: true}, FirstColor: Random, Mode: ModeCorrespondence1}

	assert.Equal(t, []ChessMeta{m3, m2, m1}, metaList1)
	assert.Equal(t, []ChessMeta{m1}, metaList2)
	assert.Empty(t, metaList3)
	assert.Equal(t, []ChessMeta{m3, m2}, metaList4)
	assert.Equal(t, []ChessMeta{m1}, metaList5)
}

func TestEchoStateChats(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	chatsIn := []StateChat{
		{
			Player: PlayerState{ID: 1, Name: "name", Country: "us", IsGuest: false, Present: true},
			SentAt: time.Date(2022, 1, 1, 0, 1, 0, 0, time.UTC),
		},
		{SentAt: time.Date(2022, 1, 1, 0, 2, 0, 0, time.UTC)},
		{SentAt: time.Date(2022, 1, 1, 0, 3, 0, 0, time.UTC)},
	}

	// when
	for _, chat := range chatsIn {
		require.NoError(t, services.InsertStateChat(ctx, id1, chat))
	}

	chatsOut, err := services.GetStateChats(ctx, id1, 3)
	require.NoError(t, err)

	// then
	assert.Equal(t, []StateChat{chatsIn[2], chatsIn[1], chatsIn[0]}, chatsOut)
}

func TestExpireChessStates(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()

	s1 := MakeChess(StateSetup{ID: id1, Mode: ModeCorrespondence1, FirstColor: Random})

	s1.WhitePlayer = PlayerState{ID: 1, Present: true}
	s1.BlackPlayer = PlayerState{ID: 2, Present: true}

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	now := time.Now()

	// when
	// these times must be before now.Add(-GameExpireFinished)
	require.NoError(t, services.SetChessStateAt(ctx, id1, &s1, now.Add(2*-GameExpireFinished)))
	require.NoError(t, services.InsertStateChat(ctx, id1, StateChat{}))
	require.NoError(t, services.ExpireChessStates(ctx, services.Redis.GamesZSet))

	_, errExpiredID := services.GetChessState(ctx, id1)
	chats, err := services.GetStateChats(ctx, id1, 1)
	require.NoError(t, err)

	// then
	assert.Equal(t, ErrNoChessState, errExpiredID)
	assert.Empty(t, chats)
}

func TestUndo(t *testing.T) {
	t.Parallel()

	t.Run("no moves to undo", func(t *testing.T) {
		t.Parallel()

		s := MakeChess(StateSetup{
			ID:           "test",
			Game:         New(chess.MakeStartGame()),
			InitialBoard: New(chess.InitialBoard()),
		})

		err := s.Undo()

		assert.Equal(t, ErrNoMoveUndo, err)
	})

	t.Run("successfully undoing game with one move", func(t *testing.T) {
		t.Parallel()

		game := chess.MakeStartGame()
		game.Moves = append(game.Moves, game.MakeMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")}))

		s := MakeChess(StateSetup{
			ID:           "test",
			Game:         New(game),
			InitialBoard: New(chess.InitialBoard()),
		})

		err := s.Undo()

		require.NoError(t, err)
		assert.Len(t, s.Game.Moves, 0)
	})
}
