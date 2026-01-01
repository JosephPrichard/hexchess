package web

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/out"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// rest tests are block box tests that make assertions on rest api call output for a given input
// no db assertions are made and any out network calls are mocked

func TestHandleRegister(t *testing.T) {
	insertTime := db.TestTimeNow

	for _, test := range []struct {
		name        string
		body        RegisterBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "valid username",
			body:        RegisterBody{Username: "test-name", Password: "test-password", ConfirmPassword: "test-password"},
			wantSuccess: SessionView{Username: "test-name", Country: "un"},
			wantStatus:  http.StatusOK,
		},
		{
			name:       "invalid password (confirm does not match)",
			body:       RegisterBody{Username: "test-name1", Password: "test-password1", ConfirmPassword: "wrong"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"confirmPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid username and password (too short)",
			body:       RegisterBody{Username: "s", Password: "short", ConfirmPassword: "short"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"username": ErrHttpInvalidUsername.Error(), "password": ErrHttpInvalidPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid username (duplicate)",
			body:       RegisterBody{Username: db.TestUsersInsts[0].Username, Password: "test-password2", ConfirmPassword: "test-password2"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpDuplicateUsername.Error()},
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/register", asJSONReader(test.body))
			w := httptest.NewRecorder()

			state.EntropySource = &out.StableSource{Time: insertTime}
			MakeRoot(Setup{State: state}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				assertutil.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleLogin(t *testing.T) {
	user := db.TestUsersInsts[0]

	for _, test := range []struct {
		name        string
		body        LoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "invalid login",
			body:       LoginBody{Username: "test-name", Password: "test-password"},
			wantFail:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "valid login",
			body:        LoginBody{Username: user.Username, Password: user.Password},
			wantSuccess: SessionView{Username: user.Username, Country: "us"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()

			r := httptest.NewRequest(http.MethodPost, "/api/login", asJSONReader(test.body))
			w := httptest.NewRecorder()

			MakeRoot(Setup{State: state}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				assertutil.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGoogleLogin(t *testing.T) {
	for _, test := range []struct {
		name        string
		runCount    int
		setupMocks  func(*gomock.Controller) out.GoogleAPI
		body        GoogleLoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:     "invalid login token (mocked)",
			runCount: 1,
			setupMocks: func(ctrl *gomock.Controller) out.GoogleAPI {
				m := out.NewMockGoogleAPI(ctrl)
				m.EXPECT().
					ValidateIDToken(gomock.Any(), "invalidToken123").
					Return(out.GoogleIDTokenPayload{}, errors.New("invalid token"))
				return m
			},
			body:       GoogleLoginBody{Token: "invalidToken123"},
			wantFail:   ServiceView{Status: http.StatusInternalServerError, Errors: ErrHttpFatal.Error()},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:     "login with google token succesful",
			runCount: 2, // the user is created the first time, the second time we log in with the already inserted account ID
			setupMocks: func(ctrl *gomock.Controller) out.GoogleAPI {
				m := out.NewMockGoogleAPI(ctrl)
				m.EXPECT().
					ValidateIDToken(gomock.Any(), "testToken123").
					Return(out.GoogleIDTokenPayload{AccountID: "account1", Username: "email@domain.com"}, nil)
				return m
			},
			body:        GoogleLoginBody{Token: "testToken123"},
			wantSuccess: SessionView{Username: "email@domain.com", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			for range test.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", asJSONReader(test.body))
				w := httptest.NewRecorder()

				h := MakeRoot(Setup{State: state, RemoteAPIs: out.RemoteAPIs{GoogleAPI: test.setupMocks(ctrl)}})
				h.ServeHTTP(w, r)

				assert.Equal(t, test.wantStatus, w.Code)
				if test.wantStatus == http.StatusOK {
					assertutil.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
				} else {
					assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
				}
			}
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	for _, test := range []struct {
		name        string
		body        UpdateUserBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "invalid username and biography (length)",
			body:       UpdateUserBody{NewCountry: "eu", NewUsername: "s", NewBio: strings.Repeat("a", 5001)},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newBio": ErrHttpInvalidBio.Error(), "newUsername": ErrHttpInvalidUsername.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid country (unknown)",
			body:       UpdateUserBody{NewCountry: "wrong", NewUsername: "new-username", NewBio: "test biography"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newCountry": ErrHttpInvalidCountry.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "updating bio only",
			body:        UpdateUserBody{NewBio: "test biography"},
			wantSuccess: SessionView{Username: "user1", Country: "us"},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "updating username, bio, and country",
			body:        UpdateUserBody{NewUsername: "new-username", NewBio: "test biography", NewCountry: "eu"},
			wantSuccess: SessionView{Username: "new-username", Country: "eu"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()
			createTestSessions(t, state)

			r := httptest.NewRequest(http.MethodPost, "/api/users", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			MakeRoot(Setup{State: state, CountryList: []string{"eu"}}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				assertutil.AssertRespBody[SessionView](t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleUpdatePassword(t *testing.T) {
	for _, test := range []struct {
		name        string
		body        UpdatePasswordBody
		wantSuccess ServiceView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "invalid login",
			body:       UpdatePasswordBody{Password: "password2", NewPassword: "test-password", ConfirmNewPassword: "test-password"},
			wantFail:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid password (length)",
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "short", ConfirmNewPassword: "short"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newPassword": ErrHttpInvalidPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid password (confirm does not match)",
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "test-password1", ConfirmNewPassword: "test-password"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"confirmNewPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "valid password update",
			body:        UpdatePasswordBody{Password: "password1", NewPassword: "test-password", ConfirmNewPassword: "test-password"},
			wantSuccess: ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()
			createTestSessions(t, state)

			r := httptest.NewRequest(http.MethodPost, "/api/users/password", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			MakeRoot(Setup{State: state}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				assertutil.AssertRespBody[ServiceView](t, test.wantSuccess, w)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       UpdateChallengeBody
		wantFail   ServiceView
		wantStatus int
	}{
		{
			name:       "invalid challenge action",
			body:       UpdateChallengeBody{Action: "invalid"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"action": ErrHttpInvalidAction.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid challenge",
			body:       UpdateChallengeBody{ChallengerID: 999, ChallengeeID: 1, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: 404, Errors: ErrHttpNotFoundChallenge.Error()},
			wantStatus: 404,
		},
		{
			name:       "invalid delete challenge (cannot delete challenge that is not your own)",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "DELETE"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpUpdateChallenge.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid delete challenge",
			body:       UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "DELETE"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid accept challenge (cannot accept challenge that is your own)",
			body:       UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpUpdateChallenge.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid accept challenge",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "ACCEPT"},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()

			createTestSessions(t, state)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			MakeRoot(Setup{State: state}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus != http.StatusOK {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleCreateGame(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       CreateGameBody
		wantStatus int
		wantFail   ServiceView
	}{
		{
			name:       "created game",
			body:       CreateGameBody{FirstColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid mode, color, and initial fen",
			body:       CreateGameBody{InitialFEN: "invalid", FirstColor: "invalid", Mode: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceView{
				Status: http.StatusBadRequest,
				Errors: map[string]any{
					"firstColor": ErrHttpInvalidColor.Error(),
					"mode":       ErrHttpInvalidMode.Error(),
					"initialFen": ErrHttpInvalidFen.Error(),
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()
			createTestSessions(t, state)

			r := httptest.NewRequest(http.MethodPost, "/api/games/create", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			state.EntropySource = &out.StableSource{Time: db.TestTimeNow}
			MakeRoot(Setup{State: state}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus != http.StatusOK {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       CreateChallengeBody
		wantStatus int
		wantResp   ServiceView
	}{
		{
			name:       "challenging self",
			body:       CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpSelfChallenge.Error()},
		},
		{
			name:       "challenging invalid user",
			body:       CreateChallengeBody{ChallengeeID: 999, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpInvalidParticipants.Error()},
		},
		{
			name:       "creating duplicate challenge",
			body:       CreateChallengeBody{ChallengeeID: 2, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpDuplicateChallenge.Error()},
		},
		{
			name:       "created challenge",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusOK,
			wantResp:   ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
		},
		{
			name:       "invalid mode and color",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "invalid", Mode: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"startColor": ErrHttpInvalidColor.Error(), "mode": ErrHttpInvalidMode.Error()}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, closer := svc.BeforeStateTest(t, true)
			defer closer()
			createTestSessions(t, state)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			state.EntropySource = &out.StableSource{Time: db.TestTimeNow}
			MakeRoot(Setup{State: state}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			assertutil.AssertRespBody[ServiceView](t, test.wantResp, w)
		})
	}
}

func TestGetLeaderboard(t *testing.T) {
	for _, test := range []struct {
		name        string
		mode        string
		page        string
		wantStatus  int
		wantSuccess LeaderboardResp
		wantFail    ServiceView
	}{
		{
			name:       "invalid page and mode",
			mode:       "invalid",
			page:       "invalid",
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceView{
				Status: http.StatusBadRequest,
				Errors: map[string]any{"mode": ErrHttpInvalidMode.Error(), "page": ErrHttpInvalidPage.Error()},
			},
		},
		{
			name:       "valid leaderboard",
			mode:       svc.ModeTimed1Plus0.String(),
			wantStatus: http.StatusOK,
			wantSuccess: LeaderboardResp{
				TotalPages: 1,
				UserList: []svc.LbdUserEntity{
					{
						UserEntity: svc.UserEntity{
							ID:       1,
							Username: "user1",
							Country:  "us",
							JoinedOn: db.TestTimeNow,
						},
						Elo:        1050,
						HighestElo: 1050,
						Wins:       6,
						Losses:     5,
						Winrate:    54,
						Rank:       1,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			state, closer := svc.BeforeStateTest(t, false)
			defer closer()

			ctx := context.WithValue(context.Background(), logutil.Trace, "setup-get-leaderboard")
			require.NoError(t, state.SetLeaderboard(ctx, svc.UpdtLbChangeSet{Mode: svc.ModeTimed1Plus0, ID: 1, EloDiff: 1000}))

			q := url.Values{}
			q.Set("mode", test.mode)
			q.Set("page", test.page)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leaderboard?%s", q.Encode()), nil)
			w := httptest.NewRecorder()
			h := MakeRoot(Setup{State: state})

			// when
			h.ServeHTTP(w, r)

			// then
			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				assertutil.AssertRespBody[LeaderboardResp](t, test.wantSuccess, w)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestGetPlayer(t *testing.T) {
	state, closer := svc.BeforeStateTest(t, false)
	defer closer()

	h := MakeRoot(Setup{State: state})

	for _, test := range []struct {
		name        string
		id          string
		withReplays bool
		wantSuccess FullUserResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "get player with replays",
			id:          "1",
			withReplays: true,
			wantSuccess: FullUserResp{
				User:       svc.TestUserEntities[0],
				Stats:      svc.TestUserStats[0],
				ReplayList: []svc.ReplayEntity{svc.TestReplayEntities[1], svc.TestReplayEntities[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "get player without replays",
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
			name:       "invalid user id",
			id:         "test",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s&withReplays=%v", test.id, test.withReplays), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				assertutil.AssertRespBody[FullUserResp](t, test.wantSuccess, w)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestGetChallenges(t *testing.T) {
	state, closer := svc.BeforeStateTest(t, false)
	defer closer()
	createTestSessions(t, state)

	state.EntropySource = &out.StableSource{Time: db.TestTimeNow}
	h := MakeRoot(Setup{State: state})

	for _, test := range []struct {
		name         string
		participants string
		wantSuccess  GetChallengesResp
		wantStatus   int
	}{
		{
			name:         "got sent challenges",
			participants: "sent",
			wantSuccess: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:         "got received challenges",
			participants: "received",
			wantSuccess: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[1]},
			},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", test.participants), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			assertutil.AssertRespBody[GetChallengesResp](t, test.wantSuccess, w)
		})
	}
}

func TestHandleGetUserReplays(t *testing.T) {
	state, closer := svc.BeforeStateTest(t, true)
	defer closer()

	h := MakeRoot(Setup{State: state})

	for _, test := range []struct {
		name        string
		afterID     string
		userID      string
		wantSuccess GetUserReplaysResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "got no replays for nonexistent user",
			userID:      "999",
			afterID:     "0",
			wantStatus:  http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{}},
		},
		{
			name:       "got user replays",
			afterID:    "-1",
			userID:     "1",
			wantStatus: http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{
				svc.TestReplayEntities[1],
				svc.TestReplayEntities[0],
			}},
		},
		{
			name:       "invalid user id and after id",
			afterID:    "abc",
			userID:     "xyz",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"afterId": ErrHttpInvalidID.Error(), "userId": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?afterId=%s&userId=%s", test.afterID, test.userID), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				assertutil.AssertRespBody[GetUserReplaysResp](t, test.wantSuccess, w)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGetReplay(t *testing.T) {
	state, closer := svc.BeforeStateTest(t, true)
	defer closer()

	h := MakeRoot(Setup{State: state})

	for _, test := range []struct {
		name        string
		userID      string
		wantSuccess GetReplayResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "got a replay",
			userID:      "1",
			wantStatus:  http.StatusOK,
			wantSuccess: GetReplayResp{Replay: svc.TestReplayEntities[0]},
		},
		{
			name:       "invalid user id",
			userID:     "xyz",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay?id=%s", test.userID), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				assertutil.AssertRespBody[GetReplayResp](t, test.wantSuccess, w)
			} else {
				assertutil.AssertRespBody[ServiceView](t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGetChessMetas(t *testing.T) {
	state, closer := svc.BeforeStateTest(t, false)
	defer closer()
	createTestSessions(t, state)
	createTestChessStates(t, state)

	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
	w := httptest.NewRecorder()

	h := MakeRoot(Setup{State: state})
	h.ServeHTTP(w, r)

	wantResp := ChessMetasResp{
		ChessList: []ChessMeta{
			{ID: "game3", FirstColor: svc.Random.String(), Mode: svc.ModeCorrespondence1.String()},
			{ID: "game2", FirstColor: svc.Random.String(), Mode: svc.ModeCorrespondence1.String()},
			{
				ID:          TestGameID1,
				BlackPlayer: svc.MakePlayer(2, "user2", "us"),
				FirstColor:  svc.Random.String(),
				Mode:        svc.ModeCorrespondence1.String(),
			},
		},
		SelfChessList: []ChessMeta{
			{
				ID:          TestGameID1,
				BlackPlayer: svc.MakePlayer(2, "user2", "us"),
				FirstColor:  svc.Random.String(),
				Mode:        svc.ModeCorrespondence1.String(),
			},
		},
	}
	assert.Equal(t, http.StatusOK, w.Code)
	assertutil.AssertRespBody[ChessMetasResp](t, wantResp, w)
}
