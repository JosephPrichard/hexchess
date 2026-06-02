package svc

import (
	"go.uber.org/mock/gomock"
	"hexchess-svc/pubsub"

	"testing"
	"time"

	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveUser(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	broadcaster := pubsub.NewMockBroadcasterAPI(ctrl)
	broadcaster.EXPECT().
		BroadcastActiveCount(gomock.Any(), gomock.Eq(int64(1)), gomock.Any())
	broadcaster.EXPECT().
		BroadcastActiveCount(gomock.Any(), gomock.Eq(int64(2)), gomock.Any())
	broadcaster.EXPECT().
		BroadcastActiveCount(gomock.Any(), gomock.Eq(int64(1)), gomock.Any())
	broadcaster.EXPECT().
		BroadcastActiveCount(gomock.Any(), gomock.Eq(int64(2)), gomock.Any())

	services, _ := setupServicesTest(t, serviceMocks{Broadcaster: broadcaster}, itest.Redis)
	defer services.Close()

	services.entropy = &StableEntropySource{CurrTime: time.UnixMilli(int64(ActiveUserMaxage * 5))}

	ctx := t.Context()

	_, err := services.AddActiveUser(ctx, "1")
	require.NoError(t, err)
	countAfterAdding, err := services.AddActiveUser(ctx, "2")
	require.NoError(t, err)

	countAfterRemoval, err := services.RemoveActiveUser(ctx, "2")
	require.NoError(t, err)

	// adds and does not expire
	services.entropy = &StableEntropySource{CurrTime: time.UnixMilli(int64(ActiveUserMaxage * 2))}
	countAfterRemoveAndAdd, err := services.AddActiveUser(ctx, "3")
	require.NoError(t, err)

	// gets and expires the active user we just added, without expiring any others
	services.entropy = &StableEntropySource{CurrTime: time.UnixMilli(int64(ActiveUserMaxage * 3))}
	countAfterExpiry, err := services.GetActiveCount(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(1), countAfterRemoval)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
