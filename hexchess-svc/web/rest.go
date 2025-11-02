package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/data"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RestHandler = func(w http.ResponseWriter, r *http.Request, state ServerState) error

func makeRestHandler(state ServerState, h RestHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "request received", "method", r.Method, "url", r.URL)

		if err := h(w, r, state); err != nil {
			slog.ErrorContext(r.Context(), "request failed", "method", r.Method, "url", r.URL, "error", err)

			status, m := HttpStatusFromErr(err)
			w.WriteHeader(status)

			b, err := json.Marshal(ServiceView{Message: m, Status: status})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write(FatalErrorJSON)
			} else {
				_, _ = w.Write(b)
			}
		}
	})
}

func makeJsonHandler(v any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(v)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})
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

func HandleRegister(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body RegisterBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}
	if len(body.Username) < 5 || len(body.Username) > 20 {
		return ErrHttpInvalidUsername
	}
	if err := validatePassword(body.Password, body.ConfirmPassword); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := data.InsertUser(ctx, state.Q, data.UserInst{
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
	t, err := SetSessionPlayer(ctx, state.Rdb, w, data.PlayerState{
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

func HandleLogin(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body LoginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := data.VerifyUserTx(ctx, state.PgDB, body.Username, body.Password)
	if errors.Is(err, data.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if errors.Is(err, data.ErrTooManyLoginAttempts) {
		return ErrHttpTooManyLoginAttempts
	} else if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	t, err := SetSessionPlayer(ctx, state.Rdb, w, data.PlayerState{
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

func HandleUpdatePassword(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body UpdatePasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}
	if err := validatePassword(body.NewPassword, body.ConfirmNewPassword); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	user, err := data.VerifyUserTx(ctx, state.PgDB, player.Name, body.Password)
	if errors.Is(err, data.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	}
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}
	if err := data.UpdateUserPassword(ctx, state.Q, user.ID, body.NewPassword); err != nil {
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

func HandleUpdateUser(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body UpdateUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	if _, ok := state.CountryMap[body.NewCountry]; !ok {
		return ErrHttpInvalidCountry
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	user, err := data.UpdateUser(ctx, state.Q, player.ID, data.UpdtUserParams{
		Username: body.NewUsername,
		Bio:      body.NewBio,
		Country:  body.NewCountry,
	})
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	slog.InfoContext(ctx, "user has updated username", "user", player)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
		Elo:      user.Elo,
	})
	return nil
}

type TempSessionResp struct {
	SessionID string `json:"sessionID"`
}

func HandleCreateTempSession(w http.ResponseWriter, r *http.Request, state ServerState) error {
	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	sessionID, err := MakeSessionID()
	if err != nil {
		return err
	}
	if err := data.SetSession(ctx, state.Rdb, sessionID, player, TempSessionMaxAge); err != nil {
		return fmt.Errorf("failed to set session: %w", err)
	}

	slog.InfoContext(ctx, "created temporary user session", "user", player)

	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: sessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func HandleRefreshSession(w http.ResponseWriter, r *http.Request, state ServerState) error {
	ctx := r.Context()
	player, sessionID, err := GetSessionPlayer(ctx, state.Rdb, r)
	if errors.Is(err, data.ErrSessionNotFound) {
		writeEmptyRefreshJSON(w)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	if err := data.UpdateSessionEx(ctx, state.Rdb, sessionID, SessionMaxAge); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	slog.InfoContext(ctx, "refreshed user session", "user", player)

	writeJSON(w, http.StatusOK, RefreshResp{Session: &SessionView{
		ID:       player.ID,
		Username: player.Name,
		Country:  player.Country,
		Elo:      player.Elo,
		TTLSecs:  SessionMaxAge,
	}})
	return nil
}

func HandleLogout(w http.ResponseWriter, r *http.Request, state ServerState) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return err
	}
	sessionID := cookie.Value

	ctx := r.Context()
	if err := data.DeleteSession(ctx, state.Rdb, sessionID); err != nil {
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
	GameID string `json:"gameID"`
}

func HandleCreateGame(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body CreateGameBody

	gameID, err := data.CreateGame(r.Context(), state.Rdb, body.FirstColor, body.TimeControl)
	if err != nil {
		return fmt.Errorf("failed to create game: %w", err)
	}
	slog.InfoContext(r.Context(), "created game", "gameID", gameID)

	writeJSON(w, http.StatusOK, CreateGameResp{GameID: gameID})
	return nil
}

type UpdateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeID"`
	ChallengerID int64  `json:"challengerID"`
	Action       string `json:"action"`
}

type UpdateChallengeResp struct {
	GameID string `json:"challengeID"`
}

func HandleUpdateChallenge(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body UpdateChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
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
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	if player.ID != targetID {
		return ErrHttpUpdateChallenge
	}

	dr, err := data.DeleteChallenge(ctx, state.Q, data.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if errors.Is(err, data.ErrChallengeNotFound) {
		return ErrHttpNotFoundChallenge
	}
	if err != nil {
		return fmt.Errorf("failed to delete challenge: %w", err)
	}
	slog.InfoContext(ctx, "deleted challenge", "challengerID", body.ChallengerID, "challengeeID", body.ChallengeeID)

	gameID := ""
	if body.Action == "ACCEPT" {
		gameID, err = data.CreateGame(ctx, state.Rdb, dr.FirstColor, dr.TimeControl)
		if err != nil {
			return fmt.Errorf("failed to create game: %w", err)
		}
		slog.InfoContext(ctx, "created game", "gameID", gameID)
	}

	writeJSON(w, http.StatusOK, UpdateChallengeResp{GameID: gameID})
	return nil
}

type CreateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeID"`
	StartColor   string `json:"startColor"`
	TimeControl  string `json:"timeControl"`
}

func HandleCreateChallenge(w http.ResponseWriter, r *http.Request, state ServerState) error {
	var body CreateChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}

	tc, err := data.ParseTimeControl(body.TimeControl)
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}
	fc, err := data.ParseColorSelect(body.StartColor)
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}
	ret, err := data.InsertChallengeRet(ctx, state.Q, data.ChallengeInst{
		ChallengerID: player.ID,
		ChallengeeID: body.ChallengeeID,
		TimeControl:  tc,
		StartColor:   fc,
		MadeOn:       time.Now(),
	})
	if errors.Is(err, data.ErrDuplicateChallenge) {
		return ErrHttpDuplicateChallenge
	} else if errors.Is(err, data.ErrParticipantConflict) {
		return ErrHttpInvalidParticipants
	} else if errors.Is(err, data.ErrSelfChallenge) {
		return ErrHttpSelfChallenge
	} else if err != nil {
		return fmt.Errorf("failed to insert challenge: %w", err)
	}
	writeSuccessJSON(w)

	ctx = context.WithoutCancel(ctx)
	if err := data.BroadcastChallenge(ctx, state.Rdb, player.ID, ret); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast challenge", "challenge", ret, "err", err)
	}
	if err := data.DeleteExpiredChallenges(ctx, state.Q, player.ID, data.ExpireChallengeThreshold); err != nil {
		slog.ErrorContext(ctx, "failed to delete expired challenges", "challenge", ret, "err", err)
	}
	return nil
}

func HandleGetSelf(w http.ResponseWriter, r *http.Request, state ServerState) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if errors.Is(err, data.ErrSessionNotFound) {
		writeEmptyRefreshJSON(w)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}

	user, err := data.GetUserByID(ctx, state.Q, player.ID)
	if err != nil {
		return fmt.Errorf("failed to get user by id %+v: %w", player, err)
	}
	slog.InfoContext(ctx, "retrieved user", "user", user)

	writeJSON(w, http.StatusOK, user)
	return nil
}

type LeaderboardResp struct {
	TotalPages int               `json:"totalPages"`
	UserList   []data.UserEntity `json:"userList,omitempty"`
}

func HandleGetLeaderboard(w http.ResponseWriter, r *http.Request, state ServerState) error {
	query := r.URL.Query()
	page, err := getPageQuery(query)
	if err != nil {
		return err
	}

	ctx := r.Context()
	lbd, err := data.GetLeaderboardPage(ctx, state.Rdb, int64(page), PerPage)
	if err != nil {
		return fmt.Errorf("failed to get leaderboard page: %w", err)
	}
	users, err := data.GetRankedUsers(ctx, state.Q, lbd.Users)
	if err != nil {
		return fmt.Errorf("failed to get ranked users: %w", err)
	}
	if err := data.JoinRanks(lbd.Users, users); err != nil {
		return err
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	if users == nil {
		users = []data.UserEntity{}
	}
	writeJSON(w, http.StatusOK, LeaderboardResp{TotalPages: lbd.PageCount, UserList: users})
	return nil
}

type FullUserResp struct {
	User       data.UserEntity     `json:"user"`
	ReplayList []data.ReplayEntity `json:"replayList,omitempty"`
}

func HandleGetPlayer(w http.ResponseWriter, r *http.Request, state ServerState) error {
	ctx := r.Context()

	strID := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(strID, 10, 64)
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}

	eg, egCtx := errgroup.WithContext(ctx)

	var user data.UserEntity
	var userRank int64
	var replayList []data.ReplayEntity

	eg.Go(func() error {
		u, err := data.GetUserByID(egCtx, state.Q, id)
		if err != nil {
			return fmt.Errorf("failed to get user by id: %w", err)
		}
		user = u
		return nil
	})
	eg.Go(func() error {
		rs, err := data.GetUserReplays(egCtx, state.Q, id, -1, PerPage)
		if err != nil {
			return fmt.Errorf("failed to get replays by id: %w", err)
		}
		replayList = rs
		return err
	})
	eg.Go(func() error {
		ur, err := data.GetLeaderboardRank(egCtx, state.Rdb, id)
		if err != nil {
			return fmt.Errorf("failed leaderboard rank by id: %w", err)
		}
		userRank = ur
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	user.Rank = userRank

	if replayList == nil {
		replayList = []data.ReplayEntity{}
	}
	slog.InfoContext(ctx, "retrieved user with replays", "user", user, "replays", replayList)
	writeJSON(w, http.StatusOK, FullUserResp{User: user, ReplayList: replayList})
	return nil
}

type SearchPlayersResp struct {
	UserList []data.UserEntity `json:"userList,omitempty"`
}

func HandleSearchPlayers(w http.ResponseWriter, r *http.Request, state ServerState) error {
	query := r.URL.Query()
	page, err := getPageQuery(query)
	if err != nil {
		return err
	}
	name := query.Get("username")
	ctx := r.Context()

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []data.UserEntity
	if name != "" {
		userList, err = data.SearchUsersByName(ctx, state.Q, name, int32(page), PerPage)
		if errors.Is(err, data.ErrSearchLimit) {
			return ErrHttpSearchLimit
		}
		if err != nil {
			return fmt.Errorf("failed to search users by name: %w", err)
		}
	}

	if userList == nil {
		userList = []data.UserEntity{}
	}
	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: userList})
	return nil
}

type GetReplayResp struct {
	Replay data.ReplayEntity `json:"replay"`
}

func HandleGetReplay(w http.ResponseWriter, r *http.Request, state ServerState) error {
	strID := r.URL.Query().Get("id")
	id, err := strconv.Atoi(strID)
	if err != nil {
		return err
	}

	ctx := r.Context()
	replay, err := data.GetReplay(ctx, state.Q, int64(id))
	if err != nil {
		return fmt.Errorf("failed to get replay: %w", err)
	}
	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

func HandleGetReplayMoveList(w http.ResponseWriter, r *http.Request, state ServerState) error {
	strID := r.URL.Query().Get("id")
	id, err := strconv.Atoi(strID)
	if err != nil {
		return err
	}

	ctx := r.Context()
	moveList, err := data.GetReplayMoveList(ctx, state.Q, int64(id))
	if err != nil {
		return fmt.Errorf("failed to get replay moveList: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(moveList)
	return nil
}

type GetUserReplaysResp struct {
	ReplayList []data.ReplayEntity `json:"replayList"`
}

func HandleGetUserReplays(w http.ResponseWriter, r *http.Request, state ServerState) error {
	ctx := r.Context()

	query := r.URL.Query()
	afterID, err := strconv.Atoi(query.Get("afterID"))
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}
	userID, err := strconv.Atoi(query.Get("userID"))
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}
	replays, err := data.GetUserReplays(r.Context(), state.Q, int64(userID), int64(afterID), PerPage)
	if err != nil {
		return fmt.Errorf("failed to get user replays: %w", err)
	}

	if replays == nil {
		replays = []data.ReplayEntity{}
	}
	writeJSON(w, http.StatusOK, GetUserReplaysResp{ReplayList: replays})
	return nil
}

type GetChallengesResp struct {
	ChallengeList []data.ChallengeEntity `json:"challengeList"`
}

func HandleGetChallenges(w http.ResponseWriter, r *http.Request, state ServerState) error {
	participants := r.URL.Query().Get("participants")

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("failed to get session player: %w", err)
	}

	var challengeList []data.ChallengeEntity
	switch participants {
	case "sent":
		challengeList, err = data.GetChallengesByParticipant(ctx, state.Q, data.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1}, data.ExpireChallengeThreshold)
	case "received":
		challengeList, err = data.GetChallengesByParticipant(ctx, state.Q, data.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID}, data.ExpireChallengeThreshold)
	default:
		return handleInvalidRequest(ctx, err)
	}
	if err != nil {
		return fmt.Errorf("failed to get challenges by participant: %w", err)
	}

	if challengeList == nil {
		challengeList = []data.ChallengeEntity{}
	}
	slog.InfoContext(ctx, "retrieved challenges", "challengeList", challengeList)
	writeJSON(w, http.StatusOK, GetChallengesResp{ChallengeList: challengeList})
	return nil
}

