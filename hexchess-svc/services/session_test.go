package svc

import (
	"context"
	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessions(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	playerIn := MakePlayer(1, "testing-session", "country")
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	require.NoError(t, services.SetSessions(ctx, SessInst{sessionID1, playerIn, 100 * time.Second}))
	require.NoError(t, services.SetSessions(ctx, SessInst{sessionID2, playerIn, 100 * time.Second}))
	require.NoError(t, services.SetSessions(ctx, SessInst{sessionID3, playerIn, 100 * time.Second}))

	playerOut, err := services.GetSession(ctx, sessionID1)
	require.NoError(t, err)

	require.NoError(t, services.DeleteSession(ctx, sessionID2))
	require.NoError(t, services.UpdateSessionEx(ctx, sessionID3, 0))

	_, badIDErr1 := services.GetSession(ctx, sessionID2)
	_, badIDErr2 := services.GetSession(ctx, sessionID3)

	// then
	assert.Equal(t, ErrSessionNotFound, badIDErr1)
	assert.Equal(t, ErrSessionNotFound, badIDErr2)
	assert.Equal(t, playerIn, playerOut)
}
