package network_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/network"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/opt"

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
	"hexchess-svc/utils/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var serviceViewCmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(network.ServiceResp{}, "Message"),
	cmpopts.IgnoreFields(network.OneError{}, "Message"),
}

var testSessionViewCmpOpts = cmpopts.IgnoreFields(network.SessionView{}, "ID", "TTLSecs")

func asJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}

func TestHandleRegister(t *testing.T) {
	tests := []struct {
		name        string
		body        network.RegisterBody
		wantSuccess network.SessionView
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:        "ValidUsername",
			body:        network.RegisterBody{Username: "testing-name", Password: "testing-password", ConfirmPassword: "testing-password"},
			wantSuccess: network.SessionView{Username: "testing-name", Country: "un"},
			wantStatus:  http.StatusOK,
		},
		{
			name:       "InvalidPasswordConfirmDoesNotMatch",
			body:       network.RegisterBody{Username: "testing-name1", Password: "testing-password1", ConfirmPassword: "wrong"},
			wantFail:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpConfirmPassword.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "InvalidUsernameAndPasswordTooShort",
			body: network.RegisterBody{Username: "s", Password: "short", ConfirmPassword: "short"},
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"RegisterBody.Username": {Error: network.ErrHttpInvalidUsername.Error()},
					"RegisterBody.Password": {Error: network.ErrHttpInvalidPassword.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidUsernameDuplicate",
			body:       network.RegisterBody{Username: itest.UsersInsts[0].Username, Password: "testing-password2", ConfirmPassword: "testing-password2"},
			wantFail:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpDuplicateUsername.Error()},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
	user := itest.UsersInsts[0]

	tests := []struct {
		name        string
		body        network.LoginBody
		wantSuccess network.SessionView
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:       "InvalidLogin",
			body:       network.LoginBody{Username: "testing-name", Password: "testing-password"},
			wantFail:   network.ServiceResp{Status: http.StatusUnauthorized, Error: network.ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "ValidLogin",
			body:        network.LoginBody{Username: user.Username, Password: "password1"},
			wantSuccess: network.SessionView{Username: user.Username, Country: "us"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
	apiKey := "API_KEY"

	tests := []struct {
		name        string
		runCount    int
		setupMocks  func(*gomock.Controller) cloud.SDKs
		body        network.GoogleLoginBody
		wantSuccess network.SessionView
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:     "InvalidLoginTokenMocked",
			runCount: 1,
			setupMocks: func(ctrl *gomock.Controller) cloud.SDKs {
				mockValidator := cloud.NewMockGoogleTokenValidator(ctrl)

				mockValidator.EXPECT().
					Validate(gomock.Any(), "invalidToken123", apiKey).
					Return(&idtoken.Payload{}, errors.New("invalid token"))

				return cloud.NewOptRemoteAPIs(cloud.WithGoogleIDTokenValidator(mockValidator, apiKey))
			},
			body:       network.GoogleLoginBody{Token: "invalidToken123"},
			wantFail:   network.ServiceResp{Status: http.StatusInternalServerError, Error: network.ErrHttpFatal.Error()},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:     "LoginWithGoogleTokenSuccessful",
			runCount: 2, // the user is created the first time, the second time we log in with the already inserted account key
			setupMocks: func(ctrl *gomock.Controller) cloud.SDKs {
				mockValidator := cloud.NewMockGoogleTokenValidator(ctrl)

				mockValidator.EXPECT().
					Validate(gomock.Any(), "testToken123", apiKey).
					Return(&idtoken.Payload{Subject: "account1", Claims: map[string]any{"email": "email@domain.com"}}, nil).
					Times(2)

				return cloud.NewOptRemoteAPIs(cloud.WithGoogleIDTokenValidator(mockValidator, apiKey))
			},
			body:        network.GoogleLoginBody{Token: "testToken123"},
			wantSuccess: network.SessionView{Username: "email@domain.com", Country: "un"},
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

			h, testinfra := setupMuxTest(t, mocks)
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
	tests := []struct {
		name        string
		body        network.UpdateUserBody
		sessionID   string
		wantSuccess network.SessionView
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:       "UnauthorizedUser",
			body:       network.UpdateUserBody{NewUsername: "username"},
			sessionID:  "invalid",
			wantFail:   network.ServiceResp{Status: http.StatusUnauthorized, Error: network.ErrHttpSessionExpired.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:      "InvalidUsernameAndBiographyLength",
			body:      network.UpdateUserBody{NewCountry: "us", NewUsername: "s", NewBio: strings.Repeat("a", 5001)},
			sessionID: TestSessionID1,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"UpdateUserBody.NewBio":      {Error: network.ErrHttpInvalidBio.Error()},
					"UpdateUserBody.NewUsername": {Error: network.ErrHttpInvalidUsername.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:      "InvalidCountryUnknown",
			body:      network.UpdateUserBody{NewCountry: "wrong", NewUsername: "new-username", NewBio: "testing biography"},
			sessionID: TestSessionID1,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"UpdateUserBody.NewCountry": {Error: network.ErrHttpInvalidCountry.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "UpdatingBioOnly",
			body:        network.UpdateUserBody{NewBio: "testing biography"},
			sessionID:   TestSessionID1,
			wantSuccess: network.SessionView{Username: "user1", Country: "us"},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "UpdatingUsernameBioAndCountry",
			body:        network.UpdateUserBody{NewUsername: "new-username", NewBio: "testing biography", NewCountry: "un"},
			sessionID:   TestSessionID1,
			wantSuccess: network.SessionView{Username: "new-username", Country: "un"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/users", asJSONReader(tt.body))
			r.Header.Set("Cookie", network.FmtCookie(tt.sessionID))
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
	tests := []struct {
		name        string
		body        network.UpdatePasswordBody
		wantSuccess network.ServiceResp
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:       "InvalidLogin",
			body:       network.UpdatePasswordBody{Password: "password2", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantFail:   network.ServiceResp{Status: http.StatusUnauthorized, Error: network.ErrHttpInvalidLogin.Error()},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "InvalidPasswordLength",
			body: network.UpdatePasswordBody{Password: "password1", NewPassword: "short", ConfirmNewPassword: "short"},
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"UpdatePasswordBody.NewPassword": {Error: network.ErrHttpInvalidPassword.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidPasswordConfirmDoesNotMatch",
			body:       network.UpdatePasswordBody{Password: "password1", NewPassword: "testing-password1", ConfirmNewPassword: "testing-password"},
			wantFail:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpConfirmPassword.Error()},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "ValidPasswordUpdate",
			body:        network.UpdatePasswordBody{Password: "password1", NewPassword: "testing-password", ConfirmNewPassword: "testing-password"},
			wantSuccess: network.ServiceResp{Status: http.StatusOK, Message: "SUCCESS"},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/users/password", asJSONReader(tt.body))
			r.Header.Set("Cookie", network.FmtCookie(TestSessionID1))
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
	tests := []struct {
		name       string
		body       network.UpdateChallengeBody
		sessionID  string
		wantFail   network.ServiceResp
		wantStatus int
	}{
		{
			name:       "UnauthorizedUser",
			body:       network.UpdateChallengeBody{Action: "DELETE"},
			wantFail:   network.ServiceResp{Status: http.StatusUnauthorized, Error: network.ErrHttpSessionExpired.Error()},
			sessionID:  "invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "InvalidChallengeAction",
			body: network.UpdateChallengeBody{Action: "invalid"},
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{"UpdateChallengeBody.Action": {Error: network.ErrHttpInvalidInput.Error()}},
			},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "InvalidChallenge",
			body:       network.UpdateChallengeBody{ChallengerID: 999, ChallengeeID: 1, Action: "ACCEPT"},
			wantFail:   network.ServiceResp{Status: http.StatusNotFound, Error: network.ErrHttpNotFoundChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "InvalidDeleteChallengeCannotDeleteNotOwn",
			body:       network.UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "DELETE"},
			wantFail:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpUpdateChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ValidDeleteChallenge",
			body:       network.UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "DELETE"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidAcceptChallengeCannotAcceptOwn",
			body:       network.UpdateChallengeBody{ChallengerID: 1, ChallengeeID: 2, Action: "ACCEPT"},
			wantFail:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpUpdateChallenge.Error()},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ValidAcceptChallenge",
			body:       network.UpdateChallengeBody{ChallengerID: 5, ChallengeeID: 1, Action: "ACCEPT"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/update", asJSONReader(tt.body))
			r.Header.Set("Cookie", network.FmtCookie(tt.sessionID))
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
	tests := []struct {
		name       string
		body       network.CreateGameBody
		wantStatus int
		wantFail   network.ServiceResp
	}{
		{
			name:       "CreatedGame",
			body:       network.CreateGameBody{FirstColor: "WHITE", Mode: "CORRESPONDENCE_1"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "InvalidModeColorAndInitialFen",
			body:       network.CreateGameBody{InitialFEN: "invalid", FirstColor: "invalid", Mode: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"CreateGameBody.FirstColor": {Error: network.ErrHttpInvalidInput.Error()},
					"CreateGameBody.Mode":       {Error: network.ErrHttpInvalidInput.Error()},
					"CreateGameBody.InitialFen": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := &serviceMocks{
				Entropy: &entropy.StableSource{CurrTime: itest.TimeNow},
			}

			h, testinfra := setupMuxTest(t, setup)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/games/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", network.FmtCookie(TestSessionID1))
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
	tests := []struct {
		name       string
		body       network.CreateChallengeBody
		sessionID  string
		wantStatus int
		wantResp   network.ServiceResp
	}{
		{
			name:       "UnauthorizedUser",
			body:       network.CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: "CORRESPONDENCE_1"},
			sessionID:  "invalid",
			wantStatus: http.StatusUnauthorized,
			wantResp:   network.ServiceResp{Status: http.StatusUnauthorized, Error: network.ErrHttpSessionExpired.Error()},
		},
		{
			name:       "ChallengingSelf",
			body:       network.CreateChallengeBody{ChallengeeID: 1, StartColor: "WHITE", Mode: "CORRESPONDENCE_1"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpSelfChallenge.Error()},
		},
		{
			name:       "ChallengingInvalidUser",
			body:       network.CreateChallengeBody{ChallengeeID: 999, StartColor: "WHITE", Mode: "CORRESPONDENCE_1"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpInvalidParticipants.Error()},
		},
		{
			name:       "CreatingDuplicateChallenge",
			body:       network.CreateChallengeBody{ChallengeeID: 2, StartColor: "WHITE", Mode: "CORRESPONDENCE_1"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp:   network.ServiceResp{Status: http.StatusBadRequest, Error: network.ErrHttpDuplicateChallenge.Error()},
		},
		{
			name:       "CreatedChallenge",
			body:       network.CreateChallengeBody{ChallengeeID: 4, StartColor: "WHITE", Mode: "CORRESPONDENCE_1"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusOK,
			wantResp:   network.ServiceResp{Status: http.StatusOK, Message: "SUCCESS"},
		},
		{
			name:       "InvalidModeAndColor",
			body:       network.CreateChallengeBody{ChallengeeID: 4, StartColor: "invalid", Mode: "invalid"},
			sessionID:  TestSessionID1,
			wantStatus: http.StatusBadRequest,
			wantResp: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"CreateChallengeBody.Mode":       {Error: network.ErrHttpInvalidInput.Error()},
					"CreateChallengeBody.StartColor": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := &serviceMocks{
				Entropy: &entropy.StableSource{CurrTime: itest.TimeNow},
			}
			h, testinfra := setupMuxTest(t, mocks)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/challenges/create", asJSONReader(tt.body))
			r.Header.Set("Cookie", network.FmtCookie(tt.sessionID))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w, serviceViewCmpOpts...)
		})
	}
}

func TestHandleSearchPlayers(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		page        string
		wantStatus  int
		wantSuccess network.SearchPlayersResp
		wantFail    network.ServiceResp
	}{
		{
			name:       "SearchPlayers_Postgres",
			username:   "john",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchPlayersResp{
				UserList: []model.LbdUser{
					{
						ID:         8,
						Username:   "john",
						Country:    "us",
						Bio:        "",
						JoinedOn:   time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
						Elo:        1500,
						HighestElo: 2000,
						Wins:       12,
						Losses:     4,
						Winrate:    66,
						Rank:       1,
					},
					{
						ID:         9,
						Username:   "johnny",
						Country:    "us",
						Bio:        "",
						JoinedOn:   time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
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
			h, testinfra := setupMuxTest(t, nil)
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
	tests := []struct {
		name        string
		mode        string
		page        string
		wantStatus  int
		wantSuccess network.LeaderboardResp
		wantFail    network.ServiceResp
	}{
		{
			name:       "InvalidPageAndMode",
			mode:       "invalid",
			page:       "invalid",
			wantStatus: http.StatusBadRequest,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"mode": {Error: network.ErrHttpInvalidInput.Error()},
					"page": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
		},
		{
			name:       "ValidLeaderboard",
			mode:       model.ModeTimed1Plus0.String(),
			wantStatus: http.StatusOK,
			wantSuccess: network.LeaderboardResp{
				TotalPages: 1,
				UserList: []model.LbdUser{
					{
						ID:         1,
						Username:   "user1",
						Country:    "us",
						JoinedOn:   itest.TimeNow,
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
			h, testinfra := setupMuxTest(t, nil)
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

func TestGetPersona(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		wantSuccess network.GetPersonaResp
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name: "GetPlayerWithReplays",
			id:   "1",
			wantSuccess: network.GetPersonaResp{
				Persona: model.Persona{
					User: itest.TestUser[0],
					Stats: model.UserStats{
						TotalWins:    17,
						TotalLosses:  14,
						AvgElo:       1025,
						HighestElo:   1050,
						TotalWinrate: 53,
						ModeStats: []model.ModeStats{
							{Mode: model.ModeCorrespondence7, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
							{Mode: model.ModeTimed3Plus2, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
							{Mode: model.ModeTimed15Plus10, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
							{Mode: model.ModeTimed1Plus0, Wins: 6, Losses: 5, Winrate: 54, Elo: 1050, HighestElo: 1050},
						},
					},
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
			wantFail:   network.ServiceResp{Status: http.StatusNotFound, Error: network.ErrHttpNotFoundUser.Error()},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "InvalidUserID",
			id:   "testing",
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"id": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
	tests := []struct {
		name         string
		participants string
		sessionID    string
		wantSuccess  network.GetChallengesResp
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
			wantSuccess: network.GetChallengesResp{
				ChallengeList: []model.Challenge{itest.TestChallenge[0]},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:         "GotReceivedChallenges",
			participants: "received",
			sessionID:    TestSessionID1,
			wantSuccess: network.GetChallengesResp{
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
			h, testinfra := setupMuxTest(t, mocks)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/challenges?participants=%s", tt.participants), nil)
			r.Header.Set("Cookie", network.FmtCookie(tt.sessionID))
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
	tests := []struct {
		name        string
		params      string
		wantSuccess network.SearchReplaysResp
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:       "GotUserReplays_ByUserID",
			params:     "userId=1",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
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
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
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
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
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
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID",
			params:     "whiteId=1",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID_BlackID",
			params:     "whiteId=3&blackId=1",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[2],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID_WinnerID",
			params:     "whiteId=1&winnerId=1",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[3],
				itest.TestReplays[0],
			}},
		},
		{
			name:       "GotUserReplays_ByWhiteID_LoserID",
			params:     "whiteId=1&loserId=1",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[4],
			}},
		},
		{
			name:       "GotUserReplays_ByUserID_Mode_Result_Cause",
			params:     "userId=1&mode=CORRESPONDENCE_7&result=WHITE_WINS&cause=CHECKMATE",
			wantStatus: http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
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
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
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
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
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
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{
				itest.TestReplays[3],
				itest.TestReplays[2],
			}},
		},
		// tests for non-existent users
		{
			name:        "GotNoReplaysForNonexistentUser",
			params:      "userId=999",
			wantStatus:  http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{}},
		},
		{
			name:        "GotNoReplaysForNonexistentUser",
			params:      "whiteId=999",
			wantStatus:  http.StatusOK,
			wantSuccess: network.SearchReplaysResp{ReplayList: []model.FullReplay{}},
		},
		// input field validations
		{
			name:   "Invalid_UserIDs",
			params: "userId=INVALID&afterId=INVALID&whiteId=INVALID&blackId=INVALID&loserId=INVALID&winnerId=INVALID",
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"afterId":  {Error: network.ErrHttpInvalidInput.Error()},
					"userId":   {Error: network.ErrHttpInvalidInput.Error()},
					"whiteId":  {Error: network.ErrHttpInvalidInput.Error()},
					"blackId":  {Error: network.ErrHttpInvalidInput.Error()},
					"loserId":  {Error: network.ErrHttpInvalidInput.Error()},
					"winnerId": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid_ModeResultCause",
			params:     "mode=INVALID&result=INVALID&cause=INVALID",
			wantStatus: http.StatusBadRequest,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"cause":  {Error: network.ErrHttpInvalidInput.Error()},
					"mode":   {Error: network.ErrHttpInvalidInput.Error()},
					"result": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
		},
		{
			name:       "Invalid_Datetime",
			params:     "fromDate=INVALID&toDate=INVALID",
			wantStatus: http.StatusBadRequest,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"fromDate": {Error: network.ErrHttpInvalidInput.Error()},
					"toDate":   {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
	tests := []struct {
		name        string
		userID      string
		wantSuccess network.GetReplayResp
		wantFail    network.ServiceResp
		wantStatus  int
	}{
		{
			name:        "GotReplay",
			userID:      "1",
			wantStatus:  http.StatusOK,
			wantSuccess: network.GetReplayResp{Replay: itest.TestReplays[0]},
		},
		{
			name:       "GotNoReplay",
			userID:     "998877",
			wantFail:   network.ServiceResp{Status: http.StatusNotFound, Error: network.ErrHttpNotFoundReplay.Error()},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
	allChessMetas := []network.ChessMeta{
		{Ordering: 3, GameID: itest.GameID3, Mode: "CORRESPONDENCE_1"},
		{Ordering: 2, GameID: itest.GameID2, Mode: "CORRESPONDENCE_1"},
		{
			Ordering:    1,
			GameID:      itest.GameID1,
			BlackPlayer: opt.Some(model.User{ID: 2, Username: "user2", Country: "us"}),
			Mode:        "CORRESPONDENCE_1",
		},
	}

	tests := []struct {
		name       string
		sessionID  string
		wantStatus int
		wantResp   network.ChessMetasResp
	}{
		{
			name:       "ChessMetasWithSelfList",
			sessionID:  TestSessionID2,
			wantStatus: http.StatusOK,
			wantResp: network.ChessMetasResp{
				ChessList: allChessMetas,
				SelfChessList: []network.ChessMeta{
					{
						Ordering:    1,
						GameID:      itest.GameID1,
						BlackPlayer: opt.Some(model.User{ID: 2, Username: "user2", Country: "us"}),
						Mode:        "CORRESPONDENCE_1",
					},
				},
			},
		},
		{
			name:       "ChessMetasWithoutSelfList",
			sessionID:  "invalid",
			wantStatus: http.StatusOK,
			wantResp: network.ChessMetasResp{
				ChessList:     allChessMetas,
				SelfChessList: []network.ChessMeta{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodGet, "/api/game/rooms", nil)
			r.Header.Set("Cookie", network.FmtCookie(tt.sessionID))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			testutil.AssertRespBody(t, tt.wantResp, w)
		})
	}
}

func TestHandleGetMoveReplay(t *testing.T) {
	h, testinfra := setupMuxTest(t, nil)
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
	tests := []struct {
		name          string
		tournamentKey string
		wantResp      network.GetTournamentResp
		wantStatus    int
		wantFail      network.ServiceResp
	}{
		{
			name:          "GotTournament",
			tournamentKey: itest.Tournament0LobbyKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: network.GetTournamentResp{
				Tournament:   itest.Tournaments[0],
				Participants: []model.Participant{},
				Matches:      []model.FullMatch{},
			},
		},
		{
			name:          "RetrieveFullTournament",
			tournamentKey: itest.Tournament9InProgressUncompletedKey.String(),
			wantStatus:    http.StatusOK,
			wantResp: network.GetTournamentResp{
				Tournament:   itest.Tournaments[9],
				Participants: []model.Participant{},
				Matches:      itest.MatchesTournament9, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "TournamentNotFound",
			tournamentKey: uuid.NewString(),
			wantStatus:    http.StatusNotFound,
			wantFail: network.ServiceResp{
				Status: http.StatusNotFound,
				Error:  network.ErrHttpNotFoundTournament.Error(),
			},
		},
		{
			name:          "InvalidTournamentKey",
			tournamentKey: "invalid",
			wantStatus:    http.StatusBadRequest,
			wantFail: network.ServiceResp{
				Status: http.StatusBadRequest,
				Errors: map[string]network.OneError{
					"tournamentKey": {Error: network.ErrHttpInvalidInput.Error()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
	tests := []struct {
		name       string
		userID     string
		afterID    string
		wantResp   network.GetTournamentsResp
		wantStatus int
		wantFail   network.ServiceResp
	}{
		{
			name:       "RetrievedTournaments",
			userID:     "",
			afterID:    "",
			wantStatus: http.StatusOK,
			wantResp: network.GetTournamentsResp{
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
			wantResp: network.GetTournamentsResp{
				Tournaments: []model.Tournament{
					itest.Tournaments[5],
					itest.Tournaments[2],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupMuxTest(t, nil)
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
