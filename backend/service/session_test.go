package service

import (
	"hexchess-svc/model"
	"hexchess-svc/utils/slogutil"
	"testing"
	"time"

	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t slogutil.TestLogger) (*SessionService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewSessionService(infra.Redis)

	return service, infra
}

func TestSessions(t *testing.T) {
	service, testinfra := setupTest(t)
	defer testinfra.Close()

	playerIn := model.NewPlayer(1, "testing-session", "country")
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"
	ctx := t.Context()

	require.NoError(t, service.SetSessions(ctx, SessionInst{sessionID1, playerIn, 100 * time.Second}))
	require.NoError(t, service.SetSessions(ctx, SessionInst{sessionID2, playerIn, 100 * time.Second}))
	require.NoError(t, service.SetSessions(ctx, SessionInst{sessionID3, playerIn, 100 * time.Second}))

	playerOut, err := service.GetSession(ctx, sessionID1)
	require.NoError(t, err)

	require.NoError(t, service.DeleteSession(ctx, sessionID2))
	require.NoError(t, service.UpdateSessionEx(ctx, sessionID3, 0))

	_, badIDErr1 := service.GetSession(ctx, sessionID2)
	_, badIDErr2 := service.GetSession(ctx, sessionID3)

	assert.Equal(t, ErrSessionNotFound, badIDErr1)
	assert.Equal(t, ErrSessionNotFound, badIDErr2)
	assert.Equal(t, playerIn, playerOut)
}
