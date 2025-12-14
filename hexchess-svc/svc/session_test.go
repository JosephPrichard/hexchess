package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	playerIn := PlayerState{ID: 1, Name: "testing-name1"}
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"

	ctx := context.WithValue(context.Background(), util.Trace, "testing-sessions")

	// when
	assert.NoError(t, SetSession(ctx, rdb, sessionID1, playerIn, 100*time.Second))
	assert.NoError(t, SetSession(ctx, rdb, sessionID2, playerIn, 100*time.Second))
	assert.NoError(t, SetSession(ctx, rdb, sessionID3, playerIn, 100*time.Second))

	playerOut, err := GetSession(ctx, rdb, sessionID1)
	assert.NoError(t, err)

	assert.NoError(t, DeleteSession(ctx, rdb, sessionID2))
	assert.NoError(t, UpdateSessionEx(ctx, rdb, sessionID3, 0))

	_, badIDErr1 := GetSession(ctx, rdb, sessionID2)
	_, badIDErr2 := GetSession(ctx, rdb, sessionID3)

	// then
	assert.Equal(t, ErrSessionNotFound, badIDErr1)
	assert.Equal(t, ErrSessionNotFound, badIDErr2)
	assert.Equal(t, playerIn, playerOut)
}
