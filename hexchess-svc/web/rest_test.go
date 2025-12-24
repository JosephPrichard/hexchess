package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestHandleRegister(t *testing.T) {
	insertTime := svc.TestTimeNow

	for i, test := range []struct {
		body        RegisterBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			body:        RegisterBody{Username: "test-name", Password: "test-password", ConfirmPassword: "test-password"},
			wantSuccess: SessionView{Username: "test-name", Country: "un"},
			wantStatus:  http.StatusOK,
		},
		{
			body:       RegisterBody{Username: "test-name1", Password: "test-password1", ConfirmPassword: "wrong"},
			wantFail:   ServiceView{Status: 400, Errors: map[string]any{"confirmPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: 400,
		},
		{
			body:       RegisterBody{Username: svc.TestUsersInsts[0].Username, Password: "test-password2", ConfirmPassword: "test-password2"},
			wantFail:   ServiceView{Status: 400, Errors: ErrHttpDuplicateUsername.Error()},
			wantStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", util.AsJSONReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: insertTime}}), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleLogin(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	user := svc.TestUsersInsts[0]

	for i, test := range []struct {
		body        LoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			body:       LoginBody{Username: "test-name", Password: "test-password"},
			wantFail:   ServiceView{Status: 401, Errors: ErrHttpInvalidLogin.Error()},
			wantStatus: 401,
		},
		{
			body:        LoginBody{Username: user.Username, Password: user.Password},
			wantSuccess: SessionView{Username: user.Username, Country: "us"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", util.AsJSONReader(test.body))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleLoginGoogleLogin(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	for i, test := range []struct {
		runCount    int
		setupMocks  func(ctrl *gomock.Controller) outbound.GoogleAPI
		body        GoogleLoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			runCount: 1,
			setupMocks: func(ctrl *gomock.Controller) outbound.GoogleAPI {
				m := outbound.NewMockGoogleAPI(ctrl)
				m.EXPECT().
					ValidateIDToken(gomock.Any(), "invalidToken123").
					Return(outbound.GoogleIDTokenPayload{}, errors.New("invalid token"))
				return m
			},
			body:       GoogleLoginBody{Token: "invalidToken123"},
			wantFail:   ServiceView{Status: 500, Errors: ErrHttpFatal.Error()},
			wantStatus: 500,
		},
		{
			runCount: 2, // the user is created the first time, the second time we log in with the already inserted account ID
			setupMocks: func(ctrl *gomock.Controller) outbound.GoogleAPI {
				m := outbound.NewMockGoogleAPI(ctrl)
				m.EXPECT().
					ValidateIDToken(gomock.Any(), "testToken123").
					Return(outbound.GoogleIDTokenPayload{AccountID: "account1", Username: "email@domain.com"}, nil)
				return m
			},
			body:        GoogleLoginBody{Token: "testToken123"},
			wantSuccess: SessionView{Username: "email@domain.com", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			for range test.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", util.AsJSONReader(test.body))
				w := httptest.NewRecorder()

				state := MakeServerState(ServerSetup{
					Databases: dbs,
					OutboundAPIs: outbound.APIs{
						GoogleAPI: test.setupMocks(ctrl),
					},
				})

				h := HandleRoot(state, "")
				h.ServeHTTP(w, r)

				assert.Equal(t, test.wantStatus, w.Code)
				if test.wantStatus == http.StatusOK {
					util.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
				} else {
					util.AssertRespBody[ServiceView](t, test.wantFail, w)
				}
			}
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, CountryList: []string{"eu"}}), "")

	for i, test := range []struct {
		body        UpdateUserBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			body:       UpdateUserBody{NewCountry: "test"},
			wantFail:   ServiceView{Status: 400, Errors: map[string]any{"newCountry": ErrHttpInvalidCountry.Error()}},
			wantStatus: 400,
		},
		{
			body:        UpdateUserBody{NewUsername: "new-username", NewBio: "test biography", NewCountry: "eu"},
			wantSuccess: SessionView{Username: "new-username", Country: "eu"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/users", util.AsJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleUpdatePassword(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	for i, test := range []struct {
		body        UpdatePasswordBody
		wantSuccess ServiceView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			body:       UpdatePasswordBody{Password: "password2", NewPassword: "test-password", ConfirmNewPassword: "test-password"},
			wantFail:   ServiceView{Status: 401, Errors: ErrHttpInvalidLogin.Error()},
			wantStatus: 401,
		},
		{
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "test-password1", ConfirmNewPassword: "test-password"},
			wantFail:   ServiceView{Status: 400, Errors: map[string]any{"confirmNewPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: 400,
		},
		{
			body:        UpdatePasswordBody{Password: "password1", NewPassword: "test-password", ConfirmNewPassword: "test-password"},
			wantSuccess: ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/users/password", util.AsJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				util.AssertRespBody[ServiceView](t, test.wantSuccess, w)
			} else {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	createTestSessions(t, dbs.Rdb)

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	for i, test := range []struct {
		body       UpdateChallengeBody
		wantFail   ServiceView
		wantStatus int
	}{
		{
			body:       UpdateChallengeBody{Action: "invalid"},
			wantFail:   ServiceView{Status: 400, Errors: map[string]any{"action": ErrHttpInvalidAction.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			body:       UpdateChallengeBody{ChallengerID: 999, ChallengeeID: 1, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: 404, Errors: ErrHttpNotFoundChallenge.Error()},
			wantStatus: 404,
		},
		{
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "DELETE"},
			wantFail:   ServiceView{Status: 400, Errors: ErrHttpUpdateChallenge.Error()},
			wantStatus: 400,
		},
		{
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "ACCEPT"},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", util.AsJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus != http.StatusOK {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	// given
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)

	body := CreateChallengeBody{ChallengeeID: 4, StartColor: "WHITE", Mode: "CORRESPONDENCE_1"}
	wantSuccess := ServiceView{Status: http.StatusOK, Message: "SUCCESS"}

	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", util.AsJSONReader(body))
	r.Header.Set("Cookie", FmtCookie(TestSessionID1))
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: svc.TestTimeNow}}), "")

	// when
	h.ServeHTTP(w, r)

	// then
	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[ServiceView](t, wantSuccess, w)
}

func TestGetLeaderboard(t *testing.T) {
	// given
	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "setup-get-leaderboard")
	require.NoError(t, svc.SetLeaderboard(ctx, dbs.Rdb, svc.UpdtLbChangeSet{Mode: svc.ModeTimed1Plus0, ID: 1, EloDiff: 1000}))

	q := url.Values{}
	q.Set("mode", string(svc.ModeTimed1Plus0))
	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leaderboard?%s", q.Encode()), nil)
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	// when
	h.ServeHTTP(w, r)

	// then
	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[LeaderboardResp](t, LeaderboardResp{
		TotalPages: 1,
		UserList: []svc.LbdUserEntity{
			{
				UserEntity: svc.UserEntity{
					ID:       1,
					Username: "user1",
					Country:  "us",
					JoinedOn: svc.TestTimeNow,
				},
				Elo:        1000,
				HighestElo: 1000,
				Wins:       5,
				Losses:     5,
				Winrate:    50,
				Rank:       1,
			},
		},
	}, w)
}

func TestGetPlayer(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
	defer closer()

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	for i, test := range []struct {
		id          string
		withReplays bool
		wantSuccess FullUserResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			id:          "1",
			withReplays: true,
			wantSuccess: FullUserResp{
				User:       svc.TestUserEntities[0],
				Stats:      svc.TestUserStats[0],
				ReplayList: []svc.ReplayEntity{svc.TestReplayEntities[0], svc.TestReplayEntities[1]},
			},
			wantStatus: http.StatusOK,
		},
		{
			id:          "1",
			withReplays: false,
			wantSuccess: FullUserResp{
				User:       svc.TestUserEntities[0],
				Stats:      svc.TestUserStats[0],
				ReplayList: []svc.ReplayEntity{},
			},
			wantStatus: http.StatusOK,
		},
		{
			id:         "test",
			wantFail:   ServiceView{Status: 400, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s&withReplays=%v", test.id, test.withReplays), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				util.AssertRespBody[FullUserResp](t, test.wantSuccess, w)
			} else {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestGetChallenges(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: svc.TestTimeNow}}), "")

	for i, test := range []struct {
		participants string
		wantSuccess  GetChallengesResp
		wantStatus   int
	}{
		{
			participants: "sent",
			wantSuccess: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			participants: "received",
			wantSuccess: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[1]},
			},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			util.AssertRespBody[GetChallengesResp](t, test.wantSuccess, w)
		})
	}
}

