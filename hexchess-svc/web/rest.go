package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/svc"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/api/idtoken"
	"google.golang.org/protobuf/proto"
)

type RestHandler = func(state *ServerState, w http.ResponseWriter, r *http.Request) error

func makeRestHandler(state *ServerState, h RestHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.InfoContext(ctx, "request received", "method", r.Method, "url", r.URL, "headers", r.Header)

		if err := h(state, w, r); err != nil {
			slog.ErrorContext(ctx, "failed to request failed", "err", err, "method", r.Method, "url", r.URL)

			status, m := HttpStatusFromErr(err)
			writeJSON(w, status, ServiceView{Message: m, Status: status})
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
		if _, err = w.Write(b); err != nil {
			slog.ErrorContext(r.Context(), "write json", "err", err)
		}
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

func HandleRegister(state *ServerState, w http.ResponseWriter, r *http.Request) error {
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
	user, err := svc.InsertUser(ctx, state.Pdb.Query, svc.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  "us",
		Elo:      svc.StartElo,
	})
	if errors.Is(err, svc.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	t, err := SetSessionPlayer(ctx, state.Rdb, w, svc.PlayerState{
		ID:      user.ID,
		Name:    user.Username,
		Country: user.Country,
		Elo:     user.Elo,
		IsGuest: false,
	})
	if err != nil {
		return fmt.Errorf("set session player: %w", err)
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

func handleLoginSession(ctx context.Context, rdb *db.Redis, w http.ResponseWriter, user svc.VerifiedUser) error {
	t, err := SetSessionPlayer(ctx, rdb, w, svc.PlayerState{
		ID:      user.ID,
		Name:    user.Username,
		Country: user.Country,
		Elo:     user.Elo,
		IsGuest: false,
	})
	if err != nil {
		return fmt.Errorf("set session player: %w", err)
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

type LoginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HandleLogin(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body LoginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	ctx := r.Context()
	user, err := svc.VerifyUserTx(ctx, state.Pdb, body.Username, body.Password)
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

	return handleLoginSession(ctx, state.Rdb, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

const UsernameClaim string = "email"

func HandleGoogleLogin(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body GoogleLoginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	ctx := r.Context()

	payload, err := idtoken.Validate(ctx, body.Token, state.GoogleAPIKey)
	if err != nil {
		return fmt.Errorf("validate google id token: %w", err)
	}
	googleAccountID := payload.Subject
	username, ok := payload.Claims[UsernameClaim].(string)
	if !ok {
		return fmt.Errorf("expected claim '%s' to be provided in payload: %v", UsernameClaim, payload)
	}

	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", googleAccountID)

	user, err := svc.SelectOrInsertGoogleUser(ctx, state.Pdb.Query, googleAccountID, svc.GoogleUserInst{
		Username: username,
		Country:  "us",
		Elo:      svc.StartElo,
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}
	if err := validatePassword(body.NewPassword, body.ConfirmNewPassword); err != nil {
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
	}
	if err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}

	if _, ok := state.CountryMap[body.NewCountry]; !ok {
		return ErrHttpInvalidCountry
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
		Elo:      user.Elo,
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
	}
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}
	sessionID, err := MakeSessionID()
	if err != nil {
		return err
	}
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
	}
	if err != nil {
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
		Elo:      player.Elo,
		TTLSecs:  SessionMaxAge,
	}})
	return nil
}

func HandleLogout(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return err
	}
	sessionID := cookie.Value

	ctx := r.Context()
	if err := svc.DeleteSession(ctx, state.Rdb, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type CreateGameBody struct {
	FirstColor  string `json:"firstColor"`
	TimeControl string `json:"timeControl"`
	InitialFEN  string `json:"initialFen"`
}

type CreateGameResp struct {
	GameID string `json:"gameId"`
}

func HandleCreateGame(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body CreateGameBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return err
	}
	ctx := r.Context()

	var initialBoard *chess.Board
	if body.InitialFEN != "" {
		board, err := chess.ParseFen(body.InitialFEN)
		if err != nil {
			slog.WarnContext(ctx, "parse initial fen", "fen", body.InitialFEN, "err", err)
			return ErrInvalidFen
		}
		initialBoard = &board
	}

	gameID, err := svc.CreateGame(ctx, state.Rdb, svc.ColorSelect(body.FirstColor), svc.TimeControl(body.TimeControl), initialBoard)
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
		return fmt.Errorf("get session player: %w", err)
	}
	if player.ID != targetID {
		return ErrHttpUpdateChallenge
	}

	dr, err := svc.DeleteChallenge(ctx, state.Pdb.Query, svc.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if errors.Is(err, svc.ErrChallengeNotFound) {
		return ErrHttpNotFoundChallenge
	}
	if err != nil {
		return fmt.Errorf("delete challenge: %w", err)
	}
	slog.InfoContext(ctx, "deleted challenge", "challengerID", body.ChallengerID, "challengeeID", body.ChallengeeID)

	gameID := ""
	if body.Action == "ACCEPT" {
		gameID, err = svc.CreateGame(ctx, state.Rdb, dr.FirstColor, dr.TimeControl, nil)
		if err != nil {
			return fmt.Errorf("create game: %w", err)
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

func HandleCreateChallenge(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	var body CreateChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		TimeControl:  svc.TimeControl(body.TimeControl),
		StartColor:   svc.ColorSelect(body.StartColor),
		MadeOn:       time.Now(),
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
	if err := svc.BroadcastChallenge(ctx, state.Rdb, player.ID, ret); err != nil {
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
	TotalPages int              `json:"totalPages"`
	UserList   []svc.UserEntity `json:"userList,omitempty"`
}

func HandleGetLeaderboard(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query := r.URL.Query()
	page, err := parsePageQuery(ctx, query)
	if err != nil {
		return err
	}

	lbd, err := svc.GetLeaderboardPage(ctx, state.Rdb, int64(page), PerPage)
	if err != nil {
		return fmt.Errorf("get leaderboard page %d: %w", page, err)
	}
	users, err := svc.GetRankedUsers(ctx, state.Pdb.Query, lbd.Users)
	if err != nil {
		return fmt.Errorf("get ranked users: %w", err)
	}
	if err := svc.JoinRanks(lbd.Users, users); err != nil {
		return err
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	if users == nil {
		users = []svc.UserEntity{}
	}
	writeJSON(w, http.StatusOK, LeaderboardResp{TotalPages: lbd.PageCount, UserList: users})
	return nil
}

type FullUserResp struct {
	User       svc.UserEntity     `json:"user"`
	ReplayList []svc.ReplayEntity `json:"replayList,omitempty"`
}

func HandleGetPlayer(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	id, err := parseIntQuery(ctx, r.URL.Query(), "id")
	if err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)

	var user svc.UserEntity
	var userRank int64
	var replayList []svc.ReplayEntity

	eg.Go(func() error {
		u, err := svc.GetUserByID(egCtx, state.Pdb.Query, int64(id))
		if err != nil {
			return fmt.Errorf("get user %d by id: %w", id, err)
		}
		user = u
		return nil
	})
	eg.Go(func() error {
		rs, err := svc.GetUserReplays(egCtx, state.Pdb.Query, int64(id), -1, PerPage)
		if err != nil {
			return fmt.Errorf("get user replays %d by id: %w", id, err)
		}
		replayList = rs
		return err
	})
	eg.Go(func() error {
		ur, err := svc.GetLeaderboardRank(egCtx, state.Rdb, int64(id))
		if err != nil {
			return fmt.Errorf("failed leaderboard %d rank by id: %w", id, err)
		}
		userRank = ur
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	user.Rank = userRank

	if replayList == nil {
		replayList = []svc.ReplayEntity{}
	}
	slog.InfoContext(ctx, "retrieved user with replays", "user", user, "replays", replayList)
	writeJSON(w, http.StatusOK, FullUserResp{User: user, ReplayList: replayList})
	return nil
}

type SearchPlayersResp struct {
	UserList []svc.UserEntity `json:"userList,omitempty"`
}

func HandleSearchPlayers(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query := r.URL.Query()
	page, err := parsePageQuery(ctx, query)
	if err != nil {
		return err
	}
	name := query.Get("username")

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []svc.UserEntity
	if name != "" {
		userList, err = svc.SearchUsersByName(ctx, state.Pdb.Query, name, int32(page), PerPage)
		if errors.Is(err, svc.ErrSearchLimit) {
			return ErrHttpSearchLimit
		}
		if err != nil {
			return fmt.Errorf("search users by name '%s': %w", name, err)
		}
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

	id, err := parseIntQuery(ctx, r.URL.Query(), "id")
	if err != nil {
		return err
	}

	replay, err := svc.GetReplay(ctx, state.Pdb.Query, int64(id))
	if err != nil {
		return fmt.Errorf("get replay: %w", err)
	}
	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

func HandleGetReplayMoveList(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	id, err := parseIntQuery(ctx, r.URL.Query(), "id")
	if err != nil {
		return err
	}

	pbMoveHist, err := svc.GetReplayMoveHistory(ctx, state.Pdb.Query, int64(id))
	if err != nil {
		return fmt.Errorf("get replay %d moveHistory: %w", id, err)
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

	query := r.URL.Query()
	afterID, aErr := parseIntQuery(ctx, query, "afterId")
	userID, uErr := parseIntQuery(ctx, query, "userId")
	if err := errsOr(aErr, uErr); err != nil {
		return err
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

	var challengeList []svc.ChallengeEntity
	switch participants {
	case "sent":
		challengeList, err = svc.GetChallengesByParticipant(ctx, state.Pdb.Query, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1}, svc.ExpireChallengeThreshold)
	case "received":
		challengeList, err = svc.GetChallengesByParticipant(ctx, state.Pdb.Query, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID}, svc.ExpireChallengeThreshold)
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

	query := r.URL.Query()
	page, pageErr := parsePageQuery(ctx, query)
	count, countErr := parseCountQuery(ctx, query)
	if err := errsOr(pageErr, countErr); err != nil {
		return err
	}

	var hasSession bool

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		if err != svc.ErrSessionNotFound {
			return fmt.Errorf("get session player: %w", err)
		}
	} else {
		hasSession = true
	}

	chessList, err := svc.GetAllChessMetas(ctx, state.Rdb, page, count)
	if err != nil {
		return fmt.Errorf("get all chess meta views: %w", err)
	}
	var selfChessList []svc.ChessMeta
	if hasSession {
		chessList, err := svc.GetUserChessMetas(ctx, state.Rdb, player.ID)
		if err != nil {
			return fmt.Errorf("get all chess meta views: %w", err)
		}
		selfChessList = chessList
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

var timeframeMap = map[string]int{
	"1m":  1,
	"3m":  3,
	"6m":  6,
	"1y":  12,
	"all": 0,
}

func HandleGetEloHistories(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query := r.URL.Query()
	userID, err := parseIntQuery(ctx, query, "userId")
	if err != nil {
		return err
	}
	timeframe := query.Get("timeframe")
	if timeframe == "" {
		timeframe = "all"
	}
	months, ok := timeframeMap[timeframe]
	if !ok {
		slog.WarnContext(ctx, "invalid timeframe", "timeframe", timeframe)
		return ErrHttpInvalidRequest
	}

	eloBuckets, _, err := svc.RetrieveEloHistoryBuckets(ctx, &state.Databases, time.Now(), svc.EloHistoriesParams{UserID: int64(userID), Months: months})
	if err != nil {
		return fmt.Errorf("retrieve elo histories buckets: %w", err)
	}
	writeJSON(w, http.StatusOK, EloHistoriesResp{Buckets: eloBuckets})

	//w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%f", bucketDuration.Seconds()))
	return nil
}
