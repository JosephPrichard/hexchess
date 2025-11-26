package web

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/data"
	"hexchess-svc/util"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHandleRegister(t *testing.T) {
	for i, test := range []struct {
		body        string
		successResp SessionView
		failResp    ServiceView
		expStatus   int
	}{
		{
			body:        `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			successResp: SessionView{ID: 6, Username: "test-name", Country: "us", Elo: 1000},
			expStatus:   http.StatusOK,
		},
		{
			body:      `{"username": "test-name1", "password": "test-password1", "confirmPassword": "wrong"}`,
			failResp:  ServiceView{Status: 400, Message: ErrHttpConfirmPassword.Error()},
			expStatus: 400,
		},
		{
			body:      fmt.Sprintf(`{"username": "%s", "password": "test-password2", "confirmPassword": "test-password2"}`, data.TestUsersInsts[0].Username),
			failResp:  ServiceView{Status: 400, Message: ErrHttpDuplicateUsername.Error()},
			expStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			stores, closer := data.BeforeStoresTests(t, true)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleLogin(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()

	user := data.TestUsersInsts[0]

	for i, test := range []struct {
		body        string
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
			successResp: SessionView{ID: 1, Username: user.Username, Country: "us", Elo: 1000},
			expStatus:   http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()
	createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		body        string
		successResp SessionView
		failResp    ServiceView
		expStatus   int
	}{
		{
			body:      `{"newCountry": "test"}`,
			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidCountry.Error()},
			expStatus: 400,
		},
		{
			body:        `{"newUsername": "new-username", "newBio": "test biography", "newCountry": "eu"}`,
			successResp: SessionView{ID: 1, Username: "new-username", Country: "eu", Elo: 1000},
			expStatus:   http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, []string{"eu"}), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleUpdatePassword(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()
	createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		body        string
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
			successResp: ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
			expStatus:   http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/users/password", strings.NewReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[ServiceView](t, test.successResp, w)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()

	createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		body      string
		failResp  ServiceView
		expStatus int
	}{
		{
			body:      `{"challengerID": 999, "challengeeID": 1, "action": "ACCEPT"}`,
			failResp:  ServiceView{Status: 404, Message: ErrHttpNotFoundChallenge.Error()},
			expStatus: 404,
		},
		{
			body:      `{"challengerID": 5, "challengeeID": 1, "action": "DELETE"}`,
			failResp:  ServiceView{Status: 400, Message: ErrHttpUpdateChallenge.Error()},
			expStatus: 400,
		},
		{
			body:      `{"challengerID": 5, "challengeeID": 1, "action": "ACCEPT"}`,
			expStatus: http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", strings.NewReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus != http.StatusOK {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()
	createTestSessions(t, stores.Rdb)

	body := `{"challengeeID": 4, "startColor": "WHITE", "timeControl": "REAL_TIME"}`
	expResp := ServiceView{Status: http.StatusOK, Message: "SUCCESS"}

	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(TestSessionID1))
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(stores, nil), "")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[ServiceView](t, expResp, w)
}

func TestGetLeaderboard(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, false)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "setup-get-leaderboard")
	assert.NoError(t, data.SetLeaderboard(ctx, stores.Rdb, data.UpdtLbChangeSet{ID: 1, EloDiff: 1000}))

	r := httptest.NewRequest(http.MethodGet, "/api/leaderboard", nil)
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(stores, nil), "")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetPlayer(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, false)
	defer closer()

	for i, test := range []struct {
		id          string
		successResp FullUserResp
		failResp    ServiceView
		expStatus   int
	}{
		{
			id: "1",
			successResp: FullUserResp{
				User:       data.TestUserEntities[0],
				ReplayList: []data.ReplayEntity{data.TestReplayEntities[0], data.TestReplayEntities[1]},
			},
			expStatus: http.StatusOK,
		},
		{
			id:        "test",
			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
			expStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s", test.id), nil)
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[FullUserResp](t, test.successResp, w, data.UserEntityCmpOpts, data.ReplayEntityCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestGetChallenges(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, false)
	defer closer()
	createTestSessions(t, stores.Rdb)

	for i, test := range []struct {
		participants string
		successResp  GetChallengesResp
		expStatus    int
	}{
		{
			participants: "sent",
			successResp: GetChallengesResp{
				ChallengeList: []data.ChallengeEntity{data.TestChallengeEntities[0]},
			},
			expStatus: http.StatusOK,
		},
		{
			participants: "received",
			successResp: GetChallengesResp{
				ChallengeList: []data.ChallengeEntity{data.TestChallengeEntities[1]},
			},
			expStatus: http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			util.AssertRespBody[GetChallengesResp](t, test.successResp, w, data.ChallengeEntityCmpOpts)
		})
	}
}

func TestHandleGetUserReplays(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()

	for i, test := range []struct {
		afterID     string
		userID      string
		successResp GetUserReplaysResp
		failResp    ServiceView
		expStatus   int
	}{
		{
			afterID:     "0",
			userID:      "999", // nonexistent user
			expStatus:   http.StatusOK,
			successResp: GetUserReplaysResp{ReplayList: []data.ReplayEntity{}},
		},
		{
			afterID:   "-1",
			userID:    "1",
			expStatus: http.StatusOK,
			successResp: GetUserReplaysResp{ReplayList: []data.ReplayEntity{
				data.TestReplayEntities[0],
				data.TestReplayEntities[1],
			}},
		},
		{
			afterID:   "abc",
			userID:    "1", // invalid afterID
			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
			expStatus: 400,
		},
		{
			afterID:   "0",
			userID:    "xyz", // invalid afterID
			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
			expStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?afterID=%s&userID=%s", test.afterID, test.userID), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(stores, nil), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if w.Code == http.StatusOK {
				util.AssertRespBody[GetUserReplaysResp](t, test.successResp, w, data.ReplayEntityCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleGetChessViews(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, false)
	defer closer()
	createTestSessions(t, stores.Rdb)
	createTestChessStates(t, stores.Rdb)

	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
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
				FirstColor:  data.Random,
				TimeControl: data.Unlimited,
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
				FirstColor:  data.Random,
				TimeControl: data.Unlimited,
			},
		},
	}

	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[ChessRoomListResp](t, expResp, w)
}
