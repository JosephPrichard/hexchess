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

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-sessions")

	// when
	require.NoError(t, SetSession(ctx, rdb, sessionID1, playerIn, 100*time.Second))
	require.NoError(t, SetSession(ctx, rdb, sessionID2, playerIn, 100*time.Second))
	require.NoError(t, SetSession(ctx, rdb, sessionID3, playerIn, 100*time.Second))

	playerOut, err := GetSession(ctx, rdb, sessionID1)
	require.NoError(t, err)

	require.NoError(t, DeleteSession(ctx, rdb, sessionID2))
	require.NoError(t, UpdateSessionEx(ctx, rdb, sessionID3, 0))

	_, badIDErr1 := GetSession(ctx, rdb, sessionID2)
	_, badIDErr2 := GetSession(ctx, rdb, sessionID3)

	// then
	assert.Equal(t, ErrSessionNotFound, badIDErr1)
	assert.Equal(t, ErrSessionNotFound, badIDErr2)
	assert.Equal(t, playerIn, playerOut)
}
