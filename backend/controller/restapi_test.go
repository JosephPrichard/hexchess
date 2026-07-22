package controller

import (
	"errors"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/utils/entropy"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"hexchess-svc/model"
	"hexchess-svc/pb"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.uber.org/mock/gomock"
	"google.golang.org/api/idtoken"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"hexchess-svc/itest"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var serviceViewCmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(ServiceResp{}, "Message"),
	cmpopts.IgnoreFields(OneError{}, "Message"),
}

func TestHandleRegister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        RegisterBody
		wantSuccess SessionView
		wantFail    ServiceResp
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
			wantFail:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpConfirmPassword.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "InvalidUsernameAndPasswordTooShort",
			body: RegisterBody{Username: "s", Password: "short", ConfirmPassword: "short"},
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"RegisterBody.Username": {Error: ErrHttpInvalidUsername.Error()},
					"RegisterBody.Password": {Error: ErrHttpInvalidPassword.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidUsernameDuplicate",
			body:       RegisterBody{Username: itest.UsersInsts[0].Username, Password: "testing-password2", ConfirmPassword: "testing-password2"},
			wantFail:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpDuplicateUsername.Error()},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			r := httptest.NewRequest(http.MethodPost, "/api/register", asJSONReader(tt.body))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name:       "InvalidLogin",
			body:       LoginBody{Username: "testing-name", Password: "testing-password"},
			wantFail:   ServiceResp{Status: http.StatusUnauthorized, Error: ErrHttpInvalidLogin.Error()},
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
			h, testinfra := setupTestHandler(t, nil, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			r := httptest.NewRequest(http.MethodPost, "/api/login", asJSONReader(tt.body))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		setupMocks  func(*gomock.Controller) cloud.SDKs
		body        GoogleLoginBody
		wantSuccess SessionView
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name:     "InvalidLoginTokenMocked",
			runCount: 1,
			setupMocks: func(ctrl *gomock.Controller) cloud.SDKs {
				validator := cloud.NewMockGoogleTokenValidator(ctrl)
				validator.EXPECT().
					Validate(gomock.Any(), "invalidToken123", apiKey).
					Return(&idtoken.Payload{}, errors.New("invalid token"))
				return cloud.NewOptRemoteAPIs(
					cloud.WithGoogleIDTokenValidator(validator, apiKey))
			},
			body:       GoogleLoginBody{Token: "invalidToken123"},
			wantFail:   ServiceResp{Status: http.StatusInternalServerError, Error: ErrHttpFatal.Error()},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:     "LoginWithGoogleTokenSuccessful",
			runCount: 2, // the user is created the first time, the second time we log in with the already inserted account key
			setupMocks: func(ctrl *gomock.Controller) cloud.SDKs {
				validator := cloud.NewMockGoogleTokenValidator(ctrl)
				validator.EXPECT().
					Validate(gomock.Any(), "testToken123", apiKey).
					Return(&idtoken.Payload{Subject: "account1", Claims: map[string]any{"email": "email@domain.com"}}, nil).
					Times(2)
				return cloud.NewOptRemoteAPIs(
					cloud.WithGoogleIDTokenValidator(validator, apiKey))
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

			mocks := &serviceMocks{
				Entropy: &entropy.StableSource{CurrTime: itest.TimeNow},
				Remote:  tt.setupMocks(ctrl),
			}

			h, testinfra := setupTestHandler(t, mocks, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			for range tt.runCount {
				r := httptest.NewRequest(http.MethodPost, "/api/login/google", asJSONReader(tt.body))
				w := httptest.NewRecorder()

				h.ServeHTTP(w, r)

				assert.Equal(t, tt.wantStatus, w.Code)
				if tt.wantStatus == http.StatusOK {
					testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
				} else {
					testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name:       "UnauthorizedUser",
			body:       UpdateUserBody{NewUsername: "username"},
			sessionID:  "invalid",
			wantFail:   ServiceResp{Status: http.StatusUnauthorized, Error: ErrHttpSessionExpired.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:      "InvalidUsernameAndBiographyLength",
			body:      UpdateUserBody{NewCountry: "us", NewUsername: "s", NewBio: strings.Repeat("a", 5001)},
			sessionID: TestSessionID1,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"UpdateUserBody.NewBio":      {Error: ErrHttpInvalidBio.Error()},
					"UpdateUserBody.NewUsername": {Error: ErrHttpInvalidUsername.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:      "InvalidCountryUnknown",
			body:      UpdateUserBody{NewCountry: "wrong", NewUsername: "new-username", NewBio: "testing biography"},
			sessionID: TestSessionID1,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"UpdateUserBody.NewCountry": {Error: ErrHttpInvalidCountry.Error()},
				},
			},
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
			h, testinfra := setupTestHandler(t, nil, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/users", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w, testSessionViewCmpOpts)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
			}
		})
	}
}

func TestHandleUpdatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        UpdatePasswordBody
		wantSuccess ServiceResp
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name:       "InvalidLogin",
			body:       UpdatePasswordBody{Password: "password2", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantFail:   ServiceResp{Status: http.StatusUnauthorized, Error: ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "InvalidPasswordLength",
			body: UpdatePasswordBody{Password: "password1", NewPassword: "short", ConfirmNewPassword: "short"},
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"UpdatePasswordBody.NewPassword": {Error: ErrHttpInvalidPassword.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidPasswordConfirmDoesNotMatch",
			body:       UpdatePasswordBody{Password: "password1", NewPassword: "testing-password1", ConfirmNewPassword: "testing-password"},
			wantFail:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpConfirmPassword.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "ValidPasswordUpdate",
			body:        UpdatePasswordBody{Password: "password1", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantSuccess: ServiceResp{Status: http.StatusOK, Message: "SUCCESS"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/users/password", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail   ServiceResp
		wantStatus int
	}{
		{
			name:       "UnauthorizedUser",
			body:       UpdateChallengeBody{Action: "DELETE"},
			wantFail:   ServiceResp{Status: http.StatusUnauthorized, Error: ErrHttpSessionExpired.Error()},
			sessionID:  "invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "InvalidChallengeAction",
			body: UpdateChallengeBody{Action: "invalid"},
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{"UpdateChallengeBody.Action": {Error: ErrHttpInvalidInput.Error()}},
			},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidChallenge",
			body:       UpdateChallengeBody{ChallengerID: 999, ChallengeeID: 1, Action: "ACCEPT"},
			wantFail:   ServiceResp{Status: http.StatusNotFound, Error: ErrHttpNotFoundChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "InvalidDeleteChallengeCannotDeleteNotOwn",
			body:       UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "DELETE"},
			wantFail:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpUpdateChallenge.Error()},
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
			wantFail:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpUpdateChallenge.Error()},
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
			h, testinfra := setupTestHandler(t, nil, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus != http.StatusOK {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail   ServiceResp
	}{
		{
			name:       "CreatedGame",
			body:       CreateGameBody{FirstColor: "WHITE", Mode: model.ModeCorrespondence1.String()},
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidModeColorAndInitialFen",
			body:       CreateGameBody{InitialFEN: "invalid", FirstColor: "invalid", Mode: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"CreateGameBody.FirstColor": {Error: ErrHttpInvalidInput.Error()},
					"CreateGameBody.Mode":       {Error: ErrHttpInvalidInput.Error()},
					"CreateGameBody.InitialFen": {Error: ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := &serviceMocks{
				Entropy: &entropy.StableSource{CurrTime: itest.TimeNow},
			}

			h, testinfra := setupTestHandler(t, setup, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/games/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(TestSessionID1))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus != http.StatusOK {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantResp   ServiceResp
	}{
		{
			name:       "UnauthorizedUser",
			body:       CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: model.ModeCorrespondence1.String()},
			sessionID:  "invalid",
			wantStatus: http.StatusUnauthorized,
			wantResp:   ServiceResp{Status: http.StatusUnauthorized, Error: ErrHttpSessionExpired.Error()},
		},
		{
			name:       "ChallengingSelf",
			body:       CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: model.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpSelfChallenge.Error()},
		},
		{
			name:       "ChallengingInvalidUser",
			body:       CreateChallengeBody{ChallengeeID: 999, StartColor: "WHITE", Mode: model.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpInvalidParticipants.Error()},
		},
		{
			name:       "CreatingDuplicateChallenge",
			body:       CreateChallengeBody{ChallengeeID: 2, StartColor: "WHITE", Mode: model.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   ServiceResp{Status: http.StatusBadRequest, Error: ErrHttpDuplicateChallenge.Error()},
		},
		{
			name:       "CreatedChallenge",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "WHITE", Mode: model.ModeCorrespondence1.String()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
			wantResp:   ServiceResp{Status: http.StatusOK, Message: "SUCCESS"},
		},
		{
			name:       "InvalidModeAndColor",
			body:       CreateChallengeBody{ChallengeeID: 4, StartColor: "invalid", Mode: "invalid"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"CreateChallengeBody.Mode":       {Error: ErrHttpInvalidInput.Error()},
					"CreateChallengeBody.StartColor": {Error: ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := &serviceMocks{
				Entropy: &entropy.StableSource{CurrTime: itest.TimeNow},
			}
			h, testinfra := setupTestHandler(t, mocks, itest.RWPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w, serviceViewCmpOpts...)
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
		wantFail    ServiceResp
	}{
		{
			name:       "SearchPlayers",
			username:   "john",
			wantStatus: http.StatusOK,
			wantSuccess: SearchPlayersResp{
				UserList: []model.LbdUser{
					{
						User: model.User{
							ID: 8, Username: "john", Country: "us", Bio: "",
							JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
						Elo:        1500,
						HighestElo: 2000,
						Wins:       12,
						Losses:     4,
						Winrate:    66,
						Rank:       1,
					},
					{
						User: model.User{
							ID: 9, Username: "johnny", Country: "us", Bio: "",
							JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
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
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres)
			defer testinfra.Close()

			q := url.Values{}
			q.Set("username", tt.username)
			q.Set("page", tt.page)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players/search?%s", q.Encode()), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail    ServiceResp
	}{
		{
			name:       "InvalidPageAndMode",
			mode:       "invalid",
			page:       "invalid",
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"mode": {Error: ErrHttpInvalidInput.Error()},
					"page": {Error: ErrHttpInvalidInput.Error()},
				},
			},
		},
		{
			name:       "ValidLeaderboard",
			mode:       model.ModeTimed1Plus0.String(),
			wantStatus: http.StatusOK,
			wantSuccess: LeaderboardResp{
				TotalPages: 1,
				UserList: []model.LbdUser{
					{
						User: model.User{
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
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres, itest.Redis)
			defer testinfra.Close()

			createLeaderboard(t, testinfra.Redis, updtLbChangeSet{Mode: model.ModeTimed1Plus0, ID: 1, EloDiff: 1000})

			q := url.Values{}
			q.Set("mode", tt.mode)
			q.Set("page", tt.page)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leaderboard?%s", q.Encode()), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
			}
		})
	}
}

func TestGetPlayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          string
		wantSuccess GetPlayersResp
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name: "GetPlayerWithReplays",
			id:   "1",
			wantSuccess: GetPlayersResp{
				FullUser: svc.FullUser{
					User:  itest.TestUser[0],
					Stats: itest.TestUserStats[0],
					ReplayList: []model.FullReplay{
						itest.TestReplays[4],
						itest.TestReplays[3],
						itest.TestReplays[2],
						itest.TestReplays[0],
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "GotNoUser",
			id:         "998877",
			wantFail:   ServiceResp{Status: http.StatusNotFound, Error: ErrHttpNotFoundUser.Error()},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "InvalidUserID",
			id:   "testing",
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"id": {Error: ErrHttpInvalidInput.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres, itest.Redis)
			defer testinfra.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/players?id=%s", tt.id), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
				ChallengeList: []model.Challenge{itest.TestChallenge[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:         "GotReceivedChallenges",
			participants: "received",
			sessionID:    TestSessionID1,
			wantSuccess: GetChallengesResp{
				ChallengeList: []model.Challenge{itest.TestChallenge[1]},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := &serviceMocks{
				Entropy: &entropy.StableSource{CurrTime: itest.TimeNow},
			}
			h, testinfra := setupTestHandler(t, mocks, itest.ROPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", tt.participants), nil)
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if http.StatusOK == tt.wantStatus {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			}
		})
	}
}

func TestHandleSearchReplays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      string
		wantSuccess SearchReplaysResp
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name:       "GotUserReplays_ByUserID",
			params:     "userId=1",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[2],
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByUserID_SortedRating",
			params:     "userId=1&sort=rating",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[0],
				itest.TestReplays[2],
			}},
		},
		{
			name:       "GotUserReplays_ByUserID_SortedTurnCount",
			params:     "userId=1&sort=turnCount",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[0],
				itest.TestReplays[3],
				itest.TestReplays[2],
				itest.TestReplays[4],
			}},
		},
		{
			name:       "GotUserReplays_ByWinnerID_LoserID",
			params:     "winnerId=1&loserId=2",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID",
			params:     "whiteId=1",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID_BlackID",
			params:     "whiteId=3&blackId=1",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[2],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID_WinnerID",
			params:     "whiteId=1&winnerId=1",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[3],
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID_LoserID",
			params:     "whiteId=1&loserId=1",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
			}},
		},
		{
			name:       "GotUserReplays_ByUserID_Mode_Result_Cause",
			params:     "userId=1&mode=CORRESPONDENCE_7&result=WHITE_WINS&cause=CHECKMATE",
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[3],
				itest.TestReplays[0],
			}},
		},
		{
			name: "GotUserReplays_ToDate",
			params: fmt.Sprintf(
				"userId=1&toDate=%s",
				(itest.TimeNow.Add(time.Hour * 24 * 2)).Format(time.DateOnly)),
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[2],
				itest.TestReplays[0],
			}},
		},
		{
			name: "GotUserReplays_FromDate",
			params: fmt.Sprintf(
				"userId=1&fromDate=%s",
				(itest.TimeNow.Add(time.Hour * 24 * 2)).Format(time.DateOnly)),
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[2],
			}},
		},
		{
			name: "GotUserReplays_ByTimeframe",
			params: fmt.Sprintf(
				"userId=1&fromDate=%s&toDate=%s",
				(itest.TimeNow.Add(time.Hour * 24 * 1)).Format(time.DateOnly),
				(itest.TimeNow.Add(time.Hour * 24 * 3)).Format(time.DateOnly)),
			wantStatus: http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[3],
				itest.TestReplays[2],
			}},
		},
		// tests for non existent users
		{
			name:        "GotNoReplaysForNonexistentUser",
			params:      "userId=999",
			wantStatus:  http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{}},
		},
		{
			name:        "GotNoReplaysForNonexistentUser",
			params:      "whiteId=999",
			wantStatus:  http.StatusOK,
			wantSuccess: SearchReplaysResp{ReplayList: []model.FullReplay{}},
		},
		// input field validations
		{
			name:   "Invalid_UserIDs",
			params: "userId=INVALID&afterId=INVALID&whiteId=INVALID&blackId=INVALID&loserId=INVALID&winnerId=INVALID",
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"afterId":  {Error: ErrHttpInvalidInput.Error()},
					"userId":   {Error: ErrHttpInvalidInput.Error()},
					"whiteId":  {Error: ErrHttpInvalidInput.Error()},
					"blackId":  {Error: ErrHttpInvalidInput.Error()},
					"loserId":  {Error: ErrHttpInvalidInput.Error()},
					"winnerId": {Error: ErrHttpInvalidInput.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid_ModeResultCause",
			params:     "mode=INVALID&result=INVALID&cause=INVALID",
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"cause":  {Error: ErrHttpInvalidInput.Error()},
					"mode":   {Error: ErrHttpInvalidInput.Error()},
					"result": {Error: ErrHttpInvalidInput.Error()},
				},
			},
		},
		{
			name:       "Invalid_Datetime",
			params:     "fromDate=INVALID&toDate=INVALID",
			wantStatus: http.StatusBadRequest,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"fromDate": {Error: ErrHttpInvalidInput.Error()},
					"toDate":   {Error: ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres)
			defer testinfra.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replays?%s", tt.params), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail    ServiceResp
		wantStatus  int
	}{
		{
			name:        "GotReplay",
			userID:      "1",
			wantStatus:  http.StatusOK,
			wantSuccess: GetReplayResp{Replay: itest.TestReplays[0]},
		},
		{
			name:       "GotNoReplay",
			userID:     "998877",
			wantFail:   ServiceResp{Status: http.StatusNotFound, Error: ErrHttpNotFoundReplay.Error()},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres)
			defer testinfra.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay?id=%s", tt.userID), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantSuccess, w)
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
			}
		})
	}
}