func TestHandleGetUserReplays(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	for i, test := range []struct {
		afterID     string
		userID      string
		wantSuccess GetUserReplaysResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			afterID:     "0",
			userID:      "999", // nonexistent user
			wantStatus:  http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{}},
		},
		{
			afterID:    "-1",
			userID:     "1",
			wantStatus: http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{
				svc.TestReplayEntities[0],
				svc.TestReplayEntities[1],
			}},
		},
		{
			afterID:    "abc", // invalid afterID
			userID:     "xyz", // invalid userID
			wantFail:   ServiceView{Status: 400, Errors: map[string]any{"afterId": ErrHttpInvalidID.Error(), "userId": ErrHttpInvalidID.Error()}},
			wantStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?afterId=%s&userId=%s", test.afterID, test.userID), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				util.AssertRespBody[GetUserReplaysResp](t, test.wantSuccess, w)
			} else {
				util.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGetChessViews(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)
	createTestChessStates(t, dbs.Rdb)

	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
	w := httptest.NewRecorder()

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
	h.ServeHTTP(w, r)

	wantResp := ChessRoomListResp{
		ChessList: []svc.ChessMeta{
			{ID: "game3", FirstColor: svc.ColorRandom, Mode: svc.ModeCorrespondence1},
			{ID: "game2", FirstColor: svc.ColorRandom, Mode: svc.ModeCorrespondence1},
			{
				ID:          "game1",
				WhitePlayer: svc.MakePlayer(2, "user2", "us"),
				FirstColor:  svc.ColorRandom,
				Mode:        svc.ModeCorrespondence1,
			},
		},
		SelfChessList: []svc.ChessMeta{
			{
				ID:          "game1",
				WhitePlayer: svc.MakePlayer(2, "user2", "us"),
				FirstColor:  svc.ColorRandom,
				Mode:        svc.ModeCorrespondence1,
			},
		},
	}
	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[ChessRoomListResp](t, wantResp, w)
}
