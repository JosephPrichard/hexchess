package gameplay

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func setupChatsServicesTest(t slogutil.TestLogger) (*ChatService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewChatService(infra.Redis, infra.Querier(), entropy.RealSource{})

	return services, infra
}

func TestEchoStateChats(t *testing.T) {
	services, testinfra := setupChatsServicesTest(t)
	defer testinfra.Close()

	id1 := model.NewGameID()
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