func TestHandleGetGameMetadata(t *testing.T) {
	t.Parallel()

	allChessMetas := []ChessMeta{
		{Ordering: 3, GameID: itest.GameID3, Mode: model.ModeCorrespondence1.String()},
		{Ordering: 2, GameID: itest.GameID2, Mode: model.ModeCorrespondence1.String()},
		{
			Ordering:    1,
			GameID:      itest.GameID1,
			BlackPlayer: model.User{ID: 2, Username: "user2", Country: "us"},
			Mode:        model.ModeCorrespondence1.String(),
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
						Ordering:    1,
						GameID:      itest.GameID1,
						BlackPlayer: model.User{ID: 2, Username: "user2", Country: "us"},
						Mode:        model.ModeCorrespondence1.String(),
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
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodGet, "/api/game/rooms", nil)
			r.Header.Set("Cookie", FmtCookie(tt.sessionID))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}

func TestHandleGetMoveReplay(t *testing.T) {
	t.Parallel()

	h, testinfra := setupTestHandler(t, nil, itest.RWPostgres)
	defer testinfra.Close()

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay/move-list?replayId=%d", 1), nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	body, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var pbMoveHist pb.MoveHistory
	require.NoError(t, proto.Unmarshal(body, &pbMoveHist))

	wantMoveReplay := itest.TestPbMoveHistory

	assert.Equal(t, http.StatusOK, w.Code)
	testutil.Equal(t, wantMoveReplay, &pbMoveHist, protocmp.Transform())
}

func TestGetTournament(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tournamentKey string
		wantResp      GetTournamentResp
		wantStatus    int
		wantFail      ServiceResp
	}{
		{
			name:          "GotTournament",
			tournamentKey: itest.Tournament0LobbyKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: GetTournamentResp{
				Tournament:   itest.Tournaments[0],
				Participants: []model.Participant{},
				Matches:      []model.FullMatch{},
			},
		},
		{
			name:          "RetrieveFullTournament",
			tournamentKey: itest.Tournament9InProgressUncompletedKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: GetTournamentResp{
				Tournament:   itest.Tournaments[9],
				Participants: []model.Participant{},
				Matches:      itest.MatchesTournament9, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "TournamentNotFound",
			tournamentKey: uuid.NewString(),
			wantStatus:    http.StatusNotFound,
			wantFail: ServiceResp{
				Status: http.StatusNotFound,
				Error:  ErrHttpNotFoundTournament.Error(),
			},
		},
		{
			name:          "InvalidTournamentKey",
			tournamentKey: "invalid",
			wantStatus:    http.StatusBadRequest,
			wantFail: ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]OneError{
					"tournamentKey": {Error: ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres, itest.Redis)
			defer testinfra.Close()

			for _, change := range itest.TournamentLbdChangeSets {
				createLeaderboard(t, testinfra.Redis, change)
			}

			q := url.Values{}
			q.Set("tournamentKey", tt.tournamentKey)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tournament?%s", q.Encode()), nil)

			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if w.Code == http.StatusOK {
				testutil.AssertRespBody(t, tt.wantResp, w,
					cmpopts.IgnoreFields(model.FullMatch{}, "Ordering"),
					cmpopts.IgnoreFields(model.Replay{}, "ID"))
			} else {
				testutil.AssertRespBody(t, tt.wantFail, w, serviceViewCmpOpts...)
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
		wantFail   ServiceResp
	}{
		{
			name:       "RetrievedTournaments",
			userID:     "",
			afterID:    "",
			wantStatus: http.StatusOK,
			wantResp: GetTournamentsResp{
				Tournaments: []model.Tournament{
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
			afterID:    "",
			wantStatus: http.StatusOK,
			wantResp: GetTournamentsResp{
				Tournaments: []model.Tournament{
					itest.Tournaments[5],
					itest.Tournaments[2],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.ROPostgres)
			defer testinfra.Close()

			q := url.Values{}
			q.Set("userId", tt.userID)
			q.Set("afterId", tt.afterID)
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tournaments?%s", q.Encode()), nil)

			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}
