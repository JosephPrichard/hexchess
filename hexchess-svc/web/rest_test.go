package web

import (
	"context"
	"fmt"
	"hexchess-svc/data"
	"hexchess-svc/logs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetLeaderboard(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "setup-get-leaderboard")
	if err := data.SetLeaderboard(ctx, stores.Rdb, data.UpdtLbChangeSet{ID: 1, EloDiff: 1000}); err != nil {
		t.Fatalf("failed to setup leaderboard: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/leaderboard", nil)
	w := httptest.NewRecorder()

	Handle(stores).ServeHTTP(w, r)

	expResp := LeaderboardResp{
		TotalPages: 1,
		UserList:   []data.UserEntity{data.TestUserEntities[0]},
	}

	updtResp := func(b *LeaderboardResp) {
		for i := range b.UserList {
			b.UserList[i].JoinedOn = time.Time{}
		}
	}
	assertResp[LeaderboardResp](t, expResp, w, updtResp)
}

func TestGetPlayer(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	tests := []struct {
		id      string
		expResp any
	}{
		{
			id: "1",
			expResp: UserWithReplaysResp{
				User:       data.TestUserEntities[0],
				ReplayList: []data.ReplayEntity{data.TestReplayEntities[0], data.TestReplayEntities[1]},
			},
		},
		{
			id:      "test",
			expResp: ErrHttpInvalidRequest.Error(),
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s", test.id), nil)
			w := httptest.NewRecorder()

			Handle(stores).ServeHTTP(w, r)

			updtResp := func(b *UserWithReplaysResp) {
				b.User.JoinedOn = time.Time{}
				for i := range b.ReplayList {
					b.ReplayList[i].PlayedOn = time.Time{}
				}
			}
			assertResp[UserWithReplaysResp](t, test.expResp, w, updtResp)
		})
	}
}
