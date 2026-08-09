package session

import (
	"hexchess-svc/model"
	"hexchess-svc/utils/alog"
	"testing"
	"time"

	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t alog.TestLogger, flags ...itest.TestFlag) (*SessionService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewSessionService(infra.Redis)

	return services, infra
}

func TestSessions(t *testing.T) {
	t.Parallel()

	services, testinfra := setupTest(t, itest.Redis)
	defer testinfra.Close()

	playerIn := model.NewPlayer(1, "testing-session", "country")
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"

	ctx := t.Context()

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
