package user

import (
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/entropy"
	"testing"
	"time"

	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupActiveTest(t alog.TestLogger) (*ActiveUserService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewActiveUserService(infra.Redis, pubsub.NewSyncBroadcaster(infra.Redis))

	return services, infra
}

func TestActiveUser(t *testing.T) {
	services, testinfra := setupActiveTest(t)
	defer testinfra.Close()

	services.entropy = &entropy.StableSource{CurrTime: time.UnixMilli(int64(ActiveUserMaxAge * 5))}
	ctx := t.Context()

	_, err := services.AddActiveUser(ctx, "1")
	require.NoError(t, err)
	countAfterAdding, err := services.AddActiveUser(ctx, "2")
	require.NoError(t, err)

	countAfterRemoval, err := services.RemoveActiveUser(ctx, "2")
	require.NoError(t, err)

	// adds and does not expire
	services.entropy = &entropy.StableSource{CurrTime: time.UnixMilli(int64(ActiveUserMaxAge * 2))}
	countAfterRemoveAndAdd, err := services.AddActiveUser(ctx, "3")
	require.NoError(t, err)

	// gets and expires the active user we just added, without expiring any others
	services.entropy = &entropy.StableSource{CurrTime: time.UnixMilli(int64(ActiveUserMaxAge * 3))}
	countAfterExpiry, err := services.GetActiveCount(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(1), countAfterRemoval)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
