package web

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRegister(t *testing.T) {
	for i, test := range []struct {
		body      string
		expResp   any
		expStatus int
	}{
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
		{
			body:      `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			expResp:   SessionView{ID: 6, Username: "test-name", Country: "us", Elo: 1000},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			stores, closer := data.BeforeStoresTests(t)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()

			h := HandleRoot(MakeServerState(stores, nil, nil))
			h.ServeHTTP(w, r)

			lib.AssertRespBody[SessionView](t, test.expResp, w, SessionViewCmpOpts)
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
			expResp:   SessionView{ID: 1, Username: testUser.Username, Country: "us", Elo: 1000},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(test.body))
			w := httptest.NewRecorder()

			h := HandleRoot(MakeServerState(stores, nil, nil))
			h.ServeHTTP(w, r)

			lib.AssertRespBody[SessionView](t, test.expResp, w, SessionViewCmpOpts)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := createTestSessions(t, stores.Rdb)

	body := `{"newUsername": "new-username", "newBio": "test biography", "newCountry": "eu"}`

	r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID))

	w := httptest.NewRecorder()

	h := HandleRoot(MakeServerState(stores, nil, nil))
	h.ServeHTTP(w, r)

	expResp := SessionView{ID: 1, Username: "new-username", Country: "eu", Elo: 1000}
	lib.AssertRespBody[SessionView](t, expResp, w, SessionViewCmpOpts)
	assert.Equal(t, 200, w.Code)
}

func TestHandleUpdatePassword(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := createTestSessions(t, stores.Rdb)

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

			h := HandleRoot(MakeServerState(stores, nil, nil))
			h.ServeHTTP(w, r)

			lib.AssertRespBody[ServiceView](t, test.expResp, w)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := createTestSessions(t, stores.Rdb)

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

			h := HandleRoot(MakeServerState(stores, nil, nil))
			h.ServeHTTP(w, r)

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

	sessionID := createTestSessions(t, stores.Rdb)

	body := `{"challengeeID": 4, "firstColor": "WHITE", "timeControl": "REAL_TIME"}`

	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID))
	w := httptest.NewRecorder()

	h := HandleRoot(MakeServerState(stores, nil, nil))
	h.ServeHTTP(w, r)

	lib.AssertRespBody[ServiceView](t, ServiceView{Status: 200, Message: "SUCCESS"}, w)
	assert.Equal(t, 200, w.Code)
}

func TestGetLeaderboard(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), lib.TraceKey, "setup-get-leaderboard")
	if err := data.SetLeaderboard(ctx, stores.Rdb, data.UpdtLbChangeSet{ID: 1, EloDiff: 1000}); err != nil {
		t.Fatalf("failed to setup leaderboard: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/leaderboard", nil)
	w := httptest.NewRecorder()

	h := HandleRoot(MakeServerState(stores, nil, nil))
	h.ServeHTTP(w, r)

	expResp := LeaderboardResp{
		TotalPages: 1,
		UserList:   []data.UserEntity{data.TestUserEntities[0]},
	}

	lib.AssertRespBody[LeaderboardResp](t, expResp, w, data.UserEntityCmpOpts)
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

			h := HandleRoot(MakeServerState(stores, nil, nil))
			h.ServeHTTP(w, r)

			lib.AssertRespBody[UserWithReplaysResp](t, test.expResp, w, data.UserEntityCmpOpts, data.ReplayEntityCmpOpts)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestGetChallenges(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		participants string
		expResp      any
		expStatus    int
	}{
		{
			participants: "sent",
			expResp: GetChallengesResp{
				ChallengeList: []data.ChallengeEntity{data.TestChallengeEntities[0]},
			},
			expStatus: 200,
		},
		{
			participants: "received",
			expResp: GetChallengesResp{
				ChallengeList: []data.ChallengeEntity{data.TestChallengeEntities[1]},
			},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(sessionID))
			w := httptest.NewRecorder()

			h := HandleRoot(MakeServerState(stores, nil, nil))
			h.ServeHTTP(w, r)

			lib.AssertRespBody[GetChallengesResp](t, test.expResp, w, data.ChallengeEntityCmpOpts)
			assert.Equal(t, test.expStatus, w.Code)
		})
	}
}

func TestHandleGetChessViews(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	sessionID := createTestSessions(t, stores.Rdb)
	createTestChessStates(t, stores.Rdb)

	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
	r.Header.Set("Cookie", FmtCookie(sessionID))
	w := httptest.NewRecorder()

	h := HandleRoot(MakeServerState(stores, nil, nil))
	h.ServeHTTP(w, r)

	meta3 := data.ChessMeta{ID: "game3", FirstColor: data.Random}
	meta2 := data.ChessMeta{ID: "game2", FirstColor: data.Random}
	meta1 := data.ChessMeta{
		ID: "game1",
		BlackPlayer: &data.PlayerState{
			ID:      1,
			Name:    "username",
			Country: "us",
			Elo:     1000,
		},
		FirstColor: data.Random,
	}
	expResp := ChessRoomListResp{
		ChessList:     []data.ChessMeta{meta3, meta2, meta1},
		SelfChessList: []data.ChessMeta{meta1},
	}
	lib.AssertRespBody[ChessRoomListResp](t, expResp, w)
	assert.Equal(t, 200, w.Code)
}
