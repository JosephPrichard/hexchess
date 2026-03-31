package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hexchess-svc/chess"
	"hexchess-svc/services"

	"golang.org/x/sync/errgroup"
)

func Rest(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.InfoContext(ctx, "received REST call", "method", r.Method, "url", r.URL, "headers", r.Header)

		if err := h(w, r); err != nil {
			resp := HttpStatusFromErrs(err)
			writeJSON(w, resp.Status, resp)

			level := slog.LevelWarn
			if resp.Status == http.StatusInternalServerError {
				level = slog.LevelError
			}
			slog.Log(ctx, level, "failed to handle REST call", "err", err, "method", r.Method, "url", r.URL)
		}
	}
}

func Json(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(v)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(b); err != nil {
			slog.ErrorContext(r.Context(), "write json", "err", err)
		}
	}
}

const (
	perPage           = 25
	minPasswordLength = 11
	minUsernameLength = 5
	maxUsernameLength = 35
	maxBioLength      = 500
)

func isPasswordValid(password string) bool {
	return len(password) >= minPasswordLength
}

func isUsernameValid(username string) bool {
	l := len(username)
	return l >= minUsernameLength && l <= maxUsernameLength
}

func isBioValid(bio string) bool {
	return len(bio) <= maxBioLength
}

type RegisterBody struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func validateRegisterBody(body RegisterBody) error {
	var respErr ResponseError
	if !isPasswordValid(body.Password) {
		respErr.Put("password", ErrHttpInvalidPassword)
	}
	if body.Password != body.ConfirmPassword {
		respErr.Put("confirmPassword", ErrHttpConfirmPassword)
	}
	if !isUsernameValid(body.Username) {
		respErr.Put("username", ErrHttpInvalidUsername)
	}
	return respErr.Interface()
}

func (server *Server) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	var body RegisterBody
	if err := parseJSON(r, &body, validateRegisterBody); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := server.Services.InsertUser(ctx, svc.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  svc.DefaultCountry,
		JoinedOn: server.EntropySource.GetNow(),
	})
	if errors.Is(err, svc.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	} else if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	ttl, err := server.SetSessionPlayer(ctx, w, svc.MakePlayer(user.ID, user.Username, user.Country))
	if err != nil {
		return fmt.Errorf("set session player: %w", err)
	}
	slog.InfoContext(ctx, "registered user", "user", user)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
		TTLSecs:  ttl,
	})
	return nil
}

func (server *Server) handleLoginSession(ctx context.Context, w http.ResponseWriter, user svc.VerifiedUserDTO) error {
	t, err := server.SetSessionPlayer(ctx, w, svc.MakePlayer(user.ID, user.Username, user.Country))
	if err != nil {
		return fmt.Errorf("set session player: %w", err)
	}
	slog.InfoContext(ctx, "user has logged in", "user", user)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
		TTLSecs:  t,
	})
	return nil
}

type LoginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (server *Server) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	var body LoginBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}

	ctx := r.Context()

	user, err := server.Services.VerifyUserTx(ctx, body.Username, body.Password)
	switch {
	case errors.Is(err, svc.ErrUserNotFound):
		return ErrHttpInvalidLogin
	case errors.Is(err, svc.ErrTooManyLoginAttempts):
		return ErrHttpTooManyLoginAttempts
	case err != nil:
		return fmt.Errorf("verify user: %w", err)
	}

	return server.handleLoginSession(ctx, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func (server *Server) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) error {
	var body GoogleLoginBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}
	ctx := r.Context()

	payload, err := server.Remote.ValidateIDToken(ctx, body.Token)
	if err != nil {
		return fmt.Errorf("validate google login id token: %w", err)
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	user, err := server.Services.SelectOrInsertGoogleUser(ctx, payload.AccountID, svc.GoogleUserInst{
		Username: payload.Username,
		Country:  svc.DefaultCountry,
	})
	if err != nil {
		return fmt.Errorf("upsert verified google user: %w", err)
	}
	return server.handleLoginSession(ctx, w, user)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func validateUpdatePasswordBody(body UpdatePasswordBody) error {
	var respErr ResponseError
	if !isPasswordValid(body.NewPassword) {
		respErr.Put("newPassword", ErrHttpInvalidPassword)
	}
	if body.NewPassword != body.ConfirmNewPassword {
		respErr.Put("confirmNewPassword", ErrHttpConfirmPassword)
	}
	return respErr.Interface()
}

func (server *Server) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	var body UpdatePasswordBody
	if err := parseJSON(r, &body, validateUpdatePasswordBody); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := server.GetSessionPlayer(ctx, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := server.Services.VerifyUserTx(ctx, player.Name, body.Password)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return fmt.Errorf("verify user: %w", err)
	}

	if err := server.Services.UpdateUserPassword(ctx, user.ID, body.NewPassword); err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	slog.InfoContext(ctx, "user has updated password", "user", user)

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type UpdateUserBody struct {
	NewUsername string `json:"newUsername"`
	NewCountry  string `json:"newCountry"`
	NewBio      string `json:"newBio"`
}

func (server *Server) validateUpdateUserBody(body UpdateUserBody) error {
	var respErr ResponseError
	if body.NewUsername != "" {
		if !isUsernameValid(body.NewUsername) {
			respErr.Put("newUsername", ErrHttpInvalidUsername)
		}
	}
	if body.NewBio != "" {
		if !isBioValid(body.NewBio) {
			respErr.Put("newBio", ErrHttpInvalidBio)
		}
	}
	if body.NewCountry != "" {
		if _, ok := server.ValidCountries[body.NewCountry]; !ok {
			respErr.Put("newCountry", ErrHttpInvalidCountry)
		}
	}
	return respErr.Interface()
}

func (server *Server) HandleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	var body UpdateUserBody
	if err := parseJSON(r, &body, server.validateUpdateUserBody); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := server.GetSessionPlayer(ctx, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := server.Services.UpdateUser(ctx, player.ID, svc.UpdtUserParams{
		Username: body.NewUsername,
		Bio:      body.NewBio,
		Country:  body.NewCountry,
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	slog.InfoContext(ctx, "user has updated username", "user", player)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
	})
	return nil
}

type TempSessionResp struct {
	SessionID string `json:"sessionId"`
}

func (server *Server) HandleCreateTempSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	alreadyHasSession := true
	var tempSessionID string

	player, _, err := server.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		alreadyHasSession = false
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	if alreadyHasSession {
		tempSessionID = MakeSessionID()
		if err := server.Services.SetSessions(ctx, svc.SessInst{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge}); err != nil {
			return fmt.Errorf("set session: %w", err)
		}
		slog.InfoContext(ctx, "created temporary user session", "user", player, "tempSessionID", tempSessionID)
	} else {
		player = svc.MakeGuest()
		tempSessionID = MakeSessionID()
		guestSessionID := MakeSessionID()

		if err := server.Services.SetSessions(ctx,
			svc.SessInst{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge},
			svc.SessInst{SessionID: guestSessionID, Player: player, Expiry: SessionMaxAge},
		); err != nil {
			return fmt.Errorf("set guest session: %w", err)
		}

		w.Header().Set("Set-Cookie", FmtCookie(guestSessionID))
		slog.InfoContext(ctx, "created guest user session", "user", player, "tempSessionID", tempSessionID, "guestSessionID", guestSessionID)
	}

	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: tempSessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func (server *Server) HandleRefreshSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, sessionID, err := server.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if err := server.Services.UpdateSessionEx(ctx, sessionID, SessionMaxAge); err != nil {
		return fmt.Errorf("update session with expiry: %d %w", SessionMaxAge, err)
	}

	w.Header().Set("Set-Cookie", FmtCookie(sessionID))
	slog.InfoContext(ctx, "refreshed user session", "user", player)

	writeJSON(w, http.StatusOK, RefreshResp{Session: &SessionView{
		ID:       player.ID,
		Username: player.Name,
		Country:  player.Country,
		TTLSecs:  SessionMaxAge,
	}})
	return nil
}

