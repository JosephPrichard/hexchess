package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/infra"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	rdb := infra.BeforeRedisTest(t)
	defer rdb.Close()

	player := PlayerState{ID: 1, Name: "testing-name1"}
	sessionID1 := "session1"
	sessionID2 := "session2"
	sessionID3 := "session3"

	ctx := context.WithValue(context.Background(), util.Trace, "testing-sessions")

	assert.NoError(t, SetSession(ctx, rdb, sessionID1, player, 100*time.Second))
	assert.NoError(t, SetSession(ctx, rdb, sessionID2, player, 100*time.Second))
	assert.NoError(t, SetSession(ctx, rdb, sessionID3, player, 100*time.Second))

	rdbPlayer1, err := GetSession(ctx, rdb, sessionID1)
	assert.NoError(t, err)
	assert.Equal(t, player, rdbPlayer1)

	assert.NoError(t, DeleteSession(ctx, rdb, sessionID2))

	assert.NoError(t, UpdateSessionEx(ctx, rdb, sessionID3, 0))

	_, err = GetSession(ctx, rdb, sessionID2)
	assert.Equal(t, ErrSessionNotFound, err)

	_, err = GetSession(ctx, rdb, sessionID3)
	assert.Equal(t, ErrSessionNotFound, err)
}
