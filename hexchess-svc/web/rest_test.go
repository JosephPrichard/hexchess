package web

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/data"
	"hexchess-svc/logs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func clearSessionView(b *SessionView) {
	b.ID = 0
	b.TTLSecs = 0
}

func initSession(t *testing.T, rdb data.Rdb) string {
	sessionID := "test-session-id"
	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-update-password")
	if err := data.SetSession(ctx, rdb, sessionID, data.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, MaxAgeCookie); err != nil {
		t.Fatalf("failed to init session: %v", err)
	}
	return sessionID
}

func TestHandleRegister(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	for i, test := range []struct {
		body      string
		expResp   any
		expStatus int
	}{
		{
			body:      `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			expResp:   SessionView{Username: "test-name", Country: "us", Elo: 1000},
			expStatus: 200,
		},
		{
			body:      `{"username": "test-name1", "password": "test-password1", "confirmPassword": "wrong"}`,
			expResp:   ErrHttpConfirmPassword.Error(),
			expStatus: 400,
		},
		{
			body:      fmt.Sprintf(`{"username": "%s", "password": "test-password2", "confirmPassword": "test-password2"}`, data.TestUsersInsts[0].Username),
			expResp:   ErrHttpDuplicateUsername.Error(),
			expStatus: 409,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertRespUpdt[SessionView](t, test.expResp, w, clearSessionView)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleLogin(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	testUser := data.TestUsersInsts[0]

	for i, test := range []struct {
		body      string
		expResp   any
		expStatus int
	}{
		{
			body:      `{"username": "test-name", "password": "test-password"}`,
			expResp:   ErrHttpInvalidLogin.Error(),
			expStatus: 401,
		},
		{
			body:      fmt.Sprintf(`{"username": "%s", "password": "%s"}`, testUser.Username, testUser.Password),
			expResp:   SessionView{Username: testUser.Username, Country: "us", Elo: 1000},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(test.body))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertRespUpdt[SessionView](t, test.expResp, w, clearSessionView)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := initSession(t, stores.Rdb)

	body := `{"newUsername": "new-username", "newBio": "test biography", "newCountry": "eu"}`
	expResp := SessionView{Username: "new-username", Country: "eu", Elo: 1000}

	r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID))

	w := httptest.NewRecorder()

	Handle(RestState{Stores: stores}).ServeHTTP(w, r)

	assertRespUpdt(t, expResp, w, clearSessionView)
	assert.Equal(t, 200, w.Code)
}

func TestHandleUpdatePassword(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := initSession(t, stores.Rdb)

	for i, test := range []struct {
		body      string
		expResp   any
		expStatus int
	}{
		{
			body:      `{"password": "password2", "newPassword": "test-password", "confirmNewPassword": "test-password"}`,
			expResp:   ErrHttpInvalidLogin.Error(),
			expStatus: 401,
		},
		{
			body:      `{"password": "password1", "newPassword": "test-password1", "confirmNewPassword": "test-password"}`,
			expResp:   ErrHttpConfirmPassword.Error(),
			expStatus: 400,
		},
		{
			body:      `{"password": "password1", "newPassword": "test-password", "confirmNewPassword": "test-password"}`,
			expResp:   ServiceView{Status: 200, Message: "SUCCESS"},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/users/password", strings.NewReader(test.body))
			r.Header.Set("Cookie", FmtCookie(sessionID))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertResp[ServiceView](t, test.expResp, w)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := initSession(t, stores.Rdb)

	for i, test := range []struct {
		body      string
		expResp   string
		expStatus int
	}{
		{
			body:      `{"challengerID": 999, "challengeeID": 1, "action": "ACCEPT"}`,
			expResp:   ErrHttpNotFoundChallenge.Error(),
			expStatus: 404,
		},
		{
			body:      `{"challengerID": 5, "challengeeID": 1, "action": "DELETE"}`,
			expResp:   ErrHttpUpdateChallenge.Error(),
			expStatus: 400,
		},
		{
			body:      `{"challengerID": 5, "challengeeID": 1, "action": "ACCEPT"}`,
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", strings.NewReader(test.body))
			r.Header.Set("Cookie", FmtCookie(sessionID))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			if test.expResp != "" {
				assert.Equal(t, test.expResp, w.Body.String())
			}
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := initSession(t, stores.Rdb)

	body := `{"challengeeID": 4, "firstColor": "WHITE", "timeControl": "REAL_TIME"}`

	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID))
	w := httptest.NewRecorder()

	Handle(RestState{Stores: stores}).ServeHTTP(w, r)

	assertResp[ServiceView](t, ServiceView{Status: 200, Message: "SUCCESS"}, w)
	assert.Equal(t, 200, w.Code)
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
	assert.Equal(t, 200, w.Code)
}

func TestGetPlayer(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	for i, test := range []struct {
		id        string
		expResp   any
		expStatus int
	}{
		{
			id: "1",
			expResp: UserWithReplaysResp{
				User:       data.TestUserEntities[0],
				ReplayList: []data.ReplayEntity{data.TestReplayEntities[0], data.TestReplayEntities[1]},
			},
			expStatus: 200,
		},
		{
			id:        "test",
			expResp:   ErrHttpInvalidRequest.Error(),
			expStatus: 400,
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
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestGetChallenges(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := initSession(t, stores.Rdb)

	for i, test := range []struct {
		participants string
		expResp      any
		expStatus    int
	}{
		{
			participants: "sent",
			expResp: GetChallengesResp{ChallengeList: []data.ChallengeEntity{
				{
					ChallengerID:      1,
					ChallengerName:    "user1",
					ChallengerCountry: "us",
					ChallengerElo:     1000,
					ChallengeeID:      2,
					ChallengeeName:    "user2",
					ChallengeeCountry: "us",
					ChallengeeElo:     1000,
					TimeControl:       data.Unlimited,
					StartColor:        data.Random,
				},
			}},
			expStatus: 200,
		},
		{
			participants: "received",
			expResp: GetChallengesResp{ChallengeList: []data.ChallengeEntity{
				{
					ChallengerID:      3,
					ChallengerName:    "user3",
					ChallengerCountry: "us",
					ChallengerElo:     900,
					ChallengeeID:      1,
					ChallengeeName:    "user1",
					ChallengeeCountry: "us",
					ChallengeeElo:     1000,
					TimeControl:       data.Unlimited,
					StartColor:        data.Random,
				},
			}},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(sessionID))
			w := httptest.NewRecorder()

			Handle(RestState{Stores: stores}).ServeHTTP(w, r)

			assertRespUpdt[GetChallengesResp](t, test.expResp, w, func(b *GetChallengesResp) {
				for i := range b.ChallengeList {
					b.ChallengeList[i].MadeOn = time.Time{}
				}
			})
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}
