package svc

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"testing"
	"time"
)

func TestDictionary(t *testing.T) {
	ctx := context.Background()

	cont, err := tcredis.Run(ctx, "redis:6-alpine", testcontainers.WithExposedPorts("6379"))
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(cont); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	host, err := cont.Container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host: %s", err)
	}
	r, err := cont.Container.Inspect(ctx)
	if err != nil {
		t.Fatalf("failed to get port: %s", err)
	}
	pm := r.NetworkSettings.Ports
	port := pm["6379/tcp"][0].HostPort

	t.Logf("connecting on host: %s and port: %v", host, port)
	client := redis.NewClient(&redis.Options{Addr: host + ":" + port})

	t.Run("test-set-then-get", func(t *testing.T) {
		testSetThenGetViews(t, client)
	})
	t.Run("test-set-then-get-user", func(t *testing.T) {
		testSetThenGetUserViews(t, client)
	})
	t.Run("test-sessions", func(t *testing.T) {
		testSessions(t, client)
	})
	t.Run("test-leaderboard", func(t *testing.T) {
		testLeaderboard(t, client)
	})
}

func testSetThenGetViews(t *testing.T, rdb *redis.Client) {
	id1 := "test-id1"
	id2 := "test-id2"
	id3 := "test-id3"
	id4 := "test-id4"

	state1 := MakeStartChessState(id1, RealTime)
	state2 := MakeStartChessState(id2, RealTime)
	state3 := MakeStartChessState(id3, RealTime)
	state4 := MakeStartChessState(id4, RealTime)

	ctx := context.WithValue(context.Background(), TraceKey, "test-set-then-get")

	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id2, state2)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id3, state3)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id4, state4)
	assert.NoError(t, err)

	viewsList1, err := GetAllChessViews(ctx, rdb, 1, 2)
	assert.NoError(t, err)
	viewsList2, err := GetAllChessViews(ctx, rdb, 2, 2)
	assert.NoError(t, err)

	expectedViewList1 := []ChessView{
		{ID: "test-id4", FirstColor: Random, TimeControl: RealTime},
		{ID: "test-id3", FirstColor: Random, TimeControl: RealTime},
	}
	expectedViewList2 := []ChessView{
		{ID: "test-id2", FirstColor: Random, TimeControl: RealTime},
		{ID: "test-id1", FirstColor: Random, TimeControl: RealTime},
	}

	assert.Equal(t, expectedViewList1, viewsList1)
	assert.Equal(t, expectedViewList2, viewsList2)
}

func testSetThenGetUserViews(t *testing.T, rdb *redis.Client) {
	id1 := "test-id1"
	id2 := "test-id2"
	id3 := "test-id3"

	state1 := MakeStartChessState(id1, RealTime)
	state2 := MakeStartChessState(id2, RealTime)
	state3 := MakeStartChessState(id3, RealTime)

	state1.WhitePlayer = &PlayerState{ID: 1}
	state1.BlackPlayer = &PlayerState{ID: 2}
	state2.BlackPlayer = &PlayerState{ID: 1}

	ctx := context.WithValue(context.Background(), TraceKey, "test-set-then-get-user")

	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id2, state2)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id3, state3)
	assert.NoError(t, err)

	viewsList1, err := GetUserChessViews(ctx, rdb, 1)
	assert.NoError(t, err)
	viewsList2, err := GetUserChessViews(ctx, rdb, 2)
	assert.NoError(t, err)
	viewsList3, err := GetUserChessViews(ctx, rdb, 3)
	assert.NoError(t, err)

	expectedViewList1 := []ChessView{
		{ID: "test-id2", BlackPlayer: &PlayerState{ID: 1}, FirstColor: Random, TimeControl: RealTime},
		{ID: "test-id1", WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: Random, TimeControl: RealTime},
	}
	expectedViewList2 := []ChessView{
		{ID: "test-id1", WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: Random, TimeControl: RealTime},
	}

	assert.Equal(t, expectedViewList1, viewsList1)
	assert.Equal(t, expectedViewList2, viewsList2)
	assert.Empty(t, viewsList3)
}

func testSessions(t *testing.T, rdb *redis.Client) {
	player1 := PlayerState{ID: 1, Name: "test-name1"}

	ctx := context.WithValue(context.Background(), TraceKey, "test-sessions")

	err := SetSession(ctx, rdb, "session1", player1, 100*time.Second)
	assert.NoError(t, err)

	player3, err := GetSession(ctx, rdb, "session1")
	assert.NoError(t, err)

	assert.Equal(t, player1, player3)
}

func testLeaderboard(t *testing.T, rdb *redis.Client) {
	ctx := context.WithValue(context.Background(), TraceKey, "test-leaderboard")

	assert.NoError(t, IncrLeaderboardUser(ctx, rdb, 10, 1500))
	assert.NoError(t, IncrLeaderboardUser(ctx, rdb, 20, 1000))
	assert.NoError(t, IncrLeaderboardUser(ctx, rdb, 30, 950))
	assert.NoError(t, IncrLeaderboardUser(ctx, rdb, 40, 835))

	rank1, err := GetLeaderboardRank(ctx, rdb, 10)
	assert.NoError(t, err)
	rank2, err := GetLeaderboardRank(ctx, rdb, 20)
	assert.NoError(t, err)
	rank3, err := GetLeaderboardRank(ctx, rdb, 30)
	assert.NoError(t, err)
	rank4, err := GetLeaderboardRank(ctx, rdb, 40)
	assert.NoError(t, err)

	assert.Equal(t, 1, rank1)
	assert.Equal(t, 2, rank2)
	assert.Equal(t, 3, rank3)
	assert.Equal(t, 4, rank4)

	leaderboard1, err := GetLeaderboard(ctx, rdb, 0, 4)
	assert.NoError(t, err)

	assert.NoError(t, IncrLeaderboardUser(ctx, rdb, 20, 30))

	leaderboard2, err := GetLeaderboard(ctx, rdb, 1, 2)
	assert.NoError(t, err)

	expectedLeaderboard1 := Leaderboard{
		Users:     []RankedUser{{ID: 10, Rank: 1}, {ID: 20, Rank: 2}, {ID: 30, Rank: 3}, {ID: 40, Rank: 4}},
		PageCount: 1,
	}

	expectedLeaderboard2 := Leaderboard{
		Users:     []RankedUser{{ID: 20, Rank: 2}, {ID: 30, Rank: 3}},
		PageCount: 2,
	}

	assert.Equal(t, expectedLeaderboard1, leaderboard1)
	assert.Equal(t, expectedLeaderboard2, leaderboard2)
}
