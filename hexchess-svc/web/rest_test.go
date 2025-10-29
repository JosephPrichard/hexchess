package web

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/data"
	"hexchess-svc/util"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRegister(t *testing.T) {
	for i, test := range []struct {
		body        string
		success     bool
		successResp SessionView
		failResp    ServiceView
		expStatus   int
	}{
		{
			body:        `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			success:     true,
			successResp: SessionView{ID: 6, Username: "test-name", Country: "us", Elo: 1000},
			expStatus:   200,
		},
		{
			body:      `{"username": "test-name1", "password": "test-password1", "confirmPassword": "wrong"}`,
			failResp:  ServiceView{Status: 400, Message: ErrHttpConfirmPassword.Error()},
			expStatus: 400,
		},
		{
			body:      fmt.Sprintf(`{"username": "%s", "password": "test-password2", "confirmPassword": "test-password2"}`, data.TestUsersInsts[0].Username),
			failResp:  ServiceView{Status: 409, Message: ErrHttpDuplicateUsername.Error()},
			expStatus: 409,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			stores, closer := data.BeforeStoresTests(t)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if !test.success {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleLogin(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()
	user := data.TestUsersInsts[0]

	for i, test := range []struct {
		body        string
		success     bool
		successResp SessionView
		failResp    ServiceView
		expStatus   int
	}{
		{
			body:      `{"username": "test-name", "password": "test-password"}`,
			failResp:  ServiceView{Status: 401, Message: ErrHttpInvalidLogin.Error()},
			expStatus: 401,
		},
		{
			body:        fmt.Sprintf(`{"username": "%s", "password": "%s"}`, user.Username, user.Password),
			success:     true,
			successResp: SessionView{ID: 1, Username: user.Username, Country: "us", Elo: 1000},
			expStatus:   200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if !test.success {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()
	createTestSessions(t, stores.Rdb)

	body := `{"newUsername": "new-username", "newBio": "test biography", "newCountry": "eu"}`
	expResp := SessionView{ID: 1, Username: "new-username", Country: "eu", Elo: 1000}

	r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID1))
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(stores, nil), "")
	h.ServeHTTP(w, r)

	util.AssertRespBody[SessionView](t, expResp, w, SessionViewCmpOpts)
	assert.Equal(t, 200, w.Code)
}

func TestHandleUpdatePassword(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()
	createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		body        string
		success     bool
		successResp ServiceView
		failResp    ServiceView
		expStatus   int
	}{
		{
			body:      `{"password": "password2", "newPassword": "test-password", "confirmNewPassword": "test-password"}`,
			failResp:  ServiceView{Status: 401, Message: ErrHttpInvalidLogin.Error()},
			expStatus: 401,
		},
		{
			body:      `{"password": "password1", "newPassword": "test-password1", "confirmNewPassword": "test-password"}`,
			failResp:  ServiceView{Status: 400, Message: ErrHttpConfirmPassword.Error()},
			expStatus: 400,
		},
		{
			body:        `{"password": "password1", "newPassword": "test-password", "confirmNewPassword": "test-password"}`,
			success:     true,
			successResp: ServiceView{Status: 200, Message: "SUCCESS"},
			expStatus:   200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/users/password", strings.NewReader(test.body))
			r.Header.Set("Cookie", FmtCookie(sessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if !test.success {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()
	createTestSessions(t, stores.Rdb)

	body := `{"challengeeID": 4, "startColor": "WHITE", "timeControl": "REAL_TIME"}`
	expResp := ServiceView{Status: 200, Message: "SUCCESS"}

	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(sessionID1))
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(stores, nil), "")
	h.ServeHTTP(w, r)

	util.AssertRespBody[ServiceView](t, expResp, w)
	assert.Equal(t, 200, w.Code)
}

func TestGetLeaderboard(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "setup-get-leaderboard")
	assert.NoError(t, data.SetLeaderboard(ctx, stores.Rdb, data.UpdtLbChangeSet{ID: 1, EloDiff: 1000}))

	r := httptest.NewRequest(http.MethodGet, "/api/leaderboard", nil)
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(stores, nil), "")
	h.ServeHTTP(w, r)

	assert.Equal(t, 200, w.Code)
}

func TestGetPlayer(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	for i, test := range []struct {
		id          string
		success     bool
		successResp FullUserResp
		failResp    ServiceView
		expStatus   int
	}{
		{
			id:      "1",
			success: true,
			successResp: FullUserResp{
				User:       data.TestUserEntities[0],
				ReplayList: []data.ReplayEntity{data.TestReplayEntities[0], data.TestReplayEntities[1]},
			},
			expStatus: 200,
		},
		{
			id:        "test",
			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
			expStatus: 400,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s", test.id), nil)
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.success {
				util.AssertRespBody[FullUserResp](t, test.successResp, w, data.UserEntityCmpOpts, data.ReplayEntityCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestGetChallenges(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()
	createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		participants string
		success      bool
		successResp  GetChallengesResp
		failResp     ServiceView
		expStatus    int
	}{
		{
			participants: "sent",
			success:      true,
			successResp: GetChallengesResp{
				ChallengeList: []data.ChallengeEntity{data.TestChallengeEntities[0]},
			},
			expStatus: 200,
		},
		{
			participants: "received",
			success:      true,
			successResp: GetChallengesResp{
				ChallengeList: []data.ChallengeEntity{data.TestChallengeEntities[1]},
			},
			expStatus: 200,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(sessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.success {
				util.AssertRespBody[GetChallengesResp](t, test.successResp, w, data.ChallengeEntityCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleGetChessViews(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()
	createTestSessions(t, stores.Rdb)
	createTestChessStates(t, stores.Rdb)

	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
	r.Header.Set("Cookie", FmtCookie(sessionID2))
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(stores, nil), "")
	h.ServeHTTP(w, r)

	expResp := ChessRoomListResp{
		ChessList: []data.ChessMeta{
			{ID: "game3", FirstColor: data.Random},
			{ID: "game2", FirstColor: data.Random},
			{
				ID: "game1",
				WhitePlayer: &data.PlayerState{
					ID:      2,
					Name:    "user2",
					Country: "us",
					Elo:     1000,
				},
				FirstColor: data.Random,
			},
		},
		SelfChessList: []data.ChessMeta{
			{
				ID: "game1",
				WhitePlayer: &data.PlayerState{
					ID:      2,
					Name:    "user2",
					Country: "us",
					Elo:     1000,
				},
				FirstColor: data.Random,
			},
		},
	}

	assert.Equal(t, 200, w.Code)
	util.AssertRespBody[ChessRoomListResp](t, expResp, w)
}
