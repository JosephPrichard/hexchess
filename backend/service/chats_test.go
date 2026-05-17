package svc

import (
	"hexchess-svc/itest"
	"hexchess-svc/lib/testutil"
	"hexchess-svc/model"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestEchoStateChats(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()

	ctx := t.Context()

	chatsIn := []model.Chat{
		{
			Player:  model.PlayerState{ID: 1, Name: "name", Country: "us", Present: true},
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
		require.NoError(t, services.InsertChat(ctx, id1, chat))
	}

	chatsOut, err := services.GetChats(ctx, id1, 3)
	require.NoError(t, err)

	wantChats := []model.Chat{
		{
			Message: "test3",
			SentAt:  time.Date(2022, 1, 1, 0, 3, 0, 0, time.UTC),
		},
		{
			Message: "test2",
			SentAt:  time.Date(2022, 1, 1, 0, 2, 0, 0, time.UTC),
		},
		{
			Player:  model.PlayerState{ID: 1, Name: "name", Country: "us", Present: true},
			Message: "test1",
			SentAt:  time.Date(2022, 1, 1, 0, 1, 0, 0, time.UTC),
		},
	}
	testutil.Equal(t, wantChats, chatsOut, protocmp.Transform())
}
