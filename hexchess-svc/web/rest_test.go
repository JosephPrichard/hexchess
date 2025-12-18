package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHandleRegister(t *testing.T) {
	nextID := svc.LastUserID + 1
	insertTime := svc.TestTimeNow

	for i, test := range []struct {
		body        string
		successResp SessionView
		failResp    ServiceView
		expStatus   int
		dbUser      *db.SelectUserByIDRow
	}{
		{
			body:        `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			successResp: SessionView{ID: nextID, Username: "test-name", Country: "us", Elo: 1000},
			expStatus:   http.StatusOK,
			dbUser: &db.SelectUserByIDRow{
				ID:         nextID,
				Username:   "test-name",
				Country:    pgtype.Text{String: "us", Valid: true},
				Elo:        1000,
				HighestElo: 1000,
				Wins:       0,
				Losses:     0,
				Bio:        "",
				JoinedOn:   pgtype.Timestamptz{Time: insertTime.Local(), Valid: true},
			},
		},
		{
			body:      `{"username": "test-name1", "password": "test-password1", "confirmPassword": "wrong"}`,
			failResp:  ServiceView{Status: 400, Message: ErrHttpConfirmPassword.Error()},
			expStatus: 400,
		},
		{
			body:      fmt.Sprintf(`{"username": "%s", "password": "test-password2", "confirmPassword": "test-password2"}`, svc.TestUsersInsts[0].Username),
			failResp:  ServiceView{Status: 400, Message: ErrHttpDuplicateUsername.Error()},
			expStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: insertTime}}), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}

			if test.dbUser != nil {
				row, err := dbs.Pdb.Query.SelectUserByID(t.Context(), test.dbUser.ID)
				if err != nil {
					t.Fatalf("failed to query user in assert: %v", err)
				}
				assert.Equal(t, *test.dbUser, row)
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

func TestHandleLoginGoogleLogin(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	for i, test := range []struct {
		runCount    int
		setupMocks  func(ctrl *gomock.Controller) outbound.GoogleAPI
		body        string
		successResp SessionView
		failResp    ServiceView
		expStatus   int
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
			body:      `{"token": "invalidToken123"}`,
			failResp:  ServiceView{Status: 500, Message: ErrHttpFatal.Error()},
			expStatus: 500,
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
			body:        `{"token": "testToken123"}`,
			successResp: SessionView{ID: svc.LastUserID + 1, Username: "email@domain.com", Country: "us", Elo: 1000},
			expStatus:   http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			for range test.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", strings.NewReader(test.body))
				w := httptest.NewRecorder()

				state := MakeServerState(ServerSetup{
					Databases:    dbs,
					OutboundAPIs: &outbound.OutboundAPIs{GoogleAPI: test.setupMocks(ctrl)}})

				h := HandleRoot(state, "")

				h.ServeHTTP(w, r)

				assert.Equal(t, test.expStatus, w.Code)
				if test.expStatus == http.StatusOK {
					util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
				} else {
					util.AssertRespBody[ServiceView](t, test.failResp, w)
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
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

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
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	createTestSessions(t, dbs.Rdb)

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

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

			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus != http.StatusOK {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	// given
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()
	createTestSessions(t, dbs.Rdb)

	body := `{"challengeeID": 4, "startColor": "WHITE", "timeControl": "REAL_TIME"}`
	expResp := ServiceView{Status: http.StatusOK, Message: "SUCCESS"}

	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", strings.NewReader(body))
	r.Header.Set("Cookie", FmtCookie(TestSessionID1))
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	// when
	h.ServeHTTP(w, r)

	// then
	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[ServiceView](t, expResp, w)
}

func TestGetLeaderboard(t *testing.T) {
	// given
	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "setup-get-leaderboard")
	assert.NoError(t, svc.SetLeaderboard(ctx, dbs.Rdb, svc.UpdtLbChangeSet{ID: 1, EloDiff: 1000}))

	r := httptest.NewRequest(http.MethodGet, "/api/leaderboard", nil)
	w := httptest.NewRecorder()
	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	// when
	h.ServeHTTP(w, r)

	// then
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetPlayer(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
	defer closer()

	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")

	for i, test := range []struct {
		id          string
		successResp FullUserResp
		failResp    ServiceView
		expStatus   int
	}{
		{
			id: "1",
			successResp: FullUserResp{
				User:       svc.TestUserEntities[0],
				ReplayList: []svc.ReplayEntity{svc.TestReplayEntities[0], svc.TestReplayEntities[1]},
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

			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if test.expStatus == http.StatusOK {
				util.AssertRespBody[FullUserResp](t, test.successResp, w)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
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
		successResp  GetChallengesResp
		expStatus    int
	}{
		{
			participants: "sent",
			successResp: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[0]},
			},
			expStatus: http.StatusOK,
		},
		{
			participants: "received",
			successResp: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[1]},
			},
			expStatus: http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			util.AssertRespBody[GetChallengesResp](t, test.successResp, w)
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
		successResp GetUserReplaysResp
		failResp    ServiceView
		expStatus   int
	}{
		{
			afterID:     "0",
			userID:      "999", // nonexistent user
			expStatus:   http.StatusOK,
			successResp: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{}},
		},
		{
			afterID:   "-1",
			userID:    "1",
			expStatus: http.StatusOK,
			successResp: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{
				svc.TestReplayEntities[0],
				svc.TestReplayEntities[1],
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
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?afterId=%s&userId=%s", test.afterID, test.userID), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.expStatus, w.Code)
			if w.Code == http.StatusOK {
				util.AssertRespBody[GetUserReplaysResp](t, test.successResp, w)
			} else {
				util.AssertRespBody[ServiceView](t, test.failResp, w)
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

	expResp := ChessRoomListResp{
		ChessList: []svc.ChessMeta{
			{ID: "game3", FirstColor: svc.ColorRandom, TimeControl: svc.TcRealTime},
			{ID: "game2", FirstColor: svc.ColorRandom, TimeControl: svc.TcRealTime},
			{
				ID:          "game1",
				WhitePlayer: svc.MakePlayer(2, "user2", "us", 1000),
				FirstColor:  svc.ColorRandom,
				TimeControl: svc.TcRealTime,
			},
		},
		SelfChessList: []svc.ChessMeta{
			{
				ID:          "game1",
				WhitePlayer: svc.MakePlayer(2, "user2", "us", 1000),
				FirstColor:  svc.ColorRandom,
				TimeControl: svc.TcRealTime,
			},
		},
	}

	assert.Equal(t, http.StatusOK, w.Code)
	util.AssertRespBody[ChessRoomListResp](t, expResp, w)
}