func (server *Server) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return fmt.Errorf("get cookie %s: %w", CookieKey, err)
	}
	sessionID := cookie.Value

	if err := server.Services.DeleteSession(r.Context(), sessionID); err != nil {
		return fmt.Errorf("logging ext session %s: %w", sessionID, err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type CreateGameBody struct {
	FirstColor string `json:"firstColor"`
	Mode       string `json:"mode"`
	InitialFEN string `json:"initialFen"`
}

type CreateGameArgs struct {
	FirstColor   svc.GameColor
	Mode         svc.GameMode
	InitialBoard chess.Board
}

func transformCreateGame(body CreateGameBody) (CreateGameArgs, error) {
	initialBoard := chess.InitialBoard()
	var respErr ResponseError

	if body.InitialFEN != "" {
		board, err := chess.ParseFen(body.InitialFEN)
		if err != nil {
			respErr.Put("initialFen", ErrHttpInvalidFen)
		} else {
			initialBoard = board
		}
	}
	color, ok := svc.GameColorMembers[body.FirstColor]
	if !ok {
		respErr.Put("firstColor", ErrHttpInvalidColor)
	}
	mode, ok := svc.GameModeMembers[body.Mode]
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	if respErr.HasErrors() {
		return CreateGameArgs{}, respErr.Interface()
	}

	return CreateGameArgs{
		FirstColor:   color,
		Mode:         mode,
		InitialBoard: initialBoard,
	}, nil
}

type CreateGameResp struct {
	GameID string `json:"gameId"`
}

func (server *Server) HandleCreateGame(w http.ResponseWriter, r *http.Request) error {
	body, err := transformJSON(r, transformCreateGame)
	if err != nil {
		return err
	}

	ctx := r.Context()
	gameID, err := server.Services.CreateGame(ctx, body.FirstColor, body.Mode, &body.InitialBoard)
	if err != nil {
		return fmt.Errorf("create game: %w", err)
	}
	slog.InfoContext(ctx, "created game", "gameID", gameID, "body", body)

	writeJSON(w, http.StatusOK, CreateGameResp{GameID: gameID})
	return nil
}

type UpdateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeID"`
	ChallengerID int64  `json:"challengerID"`
	Action       string `json:"action"`
}

type Action int

const (
	Accept Action = iota
	Reject
	Delete
)

type UpdateChallengeArgs struct {
	ChallengeeID int64
	ChallengerID int64
	TargetID     int64
	Action       Action
}

func transformUpdateChallenge(body UpdateChallengeBody) (UpdateChallengeArgs, error) {
	var action Action
	switch strings.ToUpper(body.Action) {
	case "ACCEPT":
		action = Accept
	case "REJECT":
		action = Reject
	case "DELETE":
		action = Delete
	default:
		return UpdateChallengeArgs{}, OneRespError("action", ErrHttpInvalidAction)
	}

	var targetID int64
	switch action {
	case Accept, Reject:
		// reject, accept means challenge is directed at challengee
		targetID = body.ChallengeeID
	default:
		// delete means challenge is directed at challenger
		targetID = body.ChallengerID
	}

	return UpdateChallengeArgs{
		ChallengeeID: body.ChallengeeID,
		ChallengerID: body.ChallengerID,
		TargetID:     targetID,
		Action:       action,
	}, nil
}

type UpdateChallengeResp struct {
	GameID string `json:"challengeID"`
}

func (server *Server) HandleUpdateChallenge(w http.ResponseWriter, r *http.Request) error {
	body, err := transformJSON(r, transformUpdateChallenge)
	if err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := server.GetSessionPlayer(ctx, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if player.ID != body.TargetID {
		return ErrHttpUpdateChallenge
	}

	deleteResult, err := server.Services.DeleteChallenge(ctx, svc.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if err != nil {
		if errors.Is(err, svc.ErrChallengeNotFound) {
			return ErrHttpNotFoundChallenge
		}
		return err
	}

	var gameID string
	if body.Action == Accept {
		gameID, err = server.Services.CreateGame(ctx, deleteResult.FirstColor, deleteResult.Mode, nil)
		if err != nil {
			return fmt.Errorf("create game: %w", err)
		}
	}

	writeJSON(w, http.StatusOK, UpdateChallengeResp{GameID: gameID})
	return nil
}

type CreateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeID"`
	StartColor   string `json:"startColor"`
	Mode         string `json:"mode"`
}

type CreateChallengeArgs struct {
	ChallengeeID int64
	StartColor   svc.GameColor
	Mode         svc.GameMode
}

func transformCreateChallenge(body CreateChallengeBody) (CreateChallengeArgs, error) {
	var respErr ResponseError
	color, ok := svc.GameColorMembers[body.StartColor]
	if !ok {
		respErr.Put("startColor", ErrHttpInvalidColor)
	}
	mode, ok := svc.GameModeMembers[body.Mode]
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	return CreateChallengeArgs{
		ChallengeeID: body.ChallengeeID,
		StartColor:   color,
		Mode:         mode,
	}, respErr.Interface()
}

func (server *Server) HandleCreateChallenge(w http.ResponseWriter, r *http.Request) error {
	body, err := transformJSON(r, transformCreateChallenge)
	if err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := server.GetSessionPlayer(ctx, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	ret, err := server.Services.InsertChallengeRet(ctx, svc.ChallengeInst{
		ChallengerID: player.ID,
		ChallengeeID: body.ChallengeeID,
		Mode:         body.Mode,
		StartColor:   body.StartColor,
		MadeOn:       server.EntropySource.GetNow(),
	})
	switch {
	case errors.Is(err, svc.ErrDuplicateChallenge):
		return ErrHttpDuplicateChallenge
	case errors.Is(err, svc.ErrParticipantConflict):
		return ErrHttpInvalidParticipants
	case errors.Is(err, svc.ErrSelfChallenge):
		return ErrHttpSelfChallenge
	case err != nil:
		return fmt.Errorf("insert challenge: %w", err)
	}

	// TODO: make this fire and forget
	if err := server.Services.BroadcastChallenge(ctx, ret); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast challenge", "challenge", ret, "err", err)
	}
	if err := server.Services.DeleteExpiredChallenges(ctx, player.ID); err != nil {
		slog.ErrorContext(ctx, "failed to delete expired challenges", "challenge", ret, "err", err)
	}

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (server *Server) HandleGetSelf(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, _, err := server.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	}
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := server.Services.GetUserByID(ctx, player.ID)
	if err != nil {
		return fmt.Errorf("get user by id %+v: %w", player, err)
	}
	slog.InfoContext(ctx, "retrieved user", "user", user)

	writeJSON(w, http.StatusOK, user)
	return nil
}

type LeaderboardArg struct {
	Page int
	Mode svc.GameMode
}

func (server *Server) getLeaderboardQuery(q url.Values) (LeaderboardArg, error) {
	var respErr ResponseError
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		respErr.Put("page", ErrHttpInvalidPage)
	}
	mode, ok := svc.GameModeMembers[q.Get("mode")]
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	return LeaderboardArg{
		Page: page,
		Mode: mode,
	}, respErr.Interface()
}

type LeaderboardResp struct {
	TotalPages int              `json:"totalPages"`
	UserList   []svc.LbdUserDTO `json:"userList,omitempty"`
}

func (server *Server) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) error {
	query, err := server.getLeaderboardQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	lbd, err := server.Services.GetLeaderboardPage(ctx, query.Mode, int64(query.Page), perPage)
	if err != nil {
		return fmt.Errorf("get leaderboard page %d: %w", query.Page, err)
	}
	users, _, err := server.Services.GetLeaderboardUsers(ctx, query.Mode, lbd.RankedUsers)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	writeJSON(w, http.StatusOK, LeaderboardResp{TotalPages: lbd.PageCount, UserList: users})
	return nil
}

