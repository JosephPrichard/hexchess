package web

import (
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
		wantStatus  int
		dbUser      *db.SelectUserByIDRow
	}{
		{
			body:        `{"username": "test-name", "password": "test-password", "confirmPassword": "test-password"}`,
			successResp: SessionView{ID: nextID, Username: "test-name", Country: "un"},
			wantStatus:  http.StatusOK,
			dbUser: &db.SelectUserByIDRow{
				ID:       nextID,
				Username: "test-name",
				Country:  "un",
				Bio:      "",
				JoinedOn: pgtype.Timestamptz{Time: insertTime.Local(), Valid: true},
			},
		},
		{
			body:       `{"username": "test-name1", "password": "test-password1", "confirmPassword": "wrong"}`,
			failResp:   ServiceView{Status: 400, Message: ErrHttpConfirmPassword.Error()},
			wantStatus: 400,
		},
		{
			body:       fmt.Sprintf(`{"username": "%s", "password": "test-password2", "confirmPassword": "test-password2"}`, svc.TestUsersInsts[0].Username),
			failResp:   ServiceView{Status: 400, Message: ErrHttpDuplicateUsername.Error()},
			wantStatus: 400,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: insertTime}}), "")
			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
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

//func TestHandleLogin(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
//	defer closer()
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//
//	user := svc.TestUsersInsts[0]
//
//	for i, test := range []struct {
//		body        string
//		successResp SessionView
//		failResp    ServiceView
//		wantStatus   int
//	}{
//		{
//			body:      `{"username": "test-name", "password": "test-password"}`,
//			failResp:  ServiceView{Status: 401, Message: ErrHttpInvalidLogin.Error()},
//			wantStatus: 401,
//		},
//		{
//			body:        fmt.Sprintf(`{"username": "%s", "password": "%s"}`, user.Username, user.Password),
//			successResp: SessionView{ID: 1, Username: user.Username, Country: "us"},
//			wantStatus:   http.StatusOK,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(test.body))
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			if test.wantStatus == http.StatusOK {
//				util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
//			} else {
//				util.AssertRespBody[ServiceView](t, test.failResp, w)
//			}
//		})
//	}
//}

func TestHandleLoginGoogleLogin(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
	defer closer()

	for i, test := range []struct {
		runCount    int
		setupMocks  func(ctrl *gomock.Controller) outbound.GoogleAPI
		body        string
		successResp SessionView
		failResp    ServiceView
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
			body:       `{"token": "invalidToken123"}`,
			failResp:   ServiceView{Status: 500, Message: ErrHttpFatal.Error()},
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
			body:        `{"token": "testToken123"}`,
			successResp: SessionView{ID: svc.LastUserID + 1, Username: "email@domain.com", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			for range test.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", strings.NewReader(test.body))
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
					util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
				} else {
					util.AssertRespBody[ServiceView](t, test.failResp, w)
				}
			}
		})
	}
}

