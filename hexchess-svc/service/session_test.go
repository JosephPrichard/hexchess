package svc

import (
	"context"
	"hexchess-svc/domain"

	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessions(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	playerIn := domain.MakePlayer(1, "testing-session", "country")
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	require.NoError(t, services.SetSessions(ctx, SessionInst{sessionID1, playerIn, 100 * time.Second}))
	require.NoError(t, services.SetSessions(ctx, SessionInst{sessionID2, playerIn, 100 * time.Second}))
	require.NoError(t, services.SetSessions(ctx, SessionInst{sessionID3, playerIn, 100 * time.Second}))

	playerOut, err := services.GetSession(ctx, sessionID1)
	require.NoError(t, err)

	require.NoError(t, services.DeleteSession(ctx, sessionID2))
	require.NoError(t, services.UpdateSessionEx(ctx, sessionID3, 0))

	_, badIDErr1 := services.GetSession(ctx, sessionID2)
	_, badIDErr2 := services.GetSession(ctx, sessionID3)

	assert.Equal(t, ErrSessionNotFound, badIDErr1)
	assert.Equal(t, ErrSessionNotFound, badIDErr2)
	assert.Equal(t, playerIn, playerOut)
}
