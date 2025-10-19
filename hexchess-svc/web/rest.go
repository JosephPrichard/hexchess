package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/data"
	"hexchess-svc/logs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

func handler(stores data.Stores, h func(w http.ResponseWriter, r *http.Request, stores data.Stores) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := uuid.NewString()
		r = r.WithContext(context.WithValue(r.Context(), logs.TraceKey, trace))

		slog.InfoContext(r.Context(), "request received", "trace", trace, "method", r.Method, "url", r.URL)

		if err := h(w, r, stores); err != nil {
			status, m := HttpStatusFromError(err)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(m))
		}
	})
}

func Handle(stores data.Stores) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/register", handler(stores, HandleRegister))
	mux.Handle("/api/login", handler(stores, HandleLogin))
	mux.Handle("/api/session/temp", handler(stores, HandleCreateTempSession))
	mux.Handle("/api/session/refresh", handler(stores, HandleRefreshSession))
	mux.Handle("/api/session/logout", handler(stores, HandleLogout))
	mux.Handle("/api/users/password", handler(stores, HandleUpdatePassword))
	mux.Handle("/api/users", handler(stores, HandleUpdateUser))
	mux.Handle("/api/leaderboard", handler(stores, HandleGetLeaderboard))
	mux.Handle("/api/players", handler(stores, HandleGetPlayer))
	mux.Handle("/api/self", handler(stores, HandleGetSelf))
	return mux
}

const PerPage = 25

type RegisterBody struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func validatePassword(password, confirm string) error {
	if len(password) < 10 || len(password) > 100 {
		return ErrHttpInvalidPassword
	}
	if password != confirm {
		return ErrHttpConfirmPassword
	}
	return nil
}

func HandleRegister(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	var body RegisterBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpUnknown
	}
	if len(body.Username) < 5 || len(body.Username) > 20 {
		return ErrHttpInvalidUsername
	}
	if err := validatePassword(body.Password, body.ConfirmPassword); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := data.InsertUser(ctx, stores.Q, data.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  "us",
		Elo:      data.StartElo,
		Wins:     0,
		Losses:   0,
	})
	if errors.Is(err, data.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	}
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}
	t, err := SetSessionPlayer(ctx, stores.Rdb, w, data.PlayerState{
		ID:      user.ID,
		Name:    user.Username,
		Country: user.Country,
		Elo:     user.Elo,
		IsGuest: false,
	})
	if err != nil {
		return fmt.Errorf("failed to set session player: %w", err)
	}
	slog.InfoContext(ctx, "registered user", "user", user)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
		Elo:      user.Elo,
		TTLSecs:  t,
	})
	return nil
}

type LoginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HandleLogin(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	var body LoginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpUnknown
	}

	ctx := r.Context()
	user, err := data.VerifyUser(ctx, stores.Q, body.Username, body.Password)
	if errors.Is(err, data.ErrUserNotFound) {
		return ErrHttpUserNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}
	t, err := SetSessionPlayer(ctx, stores.Rdb, w, data.PlayerState{
		ID:      user.ID,
		Name:    user.Username,
		Country: user.Country,
		Elo:     user.Elo,
		IsGuest: false,
	})
	if err != nil {
		return fmt.Errorf("failed to set session player: %w", err)
	}
	slog.InfoContext(ctx, "user has logged in", "user", user)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
		Elo:      user.Elo,
		TTLSecs:  t,
	})
	return nil
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func HandleUpdatePassword(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	var body UpdatePasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpUnknown
	}
	if err := validatePassword(body.NewPassword, body.ConfirmNewPassword); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, stores.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	user, err := data.VerifyUser(ctx, stores.Q, player.Name, body.Password)
	if errors.Is(err, data.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	}
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}
	if err := data.UpdateUserPassword(ctx, stores.Q, user.ID, body.NewPassword); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}
	slog.InfoContext(ctx, "user has updated password", "user", user)

	writeSuccessJSON(w)
	return nil
}

type UpdateUserBody struct {
	NewUsername string `json:"newUsername"`
	NewCountry  string `json:"newCountry"`
	NewBio      string `json:"newBio"`
}

