package svc

import (
	"context"

	"testing"
	"time"

	"hexchess-svc/internal/logutil"
	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveUser(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	services.entropy = &StableEntropySource{CurrTime: time.UnixMilli(int64(ActiveUserMaxage * 5))}

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

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
