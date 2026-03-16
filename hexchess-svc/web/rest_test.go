package web

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"
	"hexchess-svc/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rest tests are block box tests that make assertions on rest api call output for a given input
// no db assertions are made, and any ext network calls are mocked

func TestHandleRegister(t *testing.T) {
	t.Parallel()

	insertTime := itest.TimeNow

	for _, test := range []struct {
		name        string
		body        RegisterBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "valid username",
			body:        RegisterBody{Username: "testing-name", Password: "testing-password", ConfirmPassword: "testing-password"},
			wantSuccess: SessionView{Username: "testing-name", Country: "un"},
			wantStatus:  http.StatusOK,
		},
		{
			name:       "invalid password (confirm does not match)",
			body:       RegisterBody{Username: "testing-name1", Password: "testing-password1", ConfirmPassword: "wrong"},
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
			body:       RegisterBody{Username: itest.UsersInsts[0].Username, Password: "testing-password2", ConfirmPassword: "testing-password2"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpDuplicateUsername.Error()},
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			r := httptest.NewRequest(http.MethodPost, "/api/register", asJSONReader(test.body))
			w := httptest.NewRecorder()

			services.EntropySource = &svc.StableEntropySource{Time: insertTime}
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleLogin(t *testing.T) {
	t.Parallel()

	user := itest.UsersInsts[0]

	for _, test := range []struct {
		name        string
		body        LoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "invalid login",
			body:       LoginBody{Username: "testing-name", Password: "testing-password"},
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
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			r := httptest.NewRequest(http.MethodPost, "/api/login", asJSONReader(test.body))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGoogleLogin(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		runCount    int
		setupMocks  func(*gomock.Controller) egress.GoogleAPI
		body        GoogleLoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:     "invalid login token (mocked)",
			runCount: 1,
			setupMocks: func(ctrl *gomock.Controller) egress.GoogleAPI {
				m := egress.NewMockGoogleAPI(ctrl)
				m.EXPECT().
					ValidateIDToken(gomock.Any(), "invalidToken123").
					Return(egress.GoogleIDTokenPayload{}, errors.New("invalid token"))
				return m
			},
			body:       GoogleLoginBody{Token: "invalidToken123"},
			wantFail:   ServiceView{Status: http.StatusInternalServerError, Errors: ErrHttpFatal.Error()},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:     "login with google token succesful",
			runCount: 2, // the user is created the first time, the second time we log in with the already inserted account ID
			setupMocks: func(ctrl *gomock.Controller) egress.GoogleAPI {
				m := egress.NewMockGoogleAPI(ctrl)
				m.EXPECT().
					ValidateIDToken(gomock.Any(), "testToken123").
					Return(egress.GoogleIDTokenPayload{AccountID: "account1", Username: "email@domain.com"}, nil)
				return m
			},
			body:        GoogleLoginBody{Token: "testToken123"},
			wantSuccess: SessionView{Username: "email@domain.com", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			for range test.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", asJSONReader(test.body))
				w := httptest.NewRecorder()

				services.Remote = egress.RemoteAPIs{GoogleAPI: test.setupMocks(ctrl)}
				hander := MakeServeMux(Setup{Services: services})
				hander.ServeHTTP(w, r)

				assert.Equal(t, test.wantStatus, w.Code)
				if test.wantStatus == http.StatusOK {
					testutil.AssertRespBody(t, test.wantSuccess, w, testSessionViewCmpOpts)
				} else {
					testutil.AssertRespBody(t, test.wantFail, w)
				}
			}
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		body        UpdateUserBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name: "invalid username and biography (length)",
			body: UpdateUserBody{NewCountry: "us", NewUsername: "s", NewBio: strings.Repeat("a", 5001)},
			wantFail: ServiceView{Status: http.StatusBadRequest,
				Errors: map[string]any{"newBio": ErrHttpInvalidBio.Error(), "newUsername": ErrHttpInvalidUsername.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid country (unknown)",
			body:       UpdateUserBody{NewCountry: "wrong", NewUsername: "new-username", NewBio: "testing biography"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newCountry": ErrHttpInvalidCountry.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "updating bio only",
			body:        UpdateUserBody{NewBio: "testing biography"},
			wantSuccess: SessionView{Username: "user1", Country: "us"},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "updating username, bio, and country",
			body:        UpdateUserBody{NewUsername: "new-username", NewBio: "testing biography", NewCountry: "un"},
			wantSuccess: SessionView{Username: "new-username", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/users", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleUpdatePassword(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		body        UpdatePasswordBody
		wantSuccess ServiceView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "invalid login",
			body:       UpdatePasswordBody{Password: "password2", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
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
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "testing-password1", ConfirmNewPassword: "testing-password"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"confirmNewPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "valid password update",
			body:        UpdatePasswordBody{Password: "password1", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantSuccess: ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
			wantStatus:  http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/users/password", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			MakeServeMux(Setup{Services: services}).ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	t.Parallel()

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
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus != http.StatusOK {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleCreateGame(t *testing.T) {
	t.Parallel()

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
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/games/create", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			services.EntropySource = &svc.StableEntropySource{Time: itest.TimeNow}
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus != http.StatusOK {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	t.Parallel()

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
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", asJSONReader(test.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			services.EntropySource = &svc.StableEntropySource{Time: itest.TimeNow}
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			testutil.AssertRespBody(t, test.wantResp, w)
		})
	}
}

func TestGetLeaderboard(t *testing.T) {
	t.Parallel()

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
							JoinedOn: itest.TimeNow,
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
		t.Run(test.name, func(t *testing.T) { // given
			services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
			defer services.Close()

			ctx := context.WithValue(context.Background(), logutil.Trace, "setup-get-leaderboard")
			require.NoError(t, services.SetLeaderboard(ctx, svc.UpdtLbChangeSet{Mode: svc.ModeTimed1Plus0, ID: 1, EloDiff: 1000}))

			q := url.Values{}
			q.Set("mode", test.mode)
			q.Set("page", test.page)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leaderboard?%s", q.Encode()), nil)
			w := httptest.NewRecorder()
			hander := MakeServeMux(Setup{Services: services})

			// when
			hander.ServeHTTP(w, r)

			// then
			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestGetPlayer(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		id          string
		withReplays bool
		wantSuccess GetPlayersResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "get player with replays",
			id:          "1",
			withReplays: true,
			wantSuccess: GetPlayersResp{
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
			wantSuccess: GetPlayersResp{
				User:       svc.TestUserEntities[0],
				Stats:      svc.TestUserStats[0],
				ReplayList: []svc.ReplayEntity{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid user id",
			id:         "testing",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
			defer services.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s&withReplays=%v", test.id, test.withReplays), nil)
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestGetChallenges(t *testing.T) {
	t.Parallel()

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

			services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			services.EntropySource = &svc.StableEntropySource{Time: itest.TimeNow}
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			testutil.AssertRespBody(t, test.wantSuccess, w)
		})
	}
}

func TestHandleGetUserReplays(t *testing.T) {
	t.Parallel()

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

			services := svc.SetupServicesTest(t, itest.ROPostgres)
			defer services.Close()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGetReplay(t *testing.T) {
	t.Parallel()

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
			services := svc.SetupServicesTest(t, itest.ROPostgres)
			defer services.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay?id=%s", test.userID), nil)
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, test.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, test.wantFail, w)
			}
		})
	}
}

func TestHandleGetChessMetas(t *testing.T) {
	t.Parallel()

	services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
	defer services.Close()

	createTestSessions(t, services)
	createTestChessStates(t, services)

	r := httptest.NewRequest(http.MethodGet, "/api/chess/rooms", nil)
	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
	w := httptest.NewRecorder()

	h := MakeServeMux(Setup{Services: services})
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
	testutil.AssertRespBody(t, wantResp, w)
}
