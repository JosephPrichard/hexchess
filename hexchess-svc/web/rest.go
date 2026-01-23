package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"hexchess-svc/pkg/errmap"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	var errm error
	if !isPasswordValid(body.Password) {
		errm = errmap.Put(errm, "password", ErrHttpInvalidPassword)
	}
	if body.Password != body.ConfirmPassword {
		errm = errmap.Put(errm, "confirmPassword", ErrHttpConfirmPassword)
	}
	if !isUsernameValid(body.Username) {
		errm = errmap.Put(errm, "username", ErrHttpInvalidUsername)
	}
	return errm
}

func (app *App) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	var body RegisterBody
	if err := parseJSON(r, &body, validateRegisterBody); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := app.State.InsertUser(ctx, svc.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  svc.DefaultCountry,
		JoinedOn: app.GetNow(),
	})
	if errors.Is(err, svc.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	} else if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	t, err := SetSessionPlayer(ctx, app.State, w, svc.MakePlayer(user.ID, user.Username, user.Country))
	if err != nil {
		return fmt.Errorf("set session player: %w", err)
	}
	slog.InfoContext(ctx, "registered user", "user", user)

	writeJSON(w, http.StatusOK, SessionView{
		ID:       user.ID,
		Username: user.Username,
		Country:  user.Country,
		TTLSecs:  t,
	})
	return nil
}