const (
	GameExists    = "GAME_EXISTS"
	GameNotExists = "GAME_NOT_EXISTS"
)

func (server *Server) HandleGameExistence(w http.ResponseWriter, r *http.Request) error {
	gameID := r.URL.Query().Get("gameId")

	ctx := r.Context()
	exists := server.Services.IsGameAccessible(ctx, gameID)

	respMsg := GameExists
	if !exists {
		respMsg = GameNotExists
	}
	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: respMsg})
	return nil
}

type GetPlayerArgs struct {
	UserID      int
	WithReplays bool
}

func (server *Server) getPlayerQuery(q url.Values) (GetPlayerArgs, error) {
	userID, err := strconv.Atoi(q.Get("id"))
	if err != nil {
		return GetPlayerArgs{}, OneRespError("id", ErrHttpInvalidID)
	}

	withReplaysStr := q.Get("withReplays")
	withReplays := strings.ToLower(withReplaysStr) == "true"

	return GetPlayerArgs{UserID: userID, WithReplays: withReplays}, nil
}

type GetPlayersResp struct {
	User       svc.UserDTO         `json:"user"`
	Stats      svc.UserStatsDTO    `json:"stats"`
	ReplayList []svc.FullReplayDto `json:"replayList"`
}

func (server *Server) HandleGetPlayer(w http.ResponseWriter, r *http.Request) error {
	query, err := server.getPlayerQuery(r.URL.Query())
	if err != nil {
		return err
	}
	loadReplays := query.WithReplays

	var user svc.UserDTO
	var stats svc.UserStatsDTO
	var replayList []svc.FullReplayDto
	var lbRanks map[string]svc.LbRank

	ctx := r.Context()
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		user, err = server.Services.GetUserByID(egCtx, int64(query.UserID))
		return
	})
	eg.Go(func() (err error) {
		stats, err = server.Services.GetUserStats(egCtx, int64(query.UserID))
		return
	})
	eg.Go(func() (err error) {
		lbRanks, err = server.Services.GetUserLeaderboardRanks(egCtx, int64(query.UserID), svc.GameModeMembers)
		return
	})
	if loadReplays {
		eg.Go(func() (err error) {
			replayList, err = server.Services.GetUserReplays(egCtx, int64(query.UserID), -1, perPage)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		switch {
		case errors.Is(err, svc.ErrUserNotFound):
			return ErrHttpNotFoundUser
		default:
			return err
		}
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		lbRank, ok := lbRanks[modeStats.Mode.String()]
		if !ok {
			return fmt.Errorf("missing leaderboard rank for mode %s", modeStats.Mode)
		}
		modeStats.Rank = lbRank.Rank
	}

	playersResp := GetPlayersResp{User: user, Stats: stats, ReplayList: replayList}

	slog.InfoContext(ctx, "retrieved user with replays", "fullUser", playersResp)
	writeJSON(w, http.StatusOK, playersResp)
	return nil
}

type SearchPlayersResp struct {
	UserList []svc.LbdUserDTO `json:"userList,omitempty"`
}

func (server *Server) HandleSearchPlayers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		return OneRespError("page", ErrHttpInvalidPage)
	}
	name := q.Get("username")
	hasUser := name != ""

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []svc.LbdUserDTO
	if hasUser {
		users, err := server.Services.GetFuzzySearchLeaderboard(ctx, name, int32(page), perPage)
		if errors.Is(err, svc.ErrSearchLimit) {
			return ErrHttpSearchLimit
		} else if err != nil {
			return fmt.Errorf("search users by name=%s: %w", name, err)
		}
		userList = users
	}

	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: userList})
	return nil
}

