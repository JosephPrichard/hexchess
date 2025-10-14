package data

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/app/util"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	player := PlayerState{ID: 1, Name: "test-name1"}
	sessionID1 := "session1" + uuid.NewString()
	sessionID2 := "session2" + uuid.NewString()
	sessionID3 := "session3" + uuid.NewString()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-sessions")

	assert.NoError(t, SetSession(ctx, rdb, sessionID1, player, 100*time.Second))
	assert.NoError(t, SetSession(ctx, rdb, sessionID2, player, 100*time.Second))
	assert.NoError(t, SetSession(ctx, rdb, sessionID3, player, 100*time.Second))

	rdbPlayer1, err := GetSession(ctx, rdb, sessionID1)
	assert.NoError(t, err)
	assert.Equal(t, player, rdbPlayer1)

	assert.NoError(t, DeleteSession(ctx, rdb, sessionID2))

	assert.NoError(t, UpdateSessionEx(ctx, rdb, sessionID3, 0))

	_, err = GetSession(ctx, rdb, sessionID2)
	assert.Equal(t, ErrNoSession, err)

	_, err = GetSession(ctx, rdb, sessionID3)
	assert.Equal(t, ErrNoSession, err)
}
