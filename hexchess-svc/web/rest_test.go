package web

import (
	"context"
	"fmt"
	"hexchess-svc/data"
	"hexchess-svc/logs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleRegister(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	for i, test := range []struct {
		body    string
		expResp any
	}{
		{
			body:    `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			expResp: SessionView{Username: "test-name", Country: "us", Elo: 1000},
		},
		{
			body:    `{"username": "test-name1", "password": "test-password1", "confirmPassword": "wrong"}`,
			expResp: ErrHttpConfirmPassword.Error(),
		},
		{
			body:    fmt.Sprintf(`{"username": "%s", "password": "test-password2", "confirmPassword": "test-password2"}`, data.TestUsersInsts[0].Username),
			expResp: ErrHttpDuplicateUsername.Error(),
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertRespUpdt[SessionView](t, test.expResp, w, func(b *SessionView) {
				b.ID = 0
				b.TTLSecs = 0
			})
		})
	}
}

func TestHandleLogin(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	testUser := data.TestUsersInsts[0]

	for i, test := range []struct {
		body    string
		expResp any
	}{
		{
			body:    `{"username": "test-name", "password": "test-password"}`,
			expResp: ErrHttpUserNotFound.Error(),
		},
		{
			body:    fmt.Sprintf(`{"username": "%s", "password": "%s"}`, testUser.Username, testUser.Password),
			expResp: SessionView{Username: testUser.Username, Country: "us", Elo: 1000},
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(test.body))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertRespUpdt[SessionView](t, test.expResp, w, func(b *SessionView) {
				b.ID = 0
				b.TTLSecs = 0
			})
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := "test-session-id"

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-update-user")
	if err := data.SetSession(ctx, stores.Rdb, sessionID, data.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, MaxAgeCookie); err != nil {
		t.Fatalf("failed to init session: %v", err)
	}

	body := `{"newUsername": "new-username", "newPassword": "new-password", "newCountry": "eu"}`
	expResp := SessionView{Username: "new-username", Country: "eu", Elo: 1000}

	r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID))

	w := httptest.NewRecorder()

	Handle(RestState{Stores: stores}).ServeHTTP(w, r)

	assertRespUpdt[SessionView](t, expResp, w, func(b *SessionView) {
		b.ID = 0
		b.TTLSecs = 0
	})
}

func TestGetLeaderboard(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "setup-get-leaderboard")
	if err := data.SetLeaderboard(ctx, stores.Rdb, data.UpdtLbChangeSet{ID: 1, EloDiff: 1000}); err != nil {
		t.Fatalf("failed to setup leaderboard: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/leaderboard", nil)
	w := httptest.NewRecorder()

	Handle(RestState{Stores: stores}).ServeHTTP(w, r)

	expResp := LeaderboardResp{
		TotalPages: 1,
		UserList:   []data.UserEntity{data.TestUserEntities[0]},
	}

	assertRespUpdt[LeaderboardResp](t, expResp, w, func(b *LeaderboardResp) {
		for i := range b.UserList {
			b.UserList[i].JoinedOn = time.Time{}
		}
	})
}

func TestGetPlayer(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	for i, test := range []struct {
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
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s", test.id), nil)
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertRespUpdt[UserWithReplaysResp](t, test.expResp, w, func(b *UserWithReplaysResp) {
				b.User.JoinedOn = time.Time{}
				for i := range b.ReplayList {
					b.ReplayList[i].PlayedOn = time.Time{}
				}
			})
		})
	}
}