type GetReplayQuery struct {
	ReplayID int64
	GameID   string
}

func (server *Server) getReplayQuery(q url.Values) (GetReplayQuery, error) {
	var respErr ResponseError

	kindStr := q.Get("idKind")
	if kindStr == "" {
		kindStr = "BY_REPLAY_ID"
	}

	var replayID int64
	var gameID string

	switch kindStr {
	case "BY_REPLAY_ID":
		intID, err := strconv.Atoi(q.Get("id"))
		if err != nil {
			respErr.Put("id", ErrHttpInvalidID)
		}
		replayID = int64(intID)
	case "BY_GAME_ID":
		gameID = q.Get("id")
	}

	return GetReplayQuery{ReplayID: replayID, GameID: gameID}, respErr.Interface()
}

type GetReplayResp struct {
	Replay svc.FullReplayDto `json:"replay"`
}

func (server *Server) HandleGetReplay(w http.ResponseWriter, r *http.Request) error {
	query, err := server.getReplayQuery(r.URL.Query())
	if err != nil {
		return err
	}

	var replay svc.FullReplayDto

	ctx := r.Context()
	if query.ReplayID != 0 {
		replay, err = server.Services.GetReplay(ctx, query.ReplayID)
	} else if query.GameID != "" {
		replay, err = server.Services.GetReplayByGameID(ctx, query.GameID)
	}
	if err != nil {
		if errors.Is(err, svc.ErrNoReplay) {
			return ErrHttpNotFoundReplay
		}
		return fmt.Errorf("get replay by id %s: %w", query.GameID, err)
	}

	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

type GetReplaysArg struct {
	UserID  int
	AfterID int
}

func (server *Server) getReplaysQuery(q url.Values) (GetReplaysArg, error) {
	var respErr ResponseError
	userID, err := strconv.Atoi(q.Get("userId"))
	if err != nil {
		respErr.Put("userId", ErrHttpInvalidID)
	}
	afterID, err := strconv.Atoi(q.Get("afterId"))
	if err != nil {
		respErr.Put("afterId", ErrHttpInvalidID)
	}
	return GetReplaysArg{
		UserID:  userID,
		AfterID: afterID,
	}, respErr.Interface()
}

type GetUserReplaysResp struct {
	ReplayList []svc.FullReplayDto `json:"replayList"`
}

func (server *Server) HandleGetUserReplays(w http.ResponseWriter, r *http.Request) error {
	query, err := server.getReplaysQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	replays, err := server.Services.GetUserReplays(ctx, int64(query.UserID), int64(query.AfterID), perPage)
	if err != nil {
		return fmt.Errorf("get user %d replays: %w", query.UserID, err)
	}

	writeJSON(w, http.StatusOK, GetUserReplaysResp{ReplayList: replays})
	return nil
}

type GetChallengesResp struct {
	ChallengeList []svc.ChallengeDTO `json:"challengeList"`
}

func (server *Server) HandleGetChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	participants := r.URL.Query().Get("participants")

	player, _, err := server.GetSessionPlayer(ctx, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	var challengeList []svc.ChallengeDTO
	switch participants {
	case "sent":
		byChallenger, err := server.Services.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1})
		if err != nil {
			return fmt.Errorf("get challenges by challenger: %w", err)
		}
		challengeList = byChallenger
	case "received":
		byChallengee, err := server.Services.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID})
		if err != nil {
			return fmt.Errorf("get challenges by challengee: %w", err)
		}
		challengeList = byChallengee
	}

	slog.InfoContext(ctx, "retrieved challenges", "challengeList", challengeList)
	writeJSON(w, http.StatusOK, GetChallengesResp{ChallengeList: challengeList})
	return nil
}