func HandleUpdateUser(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	var body UpdateUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpUnknown
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, stores.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	if err := data.UpdateUser(ctx, stores.Q, player.ID, data.UpdtUserParams{
		Username: body.NewUsername,
		Bio:      body.NewBio,
		Country:  body.NewCountry,
	}); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	slog.InfoContext(ctx, "user has updated username", "user", player)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       player.ID,
		Username: player.Name,
		Country:  player.Country,
		Elo:      player.Elo,
	})
	return nil
}

type TempSessionResp struct {
	SessionID string `json:"sessionId"`
}

func HandleCreateTempSession(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, stores.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	sessionID, err := MakeSessionID()
	if err != nil {
		return err
	}
	if err := data.SetSession(ctx, stores.Rdb, sessionID, player, MaxAgeCookie); err != nil {
		return fmt.Errorf("failed to set session: %w", err)
	}

	slog.InfoContext(ctx, "created temporary user session", "user", player)

	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: sessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func HandleRefreshSession(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	ctx := r.Context()
	player, sessionID, err := GetSessionPlayer(ctx, stores.Rdb, r)
	if errors.Is(err, data.ErrNoSession) {
		writeEmptyRefreshJSON(w)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	if err := data.UpdateSessionEx(ctx, stores.Rdb, sessionID, MaxAgeCookie); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	slog.InfoContext(ctx, "refreshed user session", "user", player)

	writeJSON(w, http.StatusOK, RefreshResp{Session: &SessionView{
		ID:       player.ID,
		Username: player.Name,
		Country:  player.Country,
		Elo:      player.Elo,
		TTLSecs:  MaxAgeCookie,
	}})
	return nil
}

func HandleLogout(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return err
	}
	sessionID := cookie.Value

	ctx := r.Context()
	if err := data.DeleteSession(ctx, stores.Rdb, sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeSuccessJSON(w)
	return nil
}

type CreateGameBody struct {
	FirstColor  data.ColorSelect `json:"firstColor"`
	TimeControl data.TimeControl `json:"timeControl"`
}

type CreateGameResp struct {
	GameID string `json:"gameId"`
}

func HandleCreateGame(w http.ResponseWriter, r *http.Request, d data.GameplayDAL) error {
	var body CreateGameBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpUnknown
	}

	gameID, err := CreateGame(r.Context(), d, body.FirstColor, body.TimeControl)
	if err != nil {
		return fmt.Errorf("failed to create game: %w", err)
	}
	slog.InfoContext(r.Context(), "created game", "gameId", gameID)

	writeJSON(w, http.StatusOK, CreateGameResp{GameID: gameID})
	return nil
}

type UpdateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeId"`
	ChallengerID int64  `json:"accept"`
	Action       string `json:"action"`
}

type UpdateChallengeResp struct {
	GameID string `json:"challengeId"`
}

func UpdateChallenge(w http.ResponseWriter, r *http.Request, stores data.Stores, d data.GameplayDAL) error {
	var body UpdateChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpUnknown
	}
	body.Action = strings.ToUpper(body.Action)

	var targetID int64
	switch body.Action {
	case "ACCEPT", "REJECT":
		targetID = body.ChallengeeID
	case "DELETE":
		targetID = body.ChallengerID
	default:
		return ErrHttpInvalidRequest
	}

	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, stores.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	if player.ID != targetID {
		return ErrHttpUpdateChallenge
	}

	dr, err := data.DeleteChallenge(ctx, stores.Q, body.ChallengerID, body.ChallengeeID)
	if errors.Is(err, data.ErrChallengeNotFound) {
		return ErrHttpNotFoundChallenge
	}
	if err != nil {
		return fmt.Errorf("failed to delete challenge: %w", err)
	}
	slog.InfoContext(ctx, "deleted challenge", "challengerID", body.ChallengerID, "challengeeID", body.ChallengeeID)

	gameID := ""
	if body.Action == "ACCEPT" {
		gameID, err = CreateGame(ctx, d, dr.FirstColor, dr.TimeControl)
		if err != nil {
			return fmt.Errorf("failed to create game: %w", err)
		}
		slog.InfoContext(ctx, "created game", "gameId", gameID)
	}

	writeJSON(w, http.StatusOK, UpdateChallengeResp{GameID: gameID})
	return nil
}

type CreateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeId"`
	FirstColor   string `json:"firstColor"`
	TimeControl  string `json:"timeControl"`
}

func HandleCreateChallenge(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	return nil
}

func HandleGetSelf(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, stores.Rdb, r)
	if errors.Is(err, data.ErrNoSession) {
		writeEmptyRefreshJSON(w)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}

	user, err := data.GetUserById(ctx, stores.Q, player.ID)
	if err != nil {
		return fmt.Errorf("failed to get user by id: %w", err)
	}
	slog.InfoContext(ctx, "retrieved user", "user", user)

	writeJSON(w, http.StatusOK, user)
	return nil
}

type LeaderboardResp struct {
	TotalPages int               `json:"totalPages"`
	UserList   []data.UserEntity `json:"userList,omitempty"`
}

func HandleGetLeaderboard(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	var page int64
	strPage := r.URL.Query().Get("page")
	if strPage != "" {
		p, err := strconv.ParseInt(strPage, 10, 64)
		if err != nil {
			return ErrHttpInvalidRequest
		}
		page = p
	}

	ctx := r.Context()
	lbd, err := data.GetLeaderboardPage(ctx, stores.Rdb, page, PerPage)
	if err != nil {
		return fmt.Errorf("failed to get leaderboard page: %w", err)
	}
	users, err := data.GetRankedUsers(ctx, stores.Q, lbd.Users)
	if err != nil {
		return fmt.Errorf("failed to get ranked users: %w", err)
	}
	if err := data.JoinRanks(lbd.Users, users); err != nil {
		return err
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	writeJSON(w, http.StatusOK, LeaderboardResp{TotalPages: lbd.PageCount, UserList: users})
	return nil
}

type UserWithReplaysResp struct {
	User       data.UserEntity     `json:"user"`
	ReplayList []data.ReplayEntity `json:"replayList,omitempty"`
}

func HandleGetPlayer(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	strID := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(strID, 10, 64)
	if err != nil {
		return ErrHttpInvalidRequest
	}

	ctx := r.Context()
	eg, egCtx := errgroup.WithContext(ctx)

	var user data.UserEntity
	var userRank int64
	var replayList []data.ReplayEntity

	eg.Go(func() error {
		u, err := data.GetUserById(egCtx, stores.Q, id)
		if err != nil {
			return fmt.Errorf("failed to get user by id: %v", err)
		}
		user = u
		rs, err := data.GetUserReplays(egCtx, stores.Q, id, -1, PerPage)
		if err != nil {
			return fmt.Errorf("failed to get replays by id: %v", err)
		}
		replayList = rs
		return nil
	})
	eg.Go(func() error {
		ur, err := data.GetLeaderboardRank(egCtx, stores.Rdb, id)
		if err != nil {
			return fmt.Errorf("failed leaderboard rank by id: %v", err)
		}
		userRank = ur
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	user.Rank = userRank

	writeJSON(w, http.StatusOK, UserWithReplaysResp{User: user, ReplayList: replayList})
	return nil
}

type SearchPlayersResp struct {
	UserList []data.UserEntity `json:"userList,omitempty"`
}

func HandleSearchPlayers(w http.ResponseWriter, r *http.Request, stores data.Stores) error {
	page, err := strconv.ParseInt(r.URL.Query().Get("page"), 10, 64)
	if err != nil {
		return ErrHttpInvalidRequest
	}
	name := r.URL.Query().Get("name")

	var userList []data.UserEntity

	ctx := r.Context()
	if name != "" {
		userList, err = data.SearchUsersByName(ctx, stores.Q, name, int32(page), PerPage)
		if err != nil {
			return fmt.Errorf("failed to search users by name: %w", err)
		}
	}

	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: userList})
	return nil
}