//func TestHandleUpdateUser(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
//	defer closer()
//	createTestSessions(t, dbs.Rdb)
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, CountryList: []string{"eu"}}), "")
//
//	for i, test := range []struct {
//		body        string
//		successResp SessionView
//		failResp    ServiceView
//		wantStatus   int
//	}{
//		{
//			body:      `{"newCountry": "test"}`,
//			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidCountry.Error()},
//			wantStatus: 400,
//		},
//		{
//			body:        `{"newUsername": "new-username", "newBio": "test biography", "newCountry": "eu"}`,
//			successResp: SessionView{ID: 1, Username: "new-username", Country: "eu"},
//			wantStatus:   http.StatusOK,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(test.body))
//			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			if test.wantStatus == http.StatusOK {
//				util.AssertRespBody[SessionView](t, test.successResp, w, SessionViewCmpOpts)
//			} else {
//				util.AssertRespBody[ServiceView](t, test.failResp, w)
//			}
//		})
//	}
//}
//
//func TestHandleUpdatePassword(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
//	defer closer()
//	createTestSessions(t, dbs.Rdb)
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//
//	for i, test := range []struct {
//		body        string
//		successResp ServiceView
//		failResp    ServiceView
//		wantStatus   int
//	}{
//		{
//			body:      `{"password": "password2", "newPassword": "test-password", "confirmNewPassword": "test-password"}`,
//			failResp:  ServiceView{Status: 401, Message: ErrHttpInvalidLogin.Error()},
//			wantStatus: 401,
//		},
//		{
//			body:      `{"password": "password1", "newPassword": "test-password1", "confirmNewPassword": "test-password"}`,
//			failResp:  ServiceView{Status: 400, Message: ErrHttpConfirmPassword.Error()},
//			wantStatus: 400,
//		},
//		{
//			body:        `{"password": "password1", "newPassword": "test-password", "confirmNewPassword": "test-password"}`,
//			successResp: ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
//			wantStatus:   http.StatusOK,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodPost, "/api/users/password", strings.NewReader(test.body))
//			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			if test.wantStatus == http.StatusOK {
//				util.AssertRespBody[ServiceView](t, test.successResp, w)
//			} else {
//				util.AssertRespBody[ServiceView](t, test.failResp, w)
//			}
//		})
//	}
//}
//
//func TestHandleUpdateChallenge(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
//	defer closer()
//
//	createTestSessions(t, dbs.Rdb)
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//
//	for i, test := range []struct {
//		body      string
//		failResp  ServiceView
//		wantStatus int
//	}{
//		{
//			body:      `{"challengerID": 999, "challengeeID": 1, "action": "ACCEPT"}`,
//			failResp:  ServiceView{Status: 404, Message: ErrHttpNotFoundChallenge.Error()},
//			wantStatus: 404,
//		},
//		{
//			body:      `{"challengerID": 5, "challengeeID": 1, "action": "DELETE"}`,
//			failResp:  ServiceView{Status: 400, Message: ErrHttpUpdateChallenge.Error()},
//			wantStatus: 400,
//		},
//		{
//			body:      `{"challengerID": 5, "challengeeID": 1, "action": "ACCEPT"}`,
//			wantStatus: http.StatusOK,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", strings.NewReader(test.body))
//			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			if test.wantStatus != http.StatusOK {
//				util.AssertRespBody[ServiceView](t, test.failResp, w)
//			}
//		})
//	}
//}
//
//func TestHandleCreateChallenge(t *testing.T) {
//	// given
//	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
//	defer closer()
//	createTestSessions(t, dbs.Rdb)
//
//	body := `{"challengeeID": 4, "startColor": "WHITE", "mode": "CORRESPONDENCE_1"}`
//	wantResp := ServiceView{Status: http.StatusOK, Message: "SUCCESS"}
//
//	r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", strings.NewReader(body))
//	r.Header.Set("Cookie", FmtCookie(TestSessionID1))
//	w := httptest.NewRecorder()
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: svc.TestTimeNow}}), "")
//
//	// when
//	h.ServeHTTP(w, r)
//
//	// then
//	assert.Equal(t, http.StatusOK, w.Code)
//	util.AssertRespBody[ServiceView](t, wantResp, w)
//}
//
//func TestGetLeaderboard(t *testing.T) {
//	// given
//	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
//	defer closer()
//
//	ctx := context.WithValue(context.Background(), util.Trace, "setup-get-leaderboard")
//	assert.NoError(t, svc.SetLeaderboard(ctx, dbs.Rdb, svc.ModeTimed1Plus0, svc.UpdtLbChangeSet{ID: 1, EloDiff: 1000}))
//
//	q := url.Values{}
//	q.Set("mode", string(svc.ModeTimed1Plus0))
//	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leaderboard?%s", q.Encode()), nil)
//	w := httptest.NewRecorder()
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//
//	// when
//	h.ServeHTTP(w, r)
//
//	// then
//	assert.Equal(t, http.StatusOK, w.Code)
//}
//
//func TestGetPlayer(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
//	defer closer()
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//
//	userRanks := map[svc.GameMode]int64{
//		svc.ModeCorrespondence1:  1,
//		svc.ModeCorrespondence14: 1,
//		svc.ModeCorrespondence7:  1,
//		svc.ModeTimed1Plus0:      1,
//		svc.ModeTimed3Plus2:      1,
//		svc.ModeTimed15Plus10:    1,
//	}
//
//	for i, test := range []struct {
//		id          string
//		withReplays bool
//		successResp FullUserResp
//		failResp    ServiceView
//		wantStatus   int
//	}{
//		{
//			id:          "1",
//			withReplays: true,
//			successResp: FullUserResp{
//				User:       svc.TestUserEntities[0],
//				ReplayList: []svc.ReplayEntity{svc.TestReplayEntities[0], svc.TestReplayEntities[1]},
//				Ranks:      userRanks,
//			},
//			wantStatus: http.StatusOK,
//		},
//		{
//			id:          "1",
//			withReplays: false,
//			successResp: FullUserResp{
//				User:       svc.TestUserEntities[0],
//				ReplayList: []svc.ReplayEntity{},
//				Ranks:      userRanks,
//			},
//			wantStatus: http.StatusOK,
//		},
//		{
//			id:        "test",
//			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
//			wantStatus: 400,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s&withReplays=%v", test.id, test.withReplays), nil)
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			if test.wantStatus == http.StatusOK {
//				util.AssertRespBody[FullUserResp](t, test.successResp, w)
//			} else {
//				util.AssertRespBody[ServiceView](t, test.failResp, w)
//			}
//		})
//	}
//}
//
//func TestGetChallenges(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
//	defer closer()
//	createTestSessions(t, dbs.Rdb)
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs, Generators: &stableGenerator{time: svc.TestTimeNow}}), "")
//
//	for i, test := range []struct {
//		participants string
//		successResp  GetChallengesResp
//		wantStatus    int
//	}{
//		{
//			participants: "sent",
//			successResp: GetChallengesResp{
//				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[0]},
//			},
//			wantStatus: http.StatusOK,
//		},
//		{
//			participants: "received",
//			successResp: GetChallengesResp{
//				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[1]},
//			},
//			wantStatus: http.StatusOK,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
//			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			util.AssertRespBody[GetChallengesResp](t, test.successResp, w)
//		})
//	}
//}
//
//func TestHandleGetUserReplays(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
//	defer closer()
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//
//	for i, test := range []struct {
//		afterID     string
//		userID      string
//		successResp GetUserReplaysResp
//		failResp    ServiceView
//		wantStatus   int
//	}{
//		{
//			afterID:     "0",
//			userID:      "999", // nonexistent user
//			wantStatus:   http.StatusOK,
//			successResp: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{}},
//		},
//		{
//			afterID:   "-1",
//			userID:    "1",
//			wantStatus: http.StatusOK,
//			successResp: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{
//				svc.TestReplayEntities[0],
//				svc.TestReplayEntities[1],
//			}},
//		},
//		{
//			afterID:   "abc",
//			userID:    "1", // invalid afterID
//			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
//			wantStatus: 400,
//		},
//		{
//			afterID:   "0",
//			userID:    "xyz", // invalid afterID
//			failResp:  ServiceView{Status: 400, Message: ErrHttpInvalidRequest.Error()},
//			wantStatus: 400,
//		},
//	} {
//		t.Run(strconv.Itoa(i), func(t *testing.T) {
//			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?afterId=%s&userId=%s", test.afterID, test.userID), nil)
//			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
//			w := httptest.NewRecorder()
//
//			h.ServeHTTP(w, r)
//
//			assert.Equal(t, test.wantStatus, w.Code)
//			if w.Code == http.StatusOK {
//				util.AssertRespBody[GetUserReplaysResp](t, test.successResp, w)
//			} else {
//				util.AssertRespBody[ServiceView](t, test.failResp, w)
//			}
//		})
//	}
//}
//
//func TestHandleGetChessViews(t *testing.T) {
//	dbs, closer := db.BeforeDbTest(t, false, svc.InsertTestData)
//	defer closer()
//	createTestSessions(t, dbs.Rdb)
//	createTestChessStates(t, dbs.Rdb)
//
//	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
//	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
//	w := httptest.NewRecorder()
//
//	h := HandleRoot(MakeServerState(ServerSetup{Databases: dbs}), "")
//	h.ServeHTTP(w, r)
//
//	wantResp := ChessRoomListResp{
//		ChessList: []svc.ChessMeta{
//			{ID: "game3", FirstColor: svc.ColorRandom, Mode: svc.ModeCorrespondence1},
//			{ID: "game2", FirstColor: svc.ColorRandom, Mode: svc.ModeCorrespondence1},
//			{
//				ID:          "game1",
//				WhitePlayer: svc.MakePlayer(2, "user2", "us"),
//				FirstColor:  svc.ColorRandom,
//				Mode:        svc.ModeCorrespondence1,
//			},
//		},
//		SelfChessList: []svc.ChessMeta{
//			{
//				ID:          "game1",
//				WhitePlayer: svc.MakePlayer(2, "user2", "us"),
//				FirstColor:  svc.ColorRandom,
//				Mode:        svc.ModeCorrespondence1,
//			},
//		},
//	}
//
//	assert.Equal(t, http.StatusOK, w.Code)
//	util.AssertRespBody[ChessRoomListResp](t, wantResp, w)
//}
