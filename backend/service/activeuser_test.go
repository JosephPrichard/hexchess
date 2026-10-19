package service

import (
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/slogutil"
	"testing"
	"time"

	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupActiveTest(t slogutil.TestLogger) (*ActiveUserService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewActiveUserService(infra.Redis, pubsub.NewSyncBroadcaster(infra.Redis))

	return service, infra
}

func TestActiveUser(t *testing.T) {
	service, testinfra := setupActiveTest(t)
	defer testinfra.Close()

	service.entropy = &entropy.StableSource{CurrTime: time.UnixMilli(int64(ActiveUserMaxAge * 5))}
	ctx := t.Context()

	_, err := service.AddActiveUser(ctx, "1")
	require.NoError(t, err)
	countAfterAdding, err := service.AddActiveUser(ctx, "2")
	require.NoError(t, err)

	countAfterRemoval, err := service.RemoveActiveUser(ctx, "2")
	require.NoError(t, err)

	// adds and does not expire
	service.entropy = &entropy.StableSource{CurrTime: time.UnixMilli(int64(ActiveUserMaxAge * 2))}
	countAfterRemoveAndAdd, err := service.AddActiveUser(ctx, "3")
	require.NoError(t, err)

	// gets and expires the active user we just added, without expiring any others
	service.entropy = &entropy.StableSource{CurrTime: time.UnixMilli(int64(ActiveUserMaxAge * 3))}
	countAfterExpiry, err := service.GetActiveCount(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(1), countAfterRemoval)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
