package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type RestHandler = func(state *ServerState, w http.ResponseWriter, r *http.Request) error

func MakeRestHandler(state *ServerState, h RestHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.InfoContext(ctx, "request received", "method", r.Method, "url", r.URL, "headers", r.Header)

		if err := h(state, w, r); err != nil {
			slog.ErrorContext(ctx, "failed to request failed", "err", err, "method", r.Method, "url", r.URL)

			status, errs := HttpStatusFromErrs(err)
			writeJSON(w, status, ServiceView{Errors: errs, Status: status})
		}
	})
}

func MakeJsonHandler(v any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})
}

func isPasswordValid(password string) bool {
	return len(password) > 10
}

func isUsernameValid(username string) bool {
	return len(username) >= 5 && len(username) <= 20
}

func isBioValid(bio string) bool {
	return len(bio) <= 500
}

const PerPage = 25

type RegisterBody struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func HandleRegister(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body RegisterBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	var err error
	if !isUsernameValid(body.Username) {
		err = PutErrorMap(err, "username", ErrHttpInvalidUsername)
	}
	if !isPasswordValid(body.Password) {
		err = PutErrorMap(err, "password", ErrHttpInvalidPassword)
	}
	if body.Password != body.ConfirmPassword {
		err = PutErrorMap(err, "confirmPassword", ErrHttpConfirmPassword)
	}
	if err != nil {
		return err
	}

	ctx := r.Context()
	user, err := svc.InsertUser(ctx, state.Pdb.Query, svc.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  svc.DefaultCountry,
		JoinedOn: state.GetNow(),
	})
	if errors.Is(err, svc.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	} else if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	t, err := SetSessionPlayer(ctx, state.Rdb, w, svc.MakePlayer(user.ID, user.Username, user.Country))
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