func (app *App) handleLoginSession(ctx context.Context, w http.ResponseWriter, user svc.VerifiedUser) error {
	t, err := SetSessionPlayer(ctx, app.State, w, svc.MakePlayer(user.ID, user.Username, user.Country))
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

func (app *App) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	var body LoginBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}

	ctx := r.Context()

	user, err := app.State.VerifyUserTx(ctx, body.Username, body.Password)
	switch {
	case errors.Is(err, svc.ErrUserNotFound):
		return ErrHttpInvalidLogin
	case errors.Is(err, svc.ErrTooManyLoginAttempts):
		return ErrHttpTooManyLoginAttempts
	case err != nil:
		return fmt.Errorf("verify user: %w", err)
	}

	return app.handleLoginSession(ctx, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func (app *App) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) error {
	var body GoogleLoginBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}
	ctx := r.Context()

	payload, err := app.RemoteAPIs.ValidateIDToken(ctx, body.Token)
	if err != nil {
		return fmt.Errorf("validate google login id token: %w", err)
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	user, err := app.State.SelectOrInsertGoogleUser(ctx, payload.AccountID, svc.GoogleUserInst{
		Username: payload.Username,
		Country:  svc.DefaultCountry,
	})
	if err != nil {
		return fmt.Errorf("upsert verified google user: %w", err)
	}
	return app.handleLoginSession(ctx, w, user)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func validateUpdatePasswordBody(body UpdatePasswordBody) error {
	var errm error
	if !isPasswordValid(body.NewPassword) {
		errm = errmap.Put(errm, "newPassword", ErrHttpInvalidPassword)
	}
	if body.NewPassword != body.ConfirmNewPassword {
		errm = errmap.Put(errm, "confirmNewPassword", ErrHttpConfirmPassword)
	}
	return errm
}

func (app *App) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	var body UpdatePasswordBody
	if err := parseJSON(r, &body, validateUpdatePasswordBody); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := app.State.VerifyUserTx(ctx, player.Name, body.Password)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return fmt.Errorf("verify user: %w", err)
	}

	if err := app.State.UpdateUserPassword(ctx, user.ID, body.NewPassword); err != nil {
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

func (app *App) validateUpdateUserBody(body UpdateUserBody) error {
	var errm error
	if body.NewUsername != "" {
		if !isUsernameValid(body.NewUsername) {
			errm = errmap.Put(errm, "newUsername", ErrHttpInvalidUsername)
		}
	}
	if body.NewBio != "" {
		if !isBioValid(body.NewBio) {
			errm = errmap.Put(errm, "newBio", ErrHttpInvalidBio)
		}
	}
	if body.NewCountry != "" {
		if _, ok := app.ValidCountries[body.NewCountry]; !ok {
			errm = errmap.Put(errm, "newCountry", ErrHttpInvalidCountry)
		}
	}
	return errm
}

func (app *App) HandleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	var body UpdateUserBody
	if err := parseJSON(r, &body, app.validateUpdateUserBody); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := app.State.UpdateUser(ctx, player.ID, svc.UpdtUserParams{
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

func (app *App) HandleCreateTempSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	alreadyHasSession := true
	var tempSessionID string

	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		alreadyHasSession = false
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	if alreadyHasSession {
		tempSessionID = MakeSessionID()
		if err := app.State.SetSessions(ctx, svc.SessInst{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge}); err != nil {
			return fmt.Errorf("set session: %w", err)
		}
		slog.InfoContext(ctx, "created temporary user session", "user", player, "tempSessionID", tempSessionID)
	} else {
		player = svc.MakeGuest()
		tempSessionID = MakeSessionID()
		guestSessionID := MakeSessionID()

		if err := app.State.SetSessions(ctx,
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

func (app *App) HandleRefreshSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, sessionID, err := GetSessionPlayer(ctx, app.State, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if err := app.State.UpdateSessionEx(ctx, sessionID, SessionMaxAge); err != nil {
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

func (app *App) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return fmt.Errorf("get cookie %s: %w", CookieKey, err)
	}
	sessionID := cookie.Value

	if err := app.State.DeleteSession(r.Context(), sessionID); err != nil {
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
	FirstColor   svc.Color
	Mode         svc.GameMode
	InitialBoard *chess.Board
}

func transformCreateGame(body CreateGameBody) (CreateGameArgs, error) {
	var initialBoard *chess.Board
	var errm error

	if body.InitialFEN != "" {
		board, err := chess.ParseFen(body.InitialFEN)
		if err != nil {
			errm = errmap.Put(errm, "initialFen", ErrHttpInvalidFen)
		} else {
			initialBoard = &board
		}
	}
	color, ok := svc.ColorMap[body.FirstColor]
	if !ok {
		errm = errmap.Put(errm, "firstColor", ErrHttpInvalidColor)
	}
	mode, ok := svc.GameModeMap[body.Mode]
	if !ok {
		errm = errmap.Put(errm, "mode", ErrHttpInvalidMode)
	}
	if errm != nil {
		return CreateGameArgs{}, errm
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

func (app *App) HandleCreateGame(w http.ResponseWriter, r *http.Request) error {
	body, err := transformJSON(r, transformCreateGame)
	if err != nil {
		return err
	}

	ctx := r.Context()
	gameID, err := app.State.CreateGame(ctx, body.FirstColor, body.Mode, body.InitialBoard)
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
		return UpdateChallengeArgs{}, errmap.Put(nil, "action", ErrHttpInvalidAction)
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

func (app *App) HandleUpdateChallenge(w http.ResponseWriter, r *http.Request) error {
	body, err := transformJSON(r, transformUpdateChallenge)
	if err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if player.ID != body.TargetID {
		return ErrHttpUpdateChallenge
	}

	dr, err := app.State.DeleteChallenge(ctx, svc.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if errors.Is(err, svc.ErrChallengeNotFound) {
		return ErrHttpNotFoundChallenge
	} else if err != nil {
		return err
	}

	var gameID string
	if body.Action == Accept {
		gameID, err = app.State.CreateGame(ctx, dr.FirstColor, dr.Mode, nil)
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
	StartColor   svc.Color
	Mode         svc.GameMode
}

func transformCreateChallenge(body CreateChallengeBody) (CreateChallengeArgs, error) {
	var errm error
	color, ok := svc.ColorMap[body.StartColor]
	if !ok {
		errm = errmap.Put(errm, "startColor", ErrHttpInvalidColor)
	}
	mode, ok := svc.GameModeMap[body.Mode]
	if !ok {
		errm = errmap.Put(errm, "mode", ErrHttpInvalidMode)
	}
	return CreateChallengeArgs{
		ChallengeeID: body.ChallengeeID,
		StartColor:   color,
		Mode:         mode,
	}, errm
}

func (app *App) HandleCreateChallenge(w http.ResponseWriter, r *http.Request) error {
	body, err := transformJSON(r, transformCreateChallenge)
	if err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	ret, err := app.State.InsertChallengeRet(ctx, svc.ChallengeInst{
		ChallengerID: player.ID,
		ChallengeeID: body.ChallengeeID,
		Mode:         body.Mode,
		StartColor:   body.StartColor,
		MadeOn:       app.GetNow(),
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
	if err := app.State.BroadcastChallenge(ctx, ret); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast challenge", "challenge", ret, "err", err)
	}
	if err := app.State.DeleteExpiredChallenges(ctx, player.ID); err != nil {
		slog.ErrorContext(ctx, "failed to delete expired challenges", "challenge", ret, "err", err)
	}

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (app *App) HandleGetSelf(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	}
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := app.State.GetUserByID(ctx, player.ID)
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

func (app *App) getLeaderboardQuery(q url.Values) (LeaderboardArg, error) {
	var errm error
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		errm = errmap.Put(errm, "page", ErrHttpInvalidPage)
	}
	mode, ok := svc.GameModeMap[q.Get("mode")]
	if !ok {
		errm = errmap.Put(errm, "mode", ErrHttpInvalidMode)
	}
	return LeaderboardArg{
		Page: page,
		Mode: mode,
	}, errm
}

type LeaderboardResp struct {
	TotalPages int                 `json:"totalPages"`
	UserList   []svc.LbdUserEntity `json:"userList,omitempty"`
}

func (app *App) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) error {
	query, err := app.getLeaderboardQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	lbd, err := app.State.GetLeaderboardPage(ctx, query.Mode, int64(query.Page), perPage)
	if err != nil {
		return fmt.Errorf("get leaderboard page %d: %w", query.Page, err)
	}
	users, _, err := app.State.GetLeaderboardUsers(ctx, query.Mode, lbd.RankedUsers)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	if users == nil {
		users = []svc.LbdUserEntity{}
	}
	writeJSON(w, http.StatusOK, LeaderboardResp{TotalPages: lbd.PageCount, UserList: users})
	return nil
}

const (
	GameExists    = "GAME_EXISTS"
	GameNotExists = "GAME_NOT_EXISTS"
)

func (app *App) HandleGameExistence(w http.ResponseWriter, r *http.Request) error {
	gameID := r.URL.Query().Get("gameId")

	ctx := r.Context()
	exists := app.State.IsGameAccessible(ctx, gameID)

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

func (app *App) getPlayerQuery(q url.Values) (GetPlayerArgs, error) {
	var errm error
	userID, err := strconv.Atoi(q.Get("id"))
	if err != nil {
		errm = errmap.Put(nil, "id", ErrHttpInvalidID)
	}
	withReplaysStr := q.Get("withReplays")
	withReplays := strings.ToLower(withReplaysStr) == "true"
	return GetPlayerArgs{UserID: userID, WithReplays: withReplays}, errm
}

type GetPlayersResp struct {
	User       svc.UserEntity      `json:"user"`
	Stats      svc.UserStatsEntity `json:"stats"`
	ReplayList []svc.ReplayEntity  `json:"replayList"`
}

func (app *App) HandleGetPlayer(w http.ResponseWriter, r *http.Request) error {
	query, err := app.getPlayerQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	eg, egCtx := errgroup.WithContext(ctx)

	var resp GetPlayersResp
	var lbRanks map[string]svc.LbRank

	eg.Go(func() (err error) {
		resp.User, err = app.State.GetUserByID(egCtx, int64(query.UserID))
		return
	})
	eg.Go(func() (err error) {
		resp.Stats, err = app.State.GetUserStats(egCtx, int64(query.UserID))
		return
	})
	eg.Go(func() (err error) {
		lbRanks, err = app.State.GetLeaderboardRanks(egCtx, int64(query.UserID), svc.GameModeMap)
		return
	})
	if query.WithReplays {
		eg.Go(func() (err error) {
			resp.ReplayList, err = app.State.GetUserReplays(egCtx, int64(query.UserID), -1, perPage)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	for i := range resp.Stats.ModeStats {
		modeStats := &resp.Stats.ModeStats[i]
		lbRank, ok := lbRanks[modeStats.Mode]
		if !ok {
			return fmt.Errorf("missing leaderboard rank for mode %s", modeStats.Mode)
		}
		modeStats.Rank = lbRank.Rank
	}

	if resp.ReplayList == nil {
		resp.ReplayList = []svc.ReplayEntity{}
	}
	slog.InfoContext(ctx, "retrieved user with replays", "fullUser", resp)
	writeJSON(w, http.StatusOK, resp)
	return nil
}

type SearchPlayersResp struct {
	UserList []svc.LbdUserEntity `json:"userList,omitempty"`
}

func (app *App) HandleSearchPlayers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		return errmap.Put(nil, "page", ErrHttpInvalidPage)
	}
	name := q.Get("username")
	hasUser := name != ""

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []svc.LbdUserEntity
	if hasUser {
		userList, err = app.State.GetFuzzySearchLeaderboard(ctx, name, int32(page), perPage)
		if errors.Is(err, svc.ErrSearchLimit) {
			return ErrHttpSearchLimit
		} else if err != nil {
			return fmt.Errorf("search users by name=%s: %w", name, err)
		}
	}

	if userList == nil {
		userList = []svc.LbdUserEntity{}
	}
	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: userList})
	return nil
}

type GetReplayResp struct {
	Replay svc.ReplayEntity `json:"replay"`
}

func (app *App) HandleGetReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return errmap.Put(nil, "id", ErrHttpInvalidID)
	}

	replay, err := app.State.GetReplay(ctx, int64(id))
	if err != nil {
		return fmt.Errorf("get replay %d: %w", id, err)
	}
	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

func (app *App) HandleGetMoveReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	replayID := r.URL.Query().Get("replayId")

	bResp, err := app.State.GetMoveReplay(ctx, replayID)
	if err != nil {
		return err
	}

	writeBytes(w, http.StatusOK, bResp)
	// aggressive cache control because this resource does not change, but the algorithm we are using to transform it might if requirements change.
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	return nil
}

type GetReplaysArg struct {
	UserID  int
	AfterID int
}

func (app *App) getReplaysQuery(q url.Values) (GetReplaysArg, error) {
	var errm error
	userID, err := strconv.Atoi(q.Get("userId"))
	if err != nil {
		errm = errmap.Put(errm, "userId", ErrHttpInvalidID)
	}
	afterID, err := strconv.Atoi(q.Get("afterId"))
	if err != nil {
		errm = errmap.Put(errm, "afterId", ErrHttpInvalidID)
	}
	return GetReplaysArg{
		UserID:  userID,
		AfterID: afterID,
	}, errm
}

type GetUserReplaysResp struct {
	ReplayList []svc.ReplayEntity `json:"replayList"`
}

func (app *App) HandleGetUserReplays(w http.ResponseWriter, r *http.Request) error {
	query, err := app.getReplaysQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	replays, err := app.State.GetUserReplays(ctx, int64(query.UserID), int64(query.AfterID), perPage)
	if err != nil {
		return fmt.Errorf("get user %d replays: %w", query.UserID, err)
	}

	if replays == nil {
		replays = []svc.ReplayEntity{}
	}
	writeJSON(w, http.StatusOK, GetUserReplaysResp{ReplayList: replays})
	return nil
}

type GetChallengesResp struct {
	ChallengeList []svc.ChallengeEntity `json:"challengeList"`
}

func (app *App) HandleGetChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	participants := r.URL.Query().Get("participants")

	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	var challengeList []svc.ChallengeEntity
	switch participants {
	case "sent":
		if challengeList, err = app.State.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1}); err != nil {
			return fmt.Errorf("get challenges by challenger: %w", err)
		}
	case "received":
		if challengeList, err = app.State.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID}); err != nil {
			return fmt.Errorf("get challenges by challengee: %w", err)
		}
	}

	if challengeList == nil {
		challengeList = []svc.ChallengeEntity{}
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

func (app *App) getChessMetasQuery(q url.Values) (ChessMetasArg, error) {
	var errm error
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		errm = errmap.Put(errm, "page", ErrHttpInvalidPage)
	}
	count, err := intQueryDefault(q, "count", perPage)
	if err != nil {
		errm = errmap.Put(errm, "count", ErrHttpInvalidCount)
	}
	return ChessMetasArg{
		Page:  page,
		Count: count,
	}, errm
}

type ChessMetasResp struct {
	ChessList     []ChessMeta `json:"chessList"`
	SelfChessList []ChessMeta `json:"selfChessList"`
}

func (app *App) HandleGetChessMetas(w http.ResponseWriter, r *http.Request) error {
	query, err := app.getChessMetasQuery(r.URL.Query())
	if err != nil {
		return err
	}

	hasSession := true

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		hasSession = false
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	allChessMetas, err := app.State.GetAllChessMetas(ctx, query.Page, query.Count)
	if err != nil {
		return fmt.Errorf("get page %d chess metas: %w", query.Page, err)
	}

	var myChessMetas []svc.ChessMeta
	if hasSession {
		if myChessMetas, err = app.State.GetUserChessMetas(ctx, player.ID); err != nil {
			return fmt.Errorf("get user %d chess metas: %w", player.ID, err)
		}
	}

	writeJSON(w, http.StatusOK, ChessMetasResp{
		ChessList:     mapChessMetas(allChessMetas),
		SelfChessList: mapChessMetas(myChessMetas),
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

func (app *App) getEloHistoriesQuery(q url.Values) (EloHistoriesArg, error) {
	var errm error
	userID, err := strconv.Atoi(q.Get("userId"))
	if err != nil {
		errm = errmap.Put(errm, "userID", ErrHttpInvalidPage)
	}
	months, ok := timeframeMap[queryDefault(q, "timeframe", "all")]
	if !ok {
		errm = errmap.Put(errm, "timeframe", ErrHttpInvalidTimeframe)
	}
	return EloHistoriesArg{
		UserID: userID,
		Months: months,
	}, errm
}

type EloHistoriesResp struct {
	Buckets svc.EloHistoryBuckets `json:"buckets"`
}

var GetEloHistoriesCacheControl = fmt.Sprintf("public, max-age=%f", svc.ShortBucketDuration.Seconds())

func (app *App) HandleGetEloHistories(w http.ResponseWriter, r *http.Request) error {
	query, err := app.getEloHistoriesQuery(r.URL.Query())
	if err != nil {
		return err
	}

	ctx := r.Context()
	params := svc.EloHistoriesParams{UserID: int64(query.UserID), Months: query.Months, TimeUntil: app.GetNow()}
	eloBuckets, _, err := app.State.RetrieveEloHistoryBuckets(ctx, params)
	if err != nil {
		return fmt.Errorf("retrieve elo histories buckets with params %v: %w", params, err)
	}
	writeJSON(w, http.StatusOK, EloHistoriesResp{Buckets: eloBuckets})

	//w.Header().Set("Cache-Control", GetEloHistoriesCacheControl)
	return nil
}

// MaxProfilePicSize 5 MiB
const MaxProfilePicSize = 5 << 20

func (app *App) HandleUploadProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, app.State, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	contentType := r.Header.Get("Content-Type")

	// the maximum memory we are using is the same as the max bytes reader to prevent writing temp files to disk
	r.Body = http.MaxBytesReader(w, r.Body, MaxProfilePicSize)

	if err := r.ParseMultipartForm(MaxProfilePicSize); err != nil {
		return fmt.Errorf("parse multipart form: %w", err)
	}
	defer func() {
		// this isn't necessary if MaxBytesReader = MaxMemory, but it will become necessary if we change that
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
	}()
	file, _, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("get file from form: %w", err)
	}
	defer file.Close()

	// uploading profile picture based off a computed key
	key := svc.MakeProfileNewPicKey(player.ID)
	slog.InfoContext(ctx, "uploading profile pic to s3", "key", key, "player", player)
	start := time.Now()

	putOutput, err := app.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(app.S3ProfileBucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
		// with max cache control. profile pics are immutable, since we issue a new unique key on upload.
		CacheControl: aws.String("public, max-age=31536000"),
	})
	if err != nil {
		return fmt.Errorf("put profile pic %s: to s3 bucket: %s: %w", key, app.S3ProfileBucket, err)
	}

	slog.InfoContext(ctx, "finished uploading profile pic to s3", "key", key, "took", time.Since(start), "player", player, "output", putOutput)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(ctx, "recovered from panic while deleting old profile pics", "error", err)
			}
		}()
		// removes old profile pictures on upload of a new profile pic, since retrieval function will always get the most recent file.
		if err := app.DeleteOldProfilePics(ctx, int(player.ID)); err != nil {
			slog.ErrorContext(ctx, "failed to remove old profile pics", "error", err)
		}
	}()

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: key})
	return nil
}

func (app *App) HandleGetProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := r.URL.Query().Get("userId")

	key, err := app.GetProfilePicKey(ctx, userID)
	if errors.Is(svc.ErrNoProfilePic, err) {
		w.Write(assets.DefaultProfilePic)
		return nil
	} else if err != nil {
		return fmt.Errorf("get profile pic key for user %s: %w", userID, err)
	}

	s3URL := app.MakeS3Url(app.S3ProfileBucket, key)
	slog.InfoContext(ctx, "resolved user ID to S3 profile pic URL", "url", s3URL, "userID", userID)

	// cache control is for what URL is being redirected to, this only changes if the user uploads a new profile pic
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	http.Redirect(w, r, s3URL, http.StatusTemporaryRedirect)
	return nil
}
