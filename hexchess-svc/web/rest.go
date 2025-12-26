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
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/errmap"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
	"strings"
)

func Rest(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.InfoContext(ctx, "request received", "method", r.Method, "url", r.URL, "headers", r.Header)

		if err := h(w, r); err != nil {
			slog.ErrorContext(ctx, "failed to request failed", "err", err, "method", r.Method, "url", r.URL)

			status, errs := HttpStatusFromErrs(err)
			writeJSON(w, status, ServiceView{Errors: errs, Status: status})
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

type RestHandler struct {
	db.Databases
	outbound.Generators
	outbound.RemoteAPIs
	ValidCountries map[string]bool
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

func (h *RestHandler) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	var body RegisterBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	var errm error
	if !isUsernameValid(body.Username) {
		errm = errmap.PutErrorMap(errm, "username", ErrHttpInvalidUsername)
	}
	if !isPasswordValid(body.Password) {
		errm = errmap.PutErrorMap(errm, "password", ErrHttpInvalidPassword)
	}
	if body.Password != body.ConfirmPassword {
		errm = errmap.PutErrorMap(errm, "confirmPassword", ErrHttpConfirmPassword)
	}
	if errm != nil {
		return errm
	}

	ctx := r.Context()
	user, err := svc.InsertUser(ctx, h.Pdb.Query, svc.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  svc.DefaultCountry,
		JoinedOn: h.GetNow(),
	})
	if errors.Is(err, svc.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	} else if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	t, err := SetSessionPlayer(ctx, h.Rdb, w, svc.MakePlayer(user.ID, user.Username, user.Country))
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

func (h *RestHandler) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	var body LoginBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := svc.VerifyUserTx(ctx, h.Pdb, body.Username, body.Password)
	if err != nil {
		switch err {
		case svc.ErrUserNotFound:
			return ErrHttpInvalidLogin
		case svc.ErrTooManyLoginAttempts:
			return ErrHttpTooManyLoginAttempts
		default:
			return fmt.Errorf("verify user: %w", err)
		}
	}
	return handleLoginSession(ctx, h.Rdb, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func (h *RestHandler) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) error {
	var body GoogleLoginBody
	if err := readJSON(r, &body); err != nil {
		return err
	}
	ctx := r.Context()

	payload, err := h.ValidateIDToken(ctx, body.Token)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	user, err := svc.SelectOrInsertGoogleUser(ctx, h.Pdb.Query, payload.AccountID, svc.GoogleUserInst{
		Username: payload.Username,
		Country:  svc.DefaultCountry,
	})
	if err != nil {
		return fmt.Errorf("upsert verified google user: %w", err)
	}
	return handleLoginSession(ctx, h.Rdb, w, user)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func (h *RestHandler) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	var body UpdatePasswordBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	var errm error
	if !isPasswordValid(body.NewPassword) {
		errm = errmap.PutErrorMap(errm, "newPassword", ErrHttpInvalidPassword)
	}
	if body.NewPassword != body.ConfirmNewPassword {
		errm = errmap.PutErrorMap(errm, "confirmNewPassword", ErrHttpConfirmPassword)
	}
	if errm != nil {
		return errm
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := svc.VerifyUserTx(ctx, h.Pdb, player.Name, body.Password)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return fmt.Errorf("verify user: %w", err)
	}

	if err := svc.UpdateUserPassword(ctx, h.Pdb.Query, user.ID, body.NewPassword); err != nil {
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

func (h *RestHandler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	var body UpdateUserBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	var errm error
	if !isUsernameValid(body.NewUsername) {
		errm = errmap.PutErrorMap(errm, "newUsername", ErrHttpInvalidUsername)
	}
	if !isBioValid(body.NewBio) {
		errm = errmap.PutErrorMap(errm, "newBio", ErrHttpInvalidBio)
	}
	if _, ok := h.ValidCountries[body.NewCountry]; !ok {
		return errmap.PutErrorMap(nil, "newCountry", ErrHttpInvalidCountry)
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	user, err := svc.UpdateUser(ctx, h.Pdb.Query, player.ID, svc.UpdtUserParams{
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

func (h *RestHandler) HandleCreateTempSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return ErrHttpSessionExpired
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	sessionID := MakeSessionID()
	if err := svc.SetSession(ctx, h.Rdb, sessionID, player, TempSessionMaxAge); err != nil {
		return fmt.Errorf("set session: %w", err)
	}

	slog.InfoContext(ctx, "created temporary user session", "user", player)
	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: sessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func (h *RestHandler) HandleRefreshSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	player, sessionID, err := GetSessionPlayer(ctx, h.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if err := svc.UpdateSessionEx(ctx, h.Rdb, sessionID, SessionMaxAge); err != nil {
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

func (h *RestHandler) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return fmt.Errorf("get cookie '%s': %w", CookieKey, err)
	}
	sessionID := cookie.Value

	ctx := r.Context()
	if err := svc.DeleteSession(ctx, h.Rdb, sessionID); err != nil {
		return fmt.Errorf("logging out session '%s': %w", sessionID, err)
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

type CreateGameResp struct {
	GameID string `json:"gameId"`
}

func (h *RestHandler) HandleCreateGame(w http.ResponseWriter, r *http.Request) error {
	var body CreateGameBody
	if err := readJSON(r, &body); err != nil {
		return err
	}
	ctx := r.Context()

	var initialBoard *chess.Board
	var color svc.Color
	var mode svc.GameMode
	var errm error

	if body.InitialFEN != "" {
		board, err := chess.ParseFen(body.InitialFEN)
		if err != nil {
			slog.WarnContext(ctx, "invalid initial FEN", "initialFEN", body.InitialFEN, "err", err)
			errm = errmap.PutErrorMap(errm, "initialFen", ErrHttpInvalidFen)
		} else {
			initialBoard = &board
		}
	}
	if c := svc.Colors.Parse(body.FirstColor); c == nil {
		errm = errmap.PutErrorMap(errm, "firstColor", ErrHttpInvalidColor)
	} else {
		color = *c
	}
	if m := svc.Modes.Parse(body.Mode); m == nil {
		errm = errmap.PutErrorMap(errm, "mode", ErrHttpInvalidMode)
	} else {
		mode = *m
	}
	if errm != nil {
		return errm
	}

	gameID, err := svc.CreateGame(ctx, h.Rdb, color, mode, initialBoard)
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

func (h *RestHandler) HandleUpdateChallenge(w http.ResponseWriter, r *http.Request) error {
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
		return errmap.PutErrorMap(nil, "action", ErrHttpInvalidAction)
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	if player.ID != targetID {
		return ErrHttpUpdateChallenge
	}

	dr, err := svc.DeleteChallenge(ctx, h.Pdb.Query, svc.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if errors.Is(err, svc.ErrChallengeNotFound) {
		return ErrHttpNotFoundChallenge
	} else if err != nil {
		return err
	}

	var gameID string
	if body.Action == "ACCEPT" {
		gameID, err = svc.CreateGame(ctx, h.Rdb, dr.FirstColor, dr.Mode, nil)
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

func (h *RestHandler) HandleCreateChallenge(w http.ResponseWriter, r *http.Request) error {
	var body CreateChallengeBody
	if err := readJSON(r, &body); err != nil {
		return err
	}

	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	var color svc.Color
	var mode svc.GameMode
	var errm error

	if c := svc.Colors.Parse(body.StartColor); c == nil {
		errm = errmap.PutErrorMap(errm, "startColor", ErrHttpInvalidColor)
	} else {
		color = *c
	}
	if m := svc.Modes.Parse(body.Mode); m == nil {
		errm = errmap.PutErrorMap(errm, "mode", ErrHttpInvalidMode)
	} else {
		mode = *m
	}
	if errm != nil {
		return errm
	}

	ret, err := svc.InsertChallengeRet(ctx, h.Pdb.Query, svc.ChallengeInst{
		ChallengerID: player.ID,
		ChallengeeID: body.ChallengeeID,
		Mode:         mode,
		StartColor:   color,
		MadeOn:       h.GetNow(),
	})
	if err != nil {
		switch err {
		case svc.ErrDuplicateChallenge:
			return ErrHttpDuplicateChallenge
		case svc.ErrParticipantConflict:
			return ErrHttpInvalidParticipants
		case svc.ErrSelfChallenge:
			return ErrHttpSelfChallenge
		default:
			return fmt.Errorf("insert challenge: %w", err)
		}
	}
	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})

	ctx = context.WithoutCancel(ctx)
	if err := svc.BroadcastChallenge(ctx, h.Rdb, ret); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast challenge", "challenge", ret, "err", err)
	}
	if err := svc.DeleteExpiredChallenges(ctx, h.Pdb.Query, player.ID, svc.ExpireChallengeThreshold); err != nil {
		slog.ErrorContext(ctx, "failed to delete expired challenges", "challenge", ret, "err", err)
	}
	return nil
}

func (h *RestHandler) HandleGetSelf(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	}
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	user, err := svc.GetUserByID(ctx, h.Pdb.Query, player.ID)
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

func (h *RestHandler) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	var errm error
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		errm = errmap.PutErrorMap(errm, "page", ErrHttpInvalidPage)
	}
	var mode svc.GameMode
	if m := svc.Modes.Parse(q.Get("mode")); m == nil {
		errm = errmap.PutErrorMap(errm, "mode", ErrHttpInvalidMode)
	} else {
		mode = *m
	}
	if errm != nil {
		return errm
	}

	lbd, err := svc.GetLeaderboardPage(ctx, h.Rdb, mode, int64(page), PerPage)
	if err != nil {
		return fmt.Errorf("get leaderboard page %d: %w", page, err)
	}
	users, err := svc.GetLeaderboardUsers(ctx, h.Pdb.Query, mode, lbd.RankedUsers)
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

func (h *RestHandler) HandleGetPlayer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	id, err := intQuery(q, "id")
	if err != nil {
		return errmap.PutErrorMap(nil, "id", ErrHttpInvalidID)
	}
	withReplaysStr := q.Get("withReplays")
	withReplays := strings.ToLower(withReplaysStr) == "true"

	eg, egCtx := errgroup.WithContext(ctx)

	var resp FullUserResp
	var lbRanks map[svc.GameMode]svc.LbRank

	eg.Go(func() (err error) {
		resp.User, err = svc.GetUserByID(egCtx, h.Pdb.Query, int64(id))
		return err
	})
	eg.Go(func() (err error) {
		resp.Stats, err = svc.GetUserElos(egCtx, h.Pdb.Query, int64(id))
		return err
	})
	eg.Go(func() (err error) {
		lbRanks, err = svc.GetLeaderboardRanks(egCtx, h.Rdb, int64(id), svc.ModesValues)
		return err
	})
	if withReplays {
		eg.Go(func() (err error) {
			resp.ReplayList, err = svc.GetUserReplays(egCtx, h.Pdb.Query, int64(id), -1, PerPage)
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

func (h *RestHandler) HandleSearchPlayers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		return errmap.PutErrorMap(nil, "page", ErrHttpInvalidPage)
	}
	name := q.Get("username")
	hasUser := name != ""

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []svc.UserEntity
	if hasUser {
		users, err := svc.SearchUsersByName(ctx, h.Pdb.Query, name, int32(page), PerPage)
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

func (h *RestHandler) HandleGetReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	id, err := intQuery(r.URL.Query(), "id")
	if err != nil {
		return errmap.PutErrorMap(nil, "page", ErrHttpInvalidID)
	}

	replay, err := svc.GetReplay(ctx, h.Pdb.Query, int64(id))
	if err != nil {
		return fmt.Errorf("get replay %d: %w", id, err)
	}
	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

func (h *RestHandler) HandleGetReplayMoveList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	id, err := intQuery(q, "userId")
	if err != nil {
		return errmap.PutErrorMap(nil, "page", ErrHttpInvalidID)
	}

	pbMoveHist, err := svc.GetReplayMoveHistory(ctx, h.Pdb.Query, int64(id))
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

func (h *RestHandler) HandleGetUserReplays(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	var errm error
	userID, err := intQuery(q, "userId")
	if err != nil {
		errm = errmap.PutErrorMap(errm, "userId", ErrHttpInvalidID)
	}
	afterID, err := intQuery(q, "afterId")
	if err != nil {
		errm = errmap.PutErrorMap(errm, "afterId", ErrHttpInvalidID)
	}
	if errm != nil {
		return errm
	}

	replays, err := svc.GetUserReplays(ctx, h.Pdb.Query, int64(userID), int64(afterID), PerPage)
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

func (h *RestHandler) HandleGetChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	participants := r.URL.Query().Get("participants")

	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	since := svc.MakeGetChallengesSince(h.GetNow())
	var challengeList []svc.ChallengeEntity
	switch participants {
	case "sent":
		challengeList, err = svc.GetChallengesByParticipant(ctx, h.Pdb.Query, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1}, since)
	case "received":
		challengeList, err = svc.GetChallengesByParticipant(ctx, h.Pdb.Query, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID}, since)
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

func (h *RestHandler) HandleGetChessRoomList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()

	var errm error
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		errm = errmap.PutErrorMap(errm, "page", ErrHttpInvalidPage)
	}
	count, err := intQueryDefault(q, "count", PerPage)
	if err != nil {
		errm = errmap.PutErrorMap(errm, "count", ErrHttpInvalidCount)
	}
	if errm != nil {
		return errm
	}

	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil && !errors.Is(err, svc.ErrSessionNotFound) {
		return fmt.Errorf("get session player: %w", err)
	}
	hasSession := !errors.Is(err, svc.ErrSessionNotFound)

	chessList, err := svc.GetAllChessMetas(ctx, h.Rdb, page, count)
	if err != nil {
		return fmt.Errorf("get page %d chess meta views: %w", page, err)
	}
	var selfChessList []svc.ChessMeta
	if hasSession {
		cl, err := svc.GetUserChessMetas(ctx, h.Rdb, player.ID)
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

func (h *RestHandler) HandleGetEloHistories(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()

	var errm error
	userID, err := intQuery(q, "userId")
	if err != nil {
		errm = errmap.PutErrorMap(errm, "userID", ErrHttpInvalidPage)
	}
	months, ok := timeframeMap[queryDefault(q, "timeframe", "all")]
	if !ok {
		errm = errmap.PutErrorMap(errm, "timeframe", ErrHttpInvalidTimeframe)
	}
	if errm != nil {
		return errm
	}

	ctx := r.Context()
	params := svc.EloHistoriesParams{UserID: int64(userID), Months: months, TimeUntil: h.GetNow()}
	eloBuckets, _, err := svc.RetrieveEloHistoryBuckets(ctx, &h.Databases, params)
	if err != nil {
		return fmt.Errorf("retrieve elo histories buckets with params %v: %w", params, err)
	}
	writeJSON(w, http.StatusOK, EloHistoriesResp{Buckets: eloBuckets})

	//w.Header().Set("Cache-Control", GetEloHistoriesCacheControl)
	return nil
}