func handleLoginSession(ctx context.Context, rdb *db.Redis, w http.ResponseWriter, user svc.VerifiedUser) error {
	t, err := SetSessionPlayer(ctx, rdb, w, svc.MakePlayer(user.ID, user.Username, user.Country))
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

func HandleLogin(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body LoginBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := svc.VerifyUserTx(ctx, state.Pdb, body.Username, body.Password)
	switch {
	case err == svc.ErrUserNotFound:
		return ErrHttpInvalidLogin
	case err == svc.ErrTooManyLoginAttempts:
		return ErrHttpTooManyLoginAttempts
	case err != nil:
		return fmt.Errorf("verify user: %w", err)
	}
	return handleLoginSession(ctx, state.Rdb, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func HandleGoogleLogin(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body GoogleLoginBody
	if err := readJSON(r, &body); err != nil {
		return err
	}
	ctx := r.Context()

	payload, err := state.ValidateIDToken(ctx, body.Token)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	user, err := svc.SelectOrInsertGoogleUser(ctx, state.Pdb.Query, payload.AccountID, svc.GoogleUserInst{
		Username: payload.Username,
		Country:  svc.DefaultCountry,
	})
	if err != nil {
		return fmt.Errorf("upsert verified google user: %w", err)
	}
	return handleLoginSession(ctx, state.Rdb, w, user)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func HandleUpdatePassword(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body UpdatePasswordBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	var err error
	if !isPasswordValid(body.NewPassword) {
		err = PutErrorMap(err, "newPassword", ErrHttpInvalidPassword)
	}
	if body.NewPassword != body.ConfirmNewPassword {
		err = PutErrorMap(err, "confirmNewPassword", ErrHttpConfirmPassword)
	}
	if err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := svc.VerifyUserTx(ctx, state.Pdb, player.Name, body.Password)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return fmt.Errorf("verify user: %w", err)
	}

	if err := svc.UpdateUserPassword(ctx, state.Pdb.Query, user.ID, body.NewPassword); err != nil {
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

func HandleUpdateUser(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body UpdateUserBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	var err error
	if !isUsernameValid(body.NewUsername) {
		err = PutErrorMap(err, "newUsername", ErrHttpInvalidUsername)
	}
	if !isBioValid(body.NewBio) {
		err = PutErrorMap(err, "newBio", ErrHttpInvalidBio)
	}
	if _, ok := state.ValidCountries[body.NewCountry]; !ok {
		return PutErrorMap(nil, "newCountry", ErrHttpInvalidCountry)
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	user, err := svc.UpdateUser(ctx, state.Pdb.Query, player.ID, svc.UpdtUserParams{
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

func HandleCreateTempSession(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return ErrHttpSessionExpired
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	sessionID := MakeSessionID()
	if err := svc.SetSession(ctx, state.Rdb, sessionID, player, TempSessionMaxAge); err != nil {
		return fmt.Errorf("set session: %w", err)
	}

	slog.InfoContext(ctx, "created temporary user session", "user", player)
	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: sessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func HandleRefreshSession(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	player, sessionID, err := GetSessionPlayer(ctx, state.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if err := svc.UpdateSessionEx(ctx, state.Rdb, sessionID, SessionMaxAge); err != nil {
		return fmt.Errorf("update session: %w", err)
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

func HandleLogout(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return fmt.Errorf("get cookie '%s': %w", CookieKey, err)
	}
	sessionID := cookie.Value

	ctx := r.Context()
	if err := svc.DeleteSession(ctx, state.Rdb, sessionID); err != nil {
		return fmt.Errorf("logging out session '%s': %w", sessionID, err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type CreateGameBody struct {
	FirstColor svc.ColorSelect `json:"firstColor"`
	Mode       svc.GameMode    `json:"mode"`
	InitialFEN string          `json:"initialFen"`
}

type CreateGameResp struct {
	GameID string `json:"gameId"`
}

func HandleCreateGame(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body CreateGameBody
	if err := readJSON(r, &body); err != nil {
		return err
	}
	ctx := r.Context()

	var initialBoard *chess.Board
	if body.InitialFEN != "" {
		board, err := chess.ParseFen(body.InitialFEN)
		if err != nil {
			slog.WarnContext(ctx, "invalid initial FEN", "initialFEN", body.InitialFEN, "err", err)
			return PutErrorMap(nil, "initialFEN", ErrInvalidFen)
		}
		initialBoard = &board
	}

	gameID, err := svc.CreateGame(ctx, state.Rdb, body.FirstColor, body.Mode, initialBoard)
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

type UpdateChallengeResp struct {
	GameID string `json:"challengeID"`
}

func HandleUpdateChallenge(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body UpdateChallengeBody
	if err := readJSON(r, &body); err != nil {
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
		return PutErrorMap(nil, "action", ErrHttpInvalidAction)
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if player.ID != targetID {
		return ErrHttpUpdateChallenge
	}
	dr, err := svc.DeleteChallenge(ctx, state.Pdb.Query, svc.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if errors.Is(err, svc.ErrChallengeNotFound) {
		return ErrHttpNotFoundChallenge
	} else if err != nil {
		return err
	}
	var gameID string
	if body.Action == "ACCEPT" {
		gameID, err = svc.CreateGame(ctx, state.Rdb, dr.FirstColor, dr.Mode, nil)
		if err != nil {
			return fmt.Errorf("create game: %w", err)
		}
	}

	writeJSON(w, http.StatusOK, UpdateChallengeResp{GameID: gameID})
	return nil
}

type CreateChallengeBody struct {
	ChallengeeID int64           `json:"challengeeID"`
	StartColor   svc.ColorSelect `json:"startColor"`
	Mode         svc.GameMode    `json:"mode"`
}

func HandleCreateChallenge(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body CreateChallengeBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	ret, err := svc.InsertChallengeRet(ctx, state.Pdb.Query, svc.ChallengeInst{
		ChallengerID: player.ID,
		ChallengeeID: body.ChallengeeID,
		Mode:         body.Mode,
		StartColor:   body.StartColor,
		MadeOn:       state.GetNow(),
	})
	switch {
	case err == svc.ErrDuplicateChallenge:
		return ErrHttpDuplicateChallenge
	case err == svc.ErrParticipantConflict:
		return ErrHttpInvalidParticipants
	case err == svc.ErrSelfChallenge:
		return ErrHttpSelfChallenge
	case err != nil:
		return fmt.Errorf("insert challenge: %w", err)
	}
	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})

	ctx = context.WithoutCancel(ctx)
	if err := svc.BroadcastChallenge(ctx, state.Rdb, ret); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast challenge", "challenge", ret, "err", err)
	}
	if err := svc.DeleteExpiredChallenges(ctx, state.Pdb.Query, player.ID, svc.ExpireChallengeThreshold); err != nil {
		slog.ErrorContext(ctx, "failed to delete expired challenges", "challenge", ret, "err", err)
	}
	return nil
}

func HandleGetSelf(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	}
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := svc.GetUserByID(ctx, state.Pdb.Query, player.ID)
	if err != nil {
		return fmt.Errorf("get user by id %+v: %w", player, err)
	}
	slog.InfoContext(ctx, "retrieved user", "user", user)

	writeJSON(w, http.StatusOK, user)
	return nil
}

type LeaderboardResp struct {
	TotalPages int                 `json:"totalPages"`
	UserList   []svc.LbdUserEntity `json:"userList,omitempty"`
}

func HandleGetLeaderboard(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	var queryErr error
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		queryErr = PutErrorMap(queryErr, "page", ErrHttpInvalidPage)
	}
	mode, err := svc.ModeFromString(q.Get("mode"))
	if err != nil {
		queryErr = PutErrorMap(queryErr, "mode", ErrHttpInvalidMode)
	}
	if queryErr != nil {
		return queryErr
	}

	lbd, err := svc.GetLeaderboardPage(ctx, state.Rdb, mode, int64(page), PerPage)
	if err != nil {
		return fmt.Errorf("get leaderboard page %d: %w", page, err)
	}
	users, err := svc.GetLeaderboardUsers(ctx, state.Pdb.Query, mode, lbd.RankedUsers)
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

type FullUserResp struct {
	User       svc.UserEntity      `json:"user"`
	Stats      svc.UserStatsEntity `json:"stats"`
	ReplayList []svc.ReplayEntity  `json:"replayList"`
}

func HandleGetPlayer(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	id, err := intQuery(q, "id")
	if err != nil {
		return PutErrorMap(nil, "id", ErrHttpInvalidID)
	}
	withReplaysStr := q.Get("withReplays")
	withReplays := strings.ToLower(withReplaysStr) == "true"

	eg, egCtx := errgroup.WithContext(ctx)
	var resp FullUserResp
	var lbRanks map[svc.GameMode]svc.LbRank

	eg.Go(func() (err error) {
		resp.User, err = svc.GetUserByID(egCtx, state.Pdb.Query, int64(id))
		return err
	})
	eg.Go(func() (err error) {
		resp.Stats, err = svc.GetUserElos(egCtx, state.Pdb.Query, int64(id))
		return err
	})
	eg.Go(func() (err error) {
		lbRanks, err = svc.GetLeaderboardRanks(egCtx, state.Rdb, int64(id), svc.GameModes)
		return err
	})
	if withReplays {
		eg.Go(func() (err error) {
			resp.ReplayList, err = svc.GetUserReplays(egCtx, state.Pdb.Query, int64(id), -1, PerPage)
			return err
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
	UserList []svc.UserEntity `json:"userList,omitempty"`
}

func HandleSearchPlayers(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		return PutErrorMap(nil, "page", ErrHttpInvalidPage)
	}
	name := q.Get("username")
	hasUser := name != ""

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []svc.UserEntity
	if hasUser {
		users, err := svc.SearchUsersByName(ctx, state.Pdb.Query, name, int32(page), PerPage)
		if errors.Is(err, svc.ErrSearchLimit) {
			return ErrHttpSearchLimit
		} else if err != nil {
			return fmt.Errorf("search users by name '%s': %w", name, err)
		}
		userList = users
	}

	if userList == nil {
		userList = []svc.UserEntity{}
	}
	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: userList})
	return nil
}

type GetReplayResp struct {
	Replay svc.ReplayEntity `json:"replay"`
}

func HandleGetReplay(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return PutErrorMap(nil, "page", ErrHttpInvalidID)
	}

	replay, err := svc.GetReplay(ctx, state.Pdb.Query, int64(id))
	if err != nil {
		return fmt.Errorf("get replay %d: %w", id, err)
	}
	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

func HandleGetReplayMoveList(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	id, err := intQuery(q, "userId")
	if err != nil {
		return PutErrorMap(nil, "page", ErrHttpInvalidID)
	}

	pbMoveHist, err := svc.GetReplayMoveHistory(ctx, state.Pdb.Query, int64(id))
	if err != nil {
		return fmt.Errorf("get replay moveHistory: %w", err)
	}
	b, err := proto.Marshal(chess.SerializeMoveReplay(pbMoveHist))
	if err != nil {
		return fmt.Errorf("marshal replay %d move list : %w", id, err)
	}
	writeBytes(w, http.StatusOK, b)
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	return nil
}

type GetUserReplaysResp struct {
	ReplayList []svc.ReplayEntity `json:"replayList"`
}

func HandleGetUserReplays(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	var queryErr error
	userID, err := intQuery(q, "userId")
	if err != nil {
		queryErr = PutErrorMap(queryErr, "userId", ErrHttpInvalidID)
	}
	afterID, err := intQuery(q, "afterId")
	if err != nil {
		queryErr = PutErrorMap(queryErr, "afterId", ErrHttpInvalidID)
	}
	if queryErr != nil {
		return queryErr
	}

	replays, err := svc.GetUserReplays(ctx, state.Pdb.Query, int64(userID), int64(afterID), PerPage)
	if err != nil {
		return fmt.Errorf("get user %d replays: %w", userID, err)
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

func HandleGetChallenges(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	participants := r.URL.Query().Get("participants")

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	since := svc.MakeGetChallengesSince(state.GetNow())
	var challengeList []svc.ChallengeEntity
	switch participants {
	case "sent":
		challengeList, err = svc.GetChallengesByParticipant(ctx, state.Pdb.Query, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1}, since)
	case "received":
		challengeList, err = svc.GetChallengesByParticipant(ctx, state.Pdb.Query, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID}, since)
	}
	if err != nil {
		return fmt.Errorf("get challenges by participant: %w", err)
	}

	if challengeList == nil {
		challengeList = []svc.ChallengeEntity{}
	}
	slog.InfoContext(ctx, "retrieved challenges", "challengeList", challengeList)
	writeJSON(w, http.StatusOK, GetChallengesResp{ChallengeList: challengeList})
	return nil
}

type ChessRoomListResp struct {
	ChessList     []svc.ChessMeta `json:"chessList"`
	SelfChessList []svc.ChessMeta `json:"selfChessList"`
}

func HandleGetChessRoomList(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	var queryErr error
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		queryErr = PutErrorMap(queryErr, "page", ErrHttpInvalidPage)
	}
	count, err := intQueryDefault(q, "count", PerPage)
	if err != nil {
		queryErr = PutErrorMap(queryErr, "count", ErrHttpInvalidCount)
	}
	if queryErr != nil {
		return queryErr
	}

	var hasSession bool
	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		if !errors.Is(err, svc.ErrSessionNotFound) {
			return fmt.Errorf("get session player: %w", err)
		}
	} else {
		hasSession = true
	}

	chessList, err := svc.GetAllChessMetas(ctx, state.Rdb, page, count)
	if err != nil {
		return fmt.Errorf("get page %d chess meta views: %w", page, err)
	}
	var selfChessList []svc.ChessMeta
	if hasSession {
		cl, err := svc.GetUserChessMetas(ctx, state.Rdb, player.ID)
		if err != nil {
			return fmt.Errorf("get user %d chess meta views: %w", player.ID, err)
		}
		selfChessList = cl
	}

	if chessList == nil {
		chessList = []svc.ChessMeta{}
	}
	if selfChessList == nil {
		selfChessList = []svc.ChessMeta{}
	}
	writeJSON(w, http.StatusOK, ChessRoomListResp{ChessList: chessList, SelfChessList: selfChessList})
	return nil
}

type EloHistoriesResp struct {
	Buckets svc.EloHistoryBuckets `json:"buckets"`
}

var timeframeMap = map[string]uint{
	"1m":  1,
	"3m":  3,
	"6m":  6,
	"1y":  12,
	"all": 0,
}

var GetEloHistoriesCacheControl = fmt.Sprintf("public, max-age=%f", svc.ShortBucketDuration.Seconds())

func HandleGetEloHistories(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	var queryErr error
	userID, err := intQuery(q, "userId")
	if err != nil {
		queryErr = PutErrorMap(queryErr, "userID", ErrHttpInvalidPage)
	}
	timeframe := q.Get("timeframe")
	if timeframe == "" {
		timeframe = "all"
	}
	months, ok := timeframeMap[timeframe]
	if !ok {
		queryErr = PutErrorMap(queryErr, "timeframe", ErrHttpInvalidTimeframe)
	}
	if queryErr != nil {
		return queryErr
	}

	ctx := r.Context()
	params := svc.EloHistoriesParams{UserID: int64(userID), Months: months, TimeUntil: state.GetNow()}
	eloBuckets, _, err := svc.RetrieveEloHistoryBuckets(ctx, &state.Databases, params)
	if err != nil {
		return fmt.Errorf("retrieve elo histories buckets with params %v: %w", params, err)
	}
	writeJSON(w, http.StatusOK, EloHistoriesResp{Buckets: eloBuckets})

	//w.Header().Set("Cache-Control", GetEloHistoriesCacheControl)
	return nil
}
