package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/chess"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/domain"
	"hexchess-svc/egress"
	"hexchess-svc/pb"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/api/idtoken"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"hexchess-svc/itest"
	svc "hexchess-svc/service"
	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"

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
			setup := svc.Mocks{
				Entropy: &svc.StableEntropySource{CurrTime: insertTime},
			}

			services, _ := svc.SetupServicesTest(t, setup, itest.RWPostgres, itest.Redis)
			defer services.Close()

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
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.RWPostgres, itest.Redis)
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

	apiKey := "API_KEY"

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
				m := egress.NewMockIDTokenValidator(ctrl)
				m.EXPECT().
					Validate(gomock.Any(), "invalidToken123", apiKey).
					Return(&idtoken.Payload{}, errors.New("invalid token"))
				return egress.MakeGoogleAPIWithValidator(apiKey, m)
			},
			body:       GoogleLoginBody{Token: "invalidToken123"},
			wantFail:   ServiceView{Status: http.StatusInternalServerError, Errors: ErrHttpFatal.Error()},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:     "LoginWithGoogleTokenSuccessful",
			runCount: 2, // the user is created the first time, the second time we log in with the already inserted account key
			setupMocks: func(ctrl *gomock.Controller) egress.GoogleAPI {
				m := egress.NewMockIDTokenValidator(ctrl)
				m.EXPECT().
					Validate(gomock.Any(), "testToken123", apiKey).
					Return(&idtoken.Payload{Subject: "account1", Claims: map[string]any{"email": "email@domain.com"}}, nil).
					Times(2)
				return egress.MakeGoogleAPIWithValidator(apiKey, m)
			},
			body:        GoogleLoginBody{Token: "testToken123"},
			wantSuccess: SessionView{Username: "email@domain.com", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := svc.Mocks{
				Entropy: &svc.StableEntropySource{CurrTime: itest.TimeNow},
				Remote:  egress.RemoteAPIs{GoogleAPI: tt.setupMocks(ctrl)},
			}

			services, _ := svc.SetupServicesTest(t, mocks, itest.RWPostgres, itest.Redis)
			defer services.Close()

			for range tt.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", asJSONReader(tt.body))
				w := httptest.NewRecorder()

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
		sessionID   string
		wantSuccess SessionView
		wantFail    ServiceView
		wantStatus  int
	}{
		{
			name:       "UnauthorizedUser",
			body:       UpdateUserBody{NewUsername: "username"},
			sessionID:  "invalid",
			wantFail:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpSessionExpired.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:      "InvalidUsernameAndBiographyLength",
			body:      UpdateUserBody{NewCountry: "us", NewUsername: "s", NewBio: strings.Repeat("a", 5001)},
			sessionID: TestSessionID1,
			wantFail: ServiceView{Status: http.StatusBadRequest,
				Errors: map[string]any{"newBio": ErrHttpInvalidBio.Error(), "newUsername": ErrHttpInvalidUsername.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidCountryUnknown",
			body:       UpdateUserBody{NewCountry: "wrong", NewUsername: "new-username", NewBio: "testing biography"},
			sessionID:  TestSessionID1,
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"newCountry": ErrHttpInvalidCountry.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "UpdatingBioOnly",
			body:        UpdateUserBody{NewBio: "testing biography"},
			sessionID:   TestSessionID1,
			wantSuccess: SessionView{Username: "user1", Country: "us"},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "UpdatingUsernameBioAndCountry",
			body:        UpdateUserBody{NewUsername: "new-username", NewBio: "testing biography", NewCountry: "un"},
			sessionID:   TestSessionID1,
			wantSuccess: SessionView{Username: "new-username", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/users", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
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
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.RWPostgres, itest.Redis)
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
		sessionID  string
		wantFail   ServiceView
		wantStatus int
	}{
		{
			name:       "UnauthorizedUser",
			body:       UpdateChallengeBody{Action: "DELETE"},
			wantFail:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpSessionExpired.Error()},
			sessionID:  "invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "InvalidChallengeAction",
			body:       UpdateChallengeBody{Action: "invalid"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"action": ErrHttpInvalidAction.Error()}},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidChallenge",
			body:       UpdateChallengeBody{ChallengerID: 999, ChallengeeID: 1, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: http.StatusNotFound, Errors: ErrHttpNotFoundChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "InvalidDeleteChallengeCannotDeleteNotOwn",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "DELETE"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpUpdateChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ValidDeleteChallenge",
			body:       UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "DELETE"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidAcceptChallengeCannotAcceptOwn",
			body:       UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "ACCEPT"},
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpUpdateChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ValidAcceptChallenge",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "ACCEPT"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
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
			body:       CreateGameBody{FirstColor: "WHITE", Mode: domain.ModeCorrespondence1.String()},
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
			setup := svc.Mocks{
				Entropy: &svc.StableEntropySource{CurrTime: itest.TimeNow},
			}

			services, _ := svc.SetupServicesTest(t, setup, itest.RWPostgres, itest.Redis)
			defer services.Close()

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
		sessionID  string
		wantStatus int
		wantResp   ServiceView
	}{
		{
			name:       "UnauthorizedUser",
			body:       CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: domain.ModeCorrespondence1.String()},
			sessionID:  "invalid",
			wantStatus: http.StatusUnauthorized,
			wantResp:   ServiceView{Status: http.StatusUnauthorized, Errors: ErrHttpSessionExpired.Error()},
		},
		{
			name:       "ChallengingSelf",
			body:       CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: domain.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpSelfChallenge.Error()},
		},
		{
			name:       "ChallengingInvalidUser",
			body:       CreateChallengeBody{ChallengeeID: 999, StartColor: "WHITE", Mode: domain.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpInvalidParticipants.Error()},
		},
		{
			name:       "CreatingDuplicateChallenge",
			body:       CreateChallengeBody{ChallengeeID: 2, StartColor: "WHITE", Mode: domain.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: ErrHttpDuplicateChallenge.Error()},
		},
		{
			name:       "CreatedChallenge",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "WHITE", Mode: domain.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
			wantResp:   ServiceView{Status: http.StatusOK, Message: "SUCCESS"},
		},
		{
			name:       "InvalidModeAndColor",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "invalid", Mode: "invalid"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"startColor": ErrHttpInvalidColor.Error(), "mode": ErrHttpInvalidMode.Error()}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := svc.Mocks{
				Entropy: &svc.StableEntropySource{CurrTime: itest.TimeNow},
			}

			services, _ := svc.SetupServicesTest(t, mocks, itest.RWPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}

func TestHandleSearchPlayers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		username    string
		page        string
		wantStatus  int
		wantSuccess SearchPlayersResp
		wantFail    ServiceView
	}{
		{
			name:       "InvalidPage",
			page:       "invalid",
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceView{
				Status: http.StatusBadRequest,
				Errors: map[string]any{"page": ErrHttpInvalidPage.Error()},
			},
		},
		{
			name:       "SearchPlayers",
			username:   "john",
			wantStatus: http.StatusOK,
			wantSuccess: SearchPlayersResp{
				UserList: []domain.LbdUser{
					{
						User:       domain.User{ID: 8, Username: "john", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
						Elo:        1500,
						HighestElo: 2000,
						Wins:       12,
						Losses:     4,
						Winrate:    66,
						Rank:       1,
					},
					{
						User:       domain.User{ID: 9, Username: "johnny", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
						Elo:        1500,
						HighestElo: 1500,
						Wins:       5,
						Losses:     2,
						Winrate:    71,
						Rank:       2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres)
			defer services.Close()

			q := url.Values{}
			q.Set("username", tt.username)
			q.Set("page", tt.page)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players/search?%s", q.Encode()), nil)
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
			mode:       domain.ModeTimed1Plus0.String(),
			wantStatus: http.StatusOK,
			wantSuccess: LeaderboardResp{
				TotalPages: 1,
				UserList: []domain.LbdUser{
					{
						User: domain.User{
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
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres, itest.Redis)
			defer services.Close()

			ctx := context.WithValue(context.Background(), logutil.Trace, "setup-get-leaderboard")
			require.NoError(t, services.SetLeaderboard(ctx, svc.UpdtLbChangeSet{Mode: domain.ModeTimed1Plus0, ID: 1, EloDiff: 1000}))

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
				FullUser: svc.FullUser{
					User:       itest.TestUser[0],
					Stats:      itest.TestUserStats[0],
					ReplayList: []domain.FullReplay{itest.TestReplay[2], itest.TestReplay[1], itest.TestReplay[0]},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "GetPlayerWithoutReplays",
			id:          "1",
			withReplays: false,
			wantSuccess: GetPlayersResp{
				FullUser: svc.FullUser{
					User:       itest.TestUser[0],
					Stats:      itest.TestUserStats[0],
					ReplayList: []domain.FullReplay{},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidUserID",
			id:         "testing",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GotNoUser",
			id:         "998877",
			wantFail:   ServiceView{Status: http.StatusNotFound, Errors: ErrHttpNotFoundUser.Error()},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres, itest.Redis)
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
		sessionID    string
		wantSuccess  GetChallengesResp
		wantStatus   int
	}{
		{
			name:         "UnauthorizedUser",
			participants: "sent",
			sessionID:    "invalid",
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:         "GotSentChallenges",
			participants: "sent",
			sessionID:    TestSessionID1,
			wantSuccess: GetChallengesResp{
				ChallengeList: []domain.Challenge{itest.TestChallenge[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:         "GotReceivedChallenges",
			participants: "received",
			sessionID:    TestSessionID1,
			wantSuccess: GetChallengesResp{
				ChallengeList: []domain.Challenge{itest.TestChallenge[1]},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := svc.Mocks{
				Entropy: &svc.StableEntropySource{CurrTime: itest.TimeNow},
			}

			services, _ := svc.SetupServicesTest(t, mocks, itest.ROPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", tt.participants), nil)
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if http.StatusOK == tt.wantStatus {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			}
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
			wantSuccess: GetUserReplaysResp{ReplayList: []domain.FullReplay{}},
		},
		{
			name:       "GotUserReplays",
			afterID:    "-1",
			userID:     "1",
			wantStatus: http.StatusOK,
			wantSuccess: GetUserReplaysResp{ReplayList: []domain.FullReplay{
				itest.TestReplay[2],
				itest.TestReplay[1],
				itest.TestReplay[0],
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
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres)
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
			name:        "GotReplay",
			userID:      "1",
			wantStatus:  http.StatusOK,
			wantSuccess: GetReplayResp{Replay: itest.TestReplay[0]},
		},
		{
			name:       "InvalidUserID",
			userID:     "xyz",
			wantFail:   ServiceView{Status: http.StatusBadRequest, Errors: map[string]any{"id": ErrHttpInvalidID.Error()}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GotNoReplay",
			userID:     "998877",
			wantFail:   ServiceView{Status: http.StatusNotFound, Errors: ErrHttpNotFoundReplay.Error()},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres)
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

	allChessMetas := []ChessMeta{
		{ID: "game3", FirstColor: domain.Random.String(), Mode: domain.ModeCorrespondence1.String()},
		{ID: "game2", FirstColor: domain.Random.String(), Mode: domain.ModeCorrespondence1.String()},
		{
			ID:          TestGameID1,
			BlackPlayer: domain.MakePlayer(2, "user2", "us"),
			FirstColor:  domain.Random.String(),
			Mode:        domain.ModeCorrespondence1.String(),
		},
	}

	tests := []struct {
		name       string
		sessionID  string
		wantStatus int
		wantResp   ChessMetasResp
	}{
		{
			name:       "ChessMetasWithSelfList",
			sessionID:  TestSessionID2,
			wantStatus: http.StatusOK,
			wantResp: ChessMetasResp{
				ChessList: allChessMetas,
				SelfChessList: []ChessMeta{
					{
						ID:          TestGameID1,
						BlackPlayer: domain.MakePlayer(2, "user2", "us"),
						FirstColor:  domain.Random.String(),
						Mode:        domain.ModeCorrespondence1.String(),
					},
				},
			},
		},
		{
			name:       "ChessMetasWithoutSelfList",
			sessionID:  "invalid",
			wantStatus: http.StatusOK,
			wantResp: ChessMetasResp{
				ChessList:     allChessMetas,
				SelfChessList: []ChessMeta{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres, itest.Redis)
			defer services.Close()

			createTestSessions(t, services)
			createTestChessStates(t, services)

			r := httptest.NewRequest(http.MethodGet, "/api/game/rooms", nil)
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))

			w := httptest.NewRecorder()
			h := MakeServeMux(Setup{Services: services})
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}

func TestHandleGetMoveReplay(t *testing.T) {
	t.Parallel()

	services, testinfra := svc.SetupServicesTest(t, svc.Mocks{}, itest.RWPostgres)
	defer services.Close()

	wantInitialGame := chess.MakeEmptyGame(false)
	pbInitialGame := chess.SerializeGame(&wantInitialGame)

	// serialize a history that contains every field so we can check that the binary data is being stored correctly. this history doesn't actually respect game rules.
	bytes, err := proto.Marshal(&pb.MoveHistory{
		InitialGame: pbInitialGame,
		Steps: []*pb.HistMove{
			{Piece: 1, FromFile: 1, FromRank: 2, ToFile: 3, ToRank: 4, Notation: "pc5"},
		},
	})
	require.NoError(t, err)

	require.NoError(t, testinfra.DB.Querier().UpsertReplayMoveHistories(t.Context(), sqlc.UpsertReplayMoveHistoriesParams{
		ReplayID: 1,
		Data:     bytes,
	}))

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay/move-list?replayId=%d", 1), nil)
	w := httptest.NewRecorder()

	hander := MakeServeMux(Setup{Services: services})
	hander.ServeHTTP(w, r)

	body, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var pbMoveHist pb.MoveHistory
	require.NoError(t, proto.Unmarshal(body, &pbMoveHist))

	wantMoveReplay := &pb.MoveHistory{
		InitialGame: pbInitialGame,
		Steps: []*pb.HistMove{
			{Piece: 1, FromFile: 1, FromRank: 2, ToFile: 3, ToRank: 4, Notation: "pc5"},
		},
	}
	assert.Equal(t, http.StatusOK, w.Code)
	testutil.Equal(t, wantMoveReplay, &pbMoveHist, protocmp.Transform())
}

func TestGetTournament(t *testing.T) {
	t.Parallel()

	setupServices := func() *svc.HexchessServices {
		services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres, itest.Redis)

		// seed leaderboard for users fetched in `RetrieveFullTournament` test.
		for _, change := range itest.TournamentLbdChangeSets {
			require.NoError(t, services.SetLeaderboard(context.WithValue(t.Context(), logutil.Trace, t.Name()), change))
		}

		return services
	}

	tests := []struct {
		name          string
		tournamentKey string
		wantResp      GetTournamentResp
		wantStatus    int
		wantFail      ServiceView
	}{
		{
			name:          "GotTournament",
			tournamentKey: itest.Tournament0LobbyKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: GetTournamentResp{
				Tournament:   itest.Tournaments[0],
				Participants: []domain.Participant{},
				Matches:      []domain.Match{},
			},
		},
		{
			name:          "RetrieveFullTournament",
			tournamentKey: itest.Tournament5InProgressKnockoutKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: GetTournamentResp{
				Tournament:   itest.Tournaments[5],
				Participants: itest.Tournament5RankedParticipants,
				Matches:      itest.MatchTournament5, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "RetrieveFullTournamentWithReplay",
			tournamentKey: itest.Tournament8FinishedKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: GetTournamentResp{
				Tournament:   itest.Tournaments[8],
				Participants: itest.Tournament8RankedParticipants,
				Matches:      itest.MatchTournament8, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "TournamentNotFound",
			tournamentKey: uuid.NewString(),
			wantStatus:    http.StatusNotFound,
			wantFail: ServiceView{
				Status: http.StatusNotFound,
				Errors: ErrHttpNotFoundTournament.Error(),
			},
		},
		{
			name:          "InvalidTournamentKey",
			tournamentKey: "invalid",
			wantStatus:    http.StatusBadRequest,
			wantFail: ServiceView{
				Status: http.StatusBadRequest,
				Errors: map[string]any{"tournamentKey": ErrHttpInvalidID.Error()},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := setupServices()
			defer services.Close()

			q := url.Values{}
			q.Set("tournamentKey", tt.tournamentKey)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tournament?%s", q.Encode()), nil)

			w := httptest.NewRecorder()
			h := MakeServeMux(Setup{Services: services})
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantResp, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w)
			}
		})
	}
}

func TestGetTournaments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userID     string
		afterID    string
		wantResp   GetTournamentsResp
		wantStatus int
		wantFail   ServiceView
	}{
		{
			name:       "RetrievedTournaments",
			userID:     "-1",
			afterID:    "-1",
			wantStatus: http.StatusOK,
			wantResp: GetTournamentsResp{
				Tournaments: []domain.Tournament{
					itest.Tournaments[9],
					itest.Tournaments[8],
					itest.Tournaments[7],
					itest.Tournaments[6],
					itest.Tournaments[5],
					itest.Tournaments[4],
					itest.Tournaments[3],
					itest.Tournaments[2],
					itest.Tournaments[1],
					itest.Tournaments[0],
				},
			},
		},
		{
			name:       "RetrievedTournamentsForParticipant",
			userID:     "4",
			afterID:    "-1",
			wantStatus: http.StatusOK,
			wantResp: GetTournamentsResp{
				Tournaments: []domain.Tournament{
					itest.Tournaments[5],
					itest.Tournaments[2],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := svc.SetupServicesTest(t, svc.Mocks{}, itest.ROPostgres)
			defer services.Close()

			q := url.Values{}
			q.Set("userId", tt.userID)
			q.Set("afterId", tt.afterID)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tournaments?%s", q.Encode()), nil)

			w := httptest.NewRecorder()
			h := MakeServeMux(Setup{Services: services})
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}
