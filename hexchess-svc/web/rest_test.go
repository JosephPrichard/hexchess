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

func TestHandleRegister(t *testing.T) {
	t.Parallel()

	insertTime := itest.TimeNow

	tests := []struct {
		name        string
		body        RegisterBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "ValidUsername",
			body:        RegisterBody{Username: "testing-name", Password: "testing-password", ConfirmPassword: "testing-password"},
			wantSuccess: SessionView{Username: "testing-name", Country: "un"},
			wantStatus:  http.StatusOK,
		},
		{
			name:       "InvalidPasswordConfirmDoesNotMatch",
			body:       RegisterBody{Username: "testing-name1", Password: "testing-password1", ConfirmPassword: "wrong"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"confirmPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidUsernameAndPasswordTooShort",
			body:       RegisterBody{Username: "s", Password: "short", ConfirmPassword: "short"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"username": ErrHttpInvalidUsername.Error(), "password": ErrHttpInvalidPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidUsernameDuplicate",
			body:       RegisterBody{Username: itest.UsersInsts[0].Username, Password: "testing-password2", ConfirmPassword: "testing-password2"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpDuplicateUsername.Error()},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()
			services.EntropySource = &svc.StableEntropySource{Time: insertTime}

			r := httptest.NewRequest(http.MethodPost, "/api/register", asJSONReader(tt.body))
			w := httptest.NewRecorder()
			
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleLogin(t *testing.T) {
	t.Parallel()

	user := itest.UsersInsts[0]

	tests := []struct {
		name        string
		body        LoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "InvalidLogin",
			body:       LoginBody{Username: "testing-name", Password: "testing-password"},
			wantFail:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "ValidLogin",
			body:        LoginBody{Username: user.Username, Password: user.Password},
			wantSuccess: SessionView{Username: user.Username, Country: "us"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			r := httptest.NewRequest(http.MethodPost, "/api/login", asJSONReader(tt.body))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleGoogleLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runCount    int
		setupMocks  func(*gomock.Controller) egress.GoogleAPI
		body        GoogleLoginBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:     "InvalidLoginTokenMocked",
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
			name:     "LoginWithGoogleTokenSuccessful",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			for range tt.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", asJSONReader(tt.body))
				w := httptest.NewRecorder()

				services.Remote = egress.RemoteAPIs{GoogleAPI: tt.setupMocks(ctrl)}

				hander := MakeServeMux(Setup{Services: services})
				hander.ServeHTTP(w, r)

				assert.Equal(t, tt.wantStatus, w.Code)
				if tt.wantStatus == http.StatusOK {
					testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
				} else {
					testutil.AssertRespBody(t, tt.wantFail, w)
				}
			}
		})
	}
}

func TestHandleUpdateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        UpdateUserBody
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name: "InvalidUsernameAndBiographyLength",
			body: UpdateUserBody{NewCountry: "us", NewUsername: "s", NewBio: strings.Repeat("a", 5001)},
			wantFail: ServiceView{Status: http.StatusBadRequest,
				Errors: map[string]any{"newBio": ErrHttpInvalidBio.Error(), "newUsername": ErrHttpInvalidUsername.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidCountryUnknown",
			body:       UpdateUserBody{NewCountry: "wrong", NewUsername: "new-username", NewBio: "testing biography"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newCountry": ErrHttpInvalidCountry.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "UpdatingBioOnly",
			body:        UpdateUserBody{NewBio: "testing biography"},
			wantSuccess: SessionView{Username: "user1", Country: "us"},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "UpdatingUsernameBioAndCountry",
			body:        UpdateUserBody{NewUsername: "new-username", NewBio: "testing biography", NewCountry: "un"},
			wantSuccess: SessionView{Username: "new-username", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/users", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleUpdatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        UpdatePasswordBody
		wantSuccess ServiceView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "InvalidLogin",
			body:       UpdatePasswordBody{Password: "password2", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantFail:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "InvalidPasswordLength",
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "short", ConfirmNewPassword: "short"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newPassword": ErrHttpInvalidPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidPasswordConfirmDoesNotMatch",
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "testing-password1", ConfirmNewPassword: "testing-password"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"confirmNewPassword": ErrHttpConfirmPassword.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "ValidPasswordUpdate",
			body:        UpdatePasswordBody{Password: "password1", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantSuccess: ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/users/password", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			handler := MakeServeMux(Setup{Services: services})
			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleUpdateChallenge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       UpdateChallengeBody
		wantFail   ServiceView
		wantStatus int
	}{
		{
			name:       "InvalidChallengeAction",
			body:       UpdateChallengeBody{Action: "invalid"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"action": ErrHttpInvalidAction.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidChallenge",
			body:       UpdateChallengeBody{ChallengerID: 999, ChallengeeID: 1, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: 404, Errors: ErrHttpNotFoundChallenge.Error()},
			wantStatus: 404,
		},
		{
			name:       "InvalidDeleteChallengeCannotDeleteNotOwn",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "DELETE"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpUpdateChallenge.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ValidDeleteChallenge",
			body:       UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "DELETE"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidAcceptChallengeCannotAcceptOwn",
			body:       UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpUpdateChallenge.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ValidAcceptChallenge",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "ACCEPT"},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus != http.StatusOK {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleCreateGame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       CreateGameBody
		wantStatus int
		wantFail   ServiceView
	}{
		{
			name:       "CreatedGame",
			body:       CreateGameBody{FirstColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidModeColorAndInitialFen",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()
			services.EntropySource = &svc.StableEntropySource{Time: itest.TimeNow}

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/games/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus != http.StatusOK {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleCreateChallenge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       CreateChallengeBody
		wantStatus int
		wantResp   ServiceView
	}{
		{
			name:       "ChallengingSelf",
			body:       CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpSelfChallenge.Error()},
		},
		{
			name:       "ChallengingInvalidUser",
			body:       CreateChallengeBody{ChallengeeID: 999, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpInvalidParticipants.Error()},
		},
		{
			name:       "CreatingDuplicateChallenge",
			body:       CreateChallengeBody{ChallengeeID: 2, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpDuplicateChallenge.Error()},
		},
		{
			name:       "CreatedChallenge",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "WHITE", Mode: svc.ModeCorrespondence1.String()},
			wantStatus: http.StatusOK,
			wantResp:   ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
		},
		{
			name:       "InvalidModeAndColor",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "invalid", Mode: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"startColor": ErrHttpInvalidColor.Error(), "mode": ErrHttpInvalidMode.Error()}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()
			services.EntropySource = &svc.StableEntropySource{Time: itest.TimeNow}


			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()	

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}

func TestGetLeaderboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        string
		page        string
		wantStatus  int
		wantSuccess LeaderboardResp
		wantFail    ServiceView
	}{
		{
			name:       "InvalidPageAndMode",
			mode:       "invalid",
			page:       "invalid",
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceView{
				Status: http.StatusBadRequest,
				Errors: map[string]any{"mode": ErrHttpInvalidMode.Error(), "page": ErrHttpInvalidPage.Error()},
			},
		},
		{
			name:       "ValidLeaderboard",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
			defer services.Close()

			ctx := context.WithValue(context.Background(), logutil.Trace, "setup-get-leaderboard")
			require.NoError(t, services.SetLeaderboard(ctx, svc.UpdtLbChangeSet{Mode: svc.ModeTimed1Plus0, ID: 1, EloDiff: 1000}))

			q := url.Values{}
			q.Set("mode", tt.mode)
			q.Set("page", tt.page)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leaderboard?%s", q.Encode()), nil)
			w := httptest.NewRecorder()
			hander := MakeServeMux(Setup{Services: services})

			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestGetPlayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          string
		withReplays bool
		wantSuccess GetPlayersResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "GetPlayerWithReplays",
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
			name:        "GetPlayerWithoutReplays",
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
			name:       "InvalidUserID",
			id:         "testing",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
			defer services.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s&withReplays=%v", tt.id, tt.withReplays), nil)
			w := httptest.NewRecorder()
			
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestGetChallenges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		participants string
		wantSuccess  GetChallengesResp
		wantStatus   int
	}{
		{
			name:         "GotSentChallenges",
			participants: "sent",
			wantSuccess: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:         "GotReceivedChallenges",
			participants: "received",
			wantSuccess: GetChallengesResp{
				ChallengeList: []svc.ChallengeEntity{svc.TestChallengeEntities[1]},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.ROPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", tt.participants), nil)
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			services.EntropySource = &svc.StableEntropySource{Time: itest.TimeNow}
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantSuccess, w)
		})
	}
}

func TestHandleGetUserReplays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		afterID     string
		userID      string
		wantSuccess GetUserReplaysResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "GotNoReplaysForNonexistentUser",
			userID:      "999",
			afterID:     "0",
			wantStatus:  http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{}},
		},
		{
			name:       "GotUserReplays",
			afterID:    "-1",
			userID:     "1",
			wantStatus: http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []svc.ReplayEntity{
				svc.TestReplayEntities[1],
				svc.TestReplayEntities[0],
			}},
		},
		{
			name:       "InvalidUserIDAndAfterID",
			afterID:    "abc",
			userID:     "xyz",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"afterId": ErrHttpInvalidID.Error(), "userId": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.ROPostgres)
			defer services.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?afterId=%s&userId=%s", tt.afterID, tt.userID), nil)
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestHandleGetReplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		userID      string
		wantSuccess GetReplayResp
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:        "GotAReplay",
			userID:      "1",
			wantStatus:  http.StatusOK,
			wantSuccess: GetReplayResp{Replay: svc.TestReplayEntities[0]},
		},
		{
			name:       "InvalidUserID",
			userID:     "xyz",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := svc.SetupServicesTest(t, itest.ROPostgres)
			defer services.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay?id=%s", tt.userID), nil)
			w := httptest.NewRecorder()
			
			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
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

	r := httptest.NewRequest(http.MethodGet, "/api/game/rooms", nil)
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