type ChessMeta struct {
	ID          string          `json:"id"`
	WhitePlayer svc.PlayerState `json:"whitePlayer"`
	BlackPlayer svc.PlayerState `json:"blackPlayer"`
	FirstColor  string          `json:"firstColor"`
	Mode        string          `json:"mode"`
	Touch       time.Time       `json:"touch"`
}

func mapChessMetas(svcMetas []svc.ChessMeta) []ChessMeta {
	metas := make([]ChessMeta, 0)
	for _, svcMeta := range svcMetas {
		metas = append(metas, ChessMeta{
			ID:          svcMeta.ID,
			WhitePlayer: svcMeta.WhitePlayer,
			BlackPlayer: svcMeta.BlackPlayer,
			FirstColor:  svcMeta.FirstColor.String(),
			Mode:        svcMeta.Mode.String(),
			Touch:       svcMeta.Touch,
		})
	}
	return metas
}

type ChessMetasArg struct {
	Page  int
	Count int
}

func (server *Server) getChessMetasQuery(q url.Values) (ChessMetasArg, error) {
	var respErr ResponseError
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		respErr.Put("page", ErrHttpInvalidPage)
	}
	count, err := intQueryDefault(q, "count", perPage)
	if err != nil {
		respErr.Put("count", ErrHttpInvalidCount)
	}
	return ChessMetasArg{
		Page:  page,
		Count: count,
	}, respErr.Interface()
}

