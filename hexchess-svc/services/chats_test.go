package svc

import (
	"context"
	"hexchess-svc/internal/logutil"
	"hexchess-svc/internal/testutil"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestEchoStateChats(t *testing.T) {
	t.Parallel()

	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	chatsIn := []Chat{
		{
			Player:  PlayerState{ID: 1, Name: "name", Country: "us", Present: true},
			Message: "test1",
			SentAt:  time.Date(2022, 1, 1, 0, 1, 0, 0, time.UTC),
		},
		{
			Message: "test2",
			SentAt:  time.Date(2022, 1, 1, 0, 2, 0, 0, time.UTC),
		},
		{
			Message: "test3",
			SentAt:  time.Date(2022, 1, 1, 0, 3, 0, 0, time.UTC),
		},
	}

	for _, chat := range chatsIn {
		require.NoError(t, services.InsertStateChat(ctx, id1, chat))
	}

	chatsOut, err := services.GetStateChats(ctx, id1, 3)
	require.NoError(t, err)

	wantChats := []*pb.ChatMessage{
		{
			Message: "test3",
			SentAt:  "2022-01-01T00:03:00Z",
		},
		{
			Message: "test2",
			SentAt:  "2022-01-01T00:02:00Z",
		},
		{
			Player: &pb.PlayerState{
				Id:      1,
				Name:    "name",
				Country: "us",
				IsGuest: false,
			},
			Message: "test1",
			SentAt:  "2022-01-01T00:01:00Z",
		},
	}
	testutil.Equal(t, wantChats, chatsOut, protocmp.Transform())
}
