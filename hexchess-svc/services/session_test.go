package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	playerIn := MakePlayer(1, "testing-session", "country")
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Redis: rdb}

	// when
	require.NoError(t, s.SetSession(ctx, sessionID1, playerIn, 100*time.Second))
	require.NoError(t, s.SetSession(ctx, sessionID2, playerIn, 100*time.Second))
	require.NoError(t, s.SetSession(ctx, sessionID3, playerIn, 100*time.Second))

	playerOut, err := s.GetSession(ctx, sessionID1)
	require.NoError(t, err)

	require.NoError(t, s.DeleteSession(ctx, sessionID2))
	require.NoError(t, s.UpdateSessionEx(ctx, sessionID3, 0))

	_, badIDErr1 := s.GetSession(ctx, sessionID2)
	_, badIDErr2 := s.GetSession(ctx, sessionID3)

	// then
	assert.Equal(t, ErrSessionNotFound, badIDErr1)
	assert.Equal(t, ErrSessionNotFound, badIDErr2)
	assert.Equal(t, playerIn, playerOut)
}