type ChessMetasResp struct {
	ChessList     []ChessMeta `json:"chessList"`
	SelfChessList []ChessMeta `json:"selfChessList"`
}

func (server *Server) HandleGetChessMetas(w http.ResponseWriter, r *http.Request) error {
	query, err := server.getChessMetasQuery(r.URL.Query())
	if err != nil {
		return err
	}

	hasSession := true

	ctx := r.Context()
	player, _, err := server.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		hasSession = false
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	allChessMetas, err := server.Services.GetAllChessMetas(ctx, query.Page, query.Count)
	if err != nil {
		return fmt.Errorf("get chess metas page %d: %w", query.Page, err)
	}

	var selfChessMetas []svc.ChessMeta
	if hasSession {
		chessMetas, err := server.Services.GetUserChessMetas(ctx, player.ID)
		if err != nil {
			return fmt.Errorf("get user %d chess metas: %w", player.ID, err)
		}
		selfChessMetas = chessMetas
	}

	writeJSON(w, http.StatusOK, ChessMetasResp{
		ChessList:     mapChessMetas(allChessMetas),
		SelfChessList: mapChessMetas(selfChessMetas),
	})
	return nil
}

type EloHistoriesArg struct {
	UserID int
	Months uint
}

var timeframeMap = map[string]uint{
	"1m":  1,
	"3m":  3,
	"6m":  6,
	"1y":  12,
	"all": 0,
}

func (server *Server) getEloHistoriesQuery(q url.Values) (EloHistoriesArg, error) {
	var respErr ResponseError
	userID, err := strconv.Atoi(q.Get("userId"))
	if err != nil {
		respErr.Put("userID", ErrHttpInvalidID)
	}
	months, ok := timeframeMap[queryDefault(q, "timeframe", "all")]
	if !ok {
		respErr.Put("timeframe", ErrHttpInvalidTimeframe)
	}
	return EloHistoriesArg{
		UserID: userID,
		Months: months,
	}, respErr.Interface()
}

type EloHistoriesResp struct {
	Buckets svc.EloHistoryBuckets `json:"buckets"`
}

var GetEloHistoriesCacheControl = fmt.Sprintf("public, max-age=%f", svc.ShortBucketDuration.Seconds())

func (server *Server) HandleGetEloHistories(w http.ResponseWriter, r *http.Request) error {
	query, err := server.getEloHistoriesQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	params := svc.EloHistoriesParams{UserID: int64(query.UserID), Months: query.Months, TimeUntil: server.EntropySource.GetNow()}
	eloBuckets, _, err := server.Services.RetrieveEloHistoryBuckets(ctx, params)
	if err != nil {
		return fmt.Errorf("retrieve elo histories buckets with params %v: %w", params, err)
	}
	writeJSON(w, http.StatusOK, EloHistoriesResp{Buckets: eloBuckets})

	//w.Header().Set("Cache-Control", GetEloHistoriesCacheControl)
	return nil
}

func (server *Server) HandleCreateTournament(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (server *Server) HandleGetTournament(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (server *Server) HandleGetTournaments(w http.ResponseWriter, r *http.Request) error {
	return nil
}