type ChessRoomListResp struct {
	ChessList     []data.ChessMeta `json:"chessList"`
	SelfChessList []data.ChessMeta `json:"selfChessList"`
}

func HandleGetChessRoomList(w http.ResponseWriter, r *http.Request, state ServerState) error {
	ctx := r.Context()

	query := r.URL.Query()
	page, err := getPageQuery(query)
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}
	count, err := getCountQuery(query)
	if err != nil {
		return handleInvalidRequest(ctx, err)
	}

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil && err != data.ErrSessionNotFound {
		return fmt.Errorf("failed to get session player: %w", err)
	}
	hasSession := err != data.ErrSessionNotFound

	chessList, err := data.GetAllChessMetas(ctx, state.Rdb, page, count)
	if err != nil {
		return fmt.Errorf("failed to get all chess meta views: %w", err)
	}
	var selfChessList []data.ChessMeta
	if hasSession {
		selfChessList, err = data.GetUserChessMetas(ctx, state.Rdb, player.ID)
		if err != nil {
			return fmt.Errorf("failed to get all chess meta views: %w", err)
		}
	}

	if chessList == nil {
		chessList = []data.ChessMeta{}
	}
	if selfChessList == nil {
		selfChessList = []data.ChessMeta{}
	}
	writeJSON(w, http.StatusOK, ChessRoomListResp{ChessList: chessList, SelfChessList: selfChessList})
	return nil
}
