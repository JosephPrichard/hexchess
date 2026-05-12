package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/internal/errutil"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	svc "hexchess-svc/service"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func Rest(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.InfoContext(ctx, "received REST call", "method", r.Method, "url", r.URL, "headers", r.Header)

		if err := h(w, r); err != nil {
			resp := HttpStatusFromErrs(err)
			writeJSON(w, resp.Status, resp)

			slog.Log(ctx, LevelFromStatus(resp.Status), "failed to handle REST call", "Err", err, "method", r.Method, "url", r.URL)
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
			slog.ErrorContext(r.Context(), "write json", "Err", err)
		}
	}
}

type RegisterBody struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (api *API) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body RegisterBody
	if err := parseJSON(r, &body, validateRegisterBody); err != nil {
		return err
	}

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
	if respErr.HasErrors() {
		return respErr.Inner()
	}

	user, err := api.services.InsertUser(ctx, svc.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  model.DefaultCountry,
		JoinedOn: time.Now(),
	})
	if errors.Is(err, svc.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	} else if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	ttl, err := api.authenticator.SetSessionPlayer(ctx, w, model.MakePlayer(user.ID, user.Username, user.Country))
	if err != nil {
		return err
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

func (api *API) handleLoginSession(ctx context.Context, w http.ResponseWriter, user svc.VerifiedUser) error {
	t, err := api.authenticator.SetSessionPlayer(ctx, w, model.MakePlayer(user.ID, user.Username, user.Country))
	if err != nil {
		return err
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

func (api *API) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body LoginBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}

	user, err := api.services.VerifyUser(ctx, body.Username, body.Password)
	switch {
	case errors.Is(err, svc.ErrUserNotFound):
		return ErrHttpInvalidLogin
	case errors.Is(err, svc.ErrTooManyLoginAttempts):
		return ErrHttpTooManyLoginAttempts
	case err != nil:
		return fmt.Errorf("verify user: %w", err)
	}

	return api.handleLoginSession(ctx, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func (api *API) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body GoogleLoginBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}

	payload, err := api.services.ValidateGoogleIDToken(ctx, body.Token)
	if err != nil {
		return fmt.Errorf("validate google login id token: %w", err)
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	user, err := api.services.SelectOrInsertGoogleUser(ctx, payload.AccountID, svc.GoogleUserInst{
		Username: payload.Username,
		Country:  model.DefaultCountry,
	})
	if err != nil {
		return fmt.Errorf("upsert verified google user: %w", err)
	}
	return api.handleLoginSession(ctx, w, user)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func (api *API) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	var body UpdatePasswordBody
	if err := parseJSON(r, &body, validateUpdatePasswordBody); err != nil {
		return err
	}

	user, err := api.services.VerifyUser(ctx, player.Name, body.Password)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return fmt.Errorf("verify user: %w", err)
	}

	if err := api.services.UpdateUserPassword(ctx, user.ID, body.NewPassword); err != nil {
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

func (api *API) HandleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	var body UpdateUserBody
	if err := parseJSON(r, &body, func(body UpdateUserBody) error {
		return validateUpdateUserBody(api.staticData, body)
	}); err != nil {
		return err
	}

	user, err := api.services.UpdateUser(ctx, player.ID, svc.UpdtUserParams{
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

func (api *API) HandleCreateTempSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	alreadyHasSession := true

	sessionPlayer, err := api.authenticator.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		alreadyHasSession = false
	} else if err != nil {
		return err
	}

	var tempSessionID string
	var sessions []svc.SessionInst

	if alreadyHasSession {
		tempSessionID = MakeSessionID()
		sessions = []svc.SessionInst{
			{SessionID: tempSessionID, Player: sessionPlayer, Expiry: TempSessionMaxAge},
		}
	} else {
		sessionPlayer = model.MakeGuestPlayer()
		tempSessionID = MakeSessionID()
		guestSessionID := MakeSessionID()

		sessions = []svc.SessionInst{
			{SessionID: tempSessionID, Player: sessionPlayer, Expiry: TempSessionMaxAge},
			{SessionID: guestSessionID, Player: sessionPlayer, Expiry: SessionMaxAge},
		}

		w.Header().Set("Set-Cookie", FmtCookie(guestSessionID))
	}

	slog.InfoContext(ctx, "created sessions", "sessions", sessions)

	if err := api.services.SetSessions(ctx, sessions...); err != nil {
		return fmt.Errorf("set sessions: %w", err)
	}

	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: tempSessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func (api *API) HandleRefreshSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, sessionID, err := api.authenticator.GetSessionPlayerAndID(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return err
	}

	if err := api.services.UpdateSessionEx(ctx, sessionID, SessionMaxAge); err != nil {
		return fmt.Errorf("update session with expiry: %d %w", SessionMaxAge, err)
	}

	w.Header().Set("Set-Cookie", FmtCookie(sessionID))
	slog.InfoContext(ctx, "refreshed user session", "user", player)

	writeJSON(w, http.StatusOK, RefreshResp{
		Session: &SessionView{
			ID:       player.ID,
			Username: player.Name,
			Country:  player.Country,
			TTLSecs:  SessionMaxAge,
		},
	})
	return nil
}

func (api *API) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return fmt.Errorf("get cookie %s: %w", CookieKey, err)
	}
	sessionID := cookie.Value

	if err := api.services.DeleteSession(r.Context(), sessionID); err != nil {
		return fmt.Errorf("logging ext session %s: %w", sessionID, err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (api *API) HandleGetSelf(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	}
	if err != nil {
		return err
	}

	user, err := api.services.GetUserByID(ctx, player.ID)
	if err != nil {
		return fmt.Errorf("get user by id %+v: %w", player, err)
	}
	slog.InfoContext(ctx, "retrieved user", "user", user)

	writeJSON(w, http.StatusOK, user)
	return nil
}

type LeaderboardQuery struct {
	Page int
	Mode model.GameMode
}

type LeaderboardResp struct {
	TotalPages int             `json:"totalPages"`
	UserList   []model.LbdUser `json:"userList"`
}

func (api *API) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseLeaderboardQuery(r.URL.Query())
	if err != nil {
		return err
	}

	leaderboard, err := api.services.GetLeaderboardPage(ctx, query.Mode, int64(query.Page), defaultPaginationCount)
	if err != nil {
		return fmt.Errorf("get leaderboard page %d: %w", query.Page, err)
	}
	users, _, err := api.services.GetLeaderboardUsers(ctx, query.Mode, leaderboard.RankedUsers)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	writeJSON(w, http.StatusOK, LeaderboardResp{TotalPages: leaderboard.PageCount, UserList: users})
	return nil
}

type GetPlayersResp struct {
	svc.FullUser
}

func (api *API) HandleGetPlayer(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	userIDStr := r.URL.Query().Get("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return ofRespError("id", fmt.Errorf("failed to parse integer: '%s'", userIDStr))
	}

	fullUser, err := api.services.GetFullUser(ctx, int64(userID), defaultPaginationCount)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpNotFoundUser
	} else if err != nil {
		return fmt.Errorf("get full user %d: %w", userID, err)
	}

	playersResp := GetPlayersResp{FullUser: fullUser}

	slog.InfoContext(ctx, "retrieved user with replays", "fullUser", playersResp)

	writeJSON(w, http.StatusOK, playersResp)
	return nil
}

type SearchPlayersResp struct {
	UserList []model.LbdUser `json:"userList"`
}

func (api *API) HandleSearchPlayers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	queryCtx := QueryParseCtx{Values: r.URL.Query(), RespErr: &ResponseError{}}

	page := parseDefaultInt(queryCtx, "page", 1)
	name := queryCtx.Values.Get("username")

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	var userList []model.LbdUser
	if name != "" {
		users, err := api.services.GetFuzzySearchLeaderboard(ctx, name, int32(page), defaultPaginationCount)
		if err != nil {
			return fmt.Errorf("search users by name=%s: %w", name, err)
		}
		userList = users
	}

	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: userList})
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

func (api *API) HandleUpdateChallenge(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	body, err := transformJSON(r, parseUpdateChallengeBody)
	if err != nil {
		return err
	}

	if player.ID != body.TargetID {
		return ErrHttpUpdateChallenge
	}

	deleteResult, err := api.services.DeleteChallenge(ctx, svc.ChallengeKey{ChallengerID: body.ChallengerID, ChallengeeID: body.ChallengeeID})
	if err != nil {
		if errors.Is(err, svc.ErrChallengeNotFound) {
			return ErrHttpNotFoundChallenge
		}
		return err
	}

	var gameID string
	if body.Action == Accept {
		gameID, err = api.services.CreateGame(ctx, deleteResult.FirstColor, deleteResult.Mode, nil)
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

func (api *API) HandleCreateChallenge(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	body, err := transformJSON(r, parseCreateChallengeBody)
	if err != nil {
		return err
	}

	ret, err := api.services.InsertChallenge(ctx, svc.ChallengeInst{
		ChallengerID: player.ID,
		ChallengeeID: body.ChallengeeID,
		Mode:         body.Mode,
		StartColor:   body.StartColor,
		MadeOn:       time.Now(),
	})
	switch {
	case errors.Is(err, svc.ErrDuplicateChallenge):
		return ErrHttpDuplicateChallenge
	case errors.Is(err, svc.ErrInvalidChallengeMember):
		return ErrHttpInvalidParticipants
	case errors.Is(err, svc.ErrSelfChallenge):
		return ErrHttpSelfChallenge
	case err != nil:
		return fmt.Errorf("insert challenge: %w", err)
	}

	go func() {
		detatchedCtx := context.WithoutCancel(ctx)
		if err := api.broadcaster.BroadcastChallenge(detatchedCtx, ret); err != nil {
			slog.ErrorContext(detatchedCtx, "failed to broadcast challenge", "challenge", ret, "Err", err)
		}
	}()

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetChallengesResp struct {
	ChallengeList []model.Challenge `json:"challengeList"`
}

const (
	SentParticipantsTarget    = "sent"
	ReceivedParticipantTarget = "received"
)

func (api *API) HandleGetChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	participants := r.URL.Query().Get("participants")

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	var challengeList []model.Challenge

	switch participants {
	case SentParticipantsTarget:
		listByChallenger, err := api.services.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1})
		if err != nil {
			return fmt.Errorf("get challenges by challenger: %w", err)
		}
		challengeList = listByChallenger
	case ReceivedParticipantTarget:
		listByChallengee, err := api.services.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID})
		if err != nil {
			return fmt.Errorf("get challenges by challengee: %w", err)
		}
		challengeList = listByChallengee
	}

	slog.InfoContext(ctx, "retrieved challenges", "challengeList", challengeList)
	writeJSON(w, http.StatusOK, GetChallengesResp{ChallengeList: challengeList})
	return nil
}

type CountChallengesResp struct {
	Count int64 `json:"count"`
}

func (api *API) HandleCountUserChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	count, err := api.services.CountUserChallenges(ctx, player.ID)
	if err != nil {
		return fmt.Errorf("count user challenges: %w", err)
	}

	writeJSON(w, http.StatusOK, CountChallengesResp{Count: count})
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

func (api *API) HandleCreateGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	body, err := transformJSON(r, parseCreateGameBody)
	if err != nil {
		return err
	}

	gameID, err := api.services.CreateGame(ctx, body.FirstColor, body.Mode, &body.InitialBoard)
	if err != nil {
		return fmt.Errorf("create game: %w", err)
	}
	slog.InfoContext(ctx, "created game", "gameID", gameID, "body", body)

	writeJSON(w, http.StatusOK, CreateGameResp{GameID: gameID})
	return nil
}

const (
	GameExists    = "GAME_EXISTS"
	GameNotExists = "GAME_NOT_EXISTS"
)

func (api *API) HandleGameExistence(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	gameID := r.URL.Query().Get("gameId")

	exists := api.services.IsGameAccessible(ctx, gameID)

	respMsg := GameExists
	if !exists {
		respMsg = GameNotExists
	}
	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: respMsg})
	return nil
}

type ChessMeta struct {
	ID          string            `json:"id"`
	WhitePlayer model.PlayerState `json:"whitePlayer"`
	BlackPlayer model.PlayerState `json:"blackPlayer"`
	FirstColor  string            `json:"firstColor"`
	Mode        string            `json:"mode"`
	Touch       time.Time         `json:"touch"`
}

func mapChessMetas(svcMetas []model.ChessMeta) []ChessMeta {
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

type ChessMetasResp struct {
	ChessList     []ChessMeta `json:"chessList"`
	SelfChessList []ChessMeta `json:"selfChessList"`
}

func (api *API) HandleGetChessMetas(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseChessMetasQuery(r.URL.Query())
	if err != nil {
		return err
	}

	hasSession := true

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		hasSession = false
	} else if err != nil {
		return err
	}

	var allChessMetas []model.ChessMeta
	var selfChessMetas []model.ChessMeta

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		allChessMetas, err = api.services.GetAllChessMetas(egCtx, query.Page, query.Count)
		return errutil.Guardf(err, "get all chess metas page %d", query.Page)
	})
	if hasSession {
		eg.Go(func() (err error) {
			selfChessMetas, err = api.services.GetUserChessMetas(egCtx, player.ID)
			return errutil.Guardf(err, "get user %d chess metas", player.ID)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, ChessMetasResp{
		ChessList:     mapChessMetas(allChessMetas),
		SelfChessList: mapChessMetas(selfChessMetas),
	})
	return nil
}

func (api *API) HandleGetGameChats(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := r.URL.Query().Get("gameId")

	chats, err := api.services.GetStateChats(ctx, gameID, 100)
	if err != nil {
		return fmt.Errorf("get state chats: %w", err)
	}
	bytes, err := proto.Marshal(&pb.ChatMessages{
		Chats: chats,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal chats: %w", err)
	}

	writeBytes(w, http.StatusOK, bytes)
	return nil
}

type GetReplayResp struct {
	Replay model.FullReplay `json:"replay"`
}

func (api *API) HandleGetReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseReplayQueryBody(r.URL.Query())
	if err != nil {
		return err
	}

	var replay model.FullReplay

	if query.HasGameID {
		replay, err = api.services.GetReplayByGameID(ctx, query.GameID)
	} else {
		replay, err = api.services.GetReplay(ctx, query.ReplayID)
	}
	if errors.Is(err, svc.ErrNoReplay) {
		return ErrHttpNotFoundReplay
	} else if err != nil {
		return fmt.Errorf("get replay by query %+v: %w", query, err)
	}

	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

type GetReplaysResp struct {
	ReplayList []model.FullReplay `json:"replayList"`
}

func (api *API) HandleGetReplays(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseReplaysQuery(r.URL.Query())
	if err != nil {
		return err
	}

	replays, err := api.services.SearchReplaysByQuery(ctx, query)
	if err != nil {
		return fmt.Errorf("get replays by query %+v: %w", query, err)
	}

	writeJSON(w, http.StatusOK, GetReplaysResp{ReplayList: replays})
	return nil
}

type EloHistoriesResp struct {
	Buckets svc.EloHistoryBuckets `json:"buckets"`
}

var GetEloHistoriesCacheControl = fmt.Sprintf("public, max-age=%f", svc.ShortBucketDuration.Seconds())

func (api *API) HandleGetEloHistories(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseEloHistoriesQuery(r.URL.Query())
	if err != nil {
		return err
	}

	params := svc.EloHistoriesParams{
		UserID:    int64(query.UserID),
		Months:    query.Months,
		TimeUntil: time.Now(),
	}
	eloBuckets, _, err := api.services.RetrieveEloHistoryBuckets(ctx, params)
	if err != nil {
		return fmt.Errorf("retrieve elo histories buckets with params %v: %w", params, err)
	}
	writeJSON(w, http.StatusOK, EloHistoriesResp{Buckets: eloBuckets})

	//w.Header().Set("Cache-Control", GetEloHistoriesCacheControl)
	return nil
}

func (api *API) HandleGetMoveReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	replayIDStr := r.URL.Query().Get("replayId")
	replayID, err := strconv.Atoi(replayIDStr)
	if err != nil {
		return ofRespError("replayId", fmt.Errorf("failed to parse integer: '%s'", replayIDStr))
	}

	bytes, err := api.services.GetMovesHistory(ctx, replayID)
	if err != nil {
		return err
	}

	writeBytes(w, http.StatusOK, bytes)
	// aggressive cache control because this resource does not change, but the algorithm we are using to parse it might if requirements change.
	//w.Header().Set("Cache-Control", "public, max-age=86400")
	return nil
}

type CreateTournamentBody struct {
	Name      string        `json:"name"`
	Mode      string        `json:"mode"`
	Ruleset   string        `json:"ruleset"`
	Rounds    int32         `json:"rounds"`
	Countdown time.Duration `json:"duration"`
}

type CreateTournamentResp struct {
	TournamentKey string `json:"tournamentKey"`
}

func (api *API) HandleCreateTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	body, err := transformJSON(r, parseCreateTournamentBody)
	if err != nil {
		return err
	}

	tournamentKey := uuid.New()
	tournamentID, err := api.services.CreateTournament(ctx, svc.TournamentInst{
		Key:       tournamentKey,
		Name:      body.Name,
		Rounds:    body.Rounds,
		Mode:      body.Mode,
		Ruleset:   body.Ruleset,
		Countdown: body.Countdown,
		CreatedOn: time.Now(),
		CreatedBy: player.ID,
	})
	if errors.Is(err, svc.ErrInvalidRounds) {
		return ErrHttpInvalidRounds
	} else if err != nil {
		return fmt.Errorf("insert tournament: %w", err)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentID", tournamentID, "tournamentKey", tournamentKey)

	writeJSON(w, http.StatusCreated, CreateTournamentResp{TournamentKey: tournamentKey.String()})
	return nil
}

type TournamentKeyBody struct {
	TournamentKey string `json:"tournamentKey"`
}

func (api *API) HandleJoinTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	var body TournamentKeyBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}
	tournamentKey, err := uuid.Parse(body.TournamentKey)
	if err != nil {
		return ofRespError("tournamentKey", BadRequestError{err})
	}

	lbdUser, err := api.services.JoinTournamentAndSelectUser(ctx, svc.JoinTournamentInst{
		TournamentKey: tournamentKey,
		JoiningUserID: player.ID,
		InsertionTime: time.Now(),
	})
	switch {
	case errors.Is(err, svc.ErrTournamentNotFound):
		return ErrHttpNotFoundTournament
	case errors.Is(err, svc.ErrTooManyParticipants):
		return ErrHttpTooManyParticipants
	case errors.Is(err, svc.ErrTournamentNotLobby):
		return ErrHttpTournamentNotLobby
	case err != nil:
		return fmt.Errorf("join tournament by tournament id %s: %w", body.TournamentKey, err)
	}

	slog.InfoContext(ctx, "participant joined tournament", "joiningID", player.ID, "tournamentKey", body.TournamentKey)

	go func() {
		detatchedCtx := context.WithoutCancel(ctx)
		err = api.broadcaster.BroadcastTournament(detatchedCtx, model.SerializeParticipantOutput(tournamentKey, lbdUser))
		if err != nil {
			slog.ErrorContext(detatchedCtx, "failed to broadcast tournament participant", "Err", err)
		}
	}()

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (api *API) HandleBeginCountdownTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	var body TournamentKeyBody
	if err := parseJSON(r, &body, nil); err != nil {
		return err
	}
	tournamentKey, err := uuid.Parse(body.TournamentKey)
	if err != nil {
		return ofRespError("tournamentKey", BadRequestError{err})
	}

	result, err := api.services.BeginTournamentCountdown(ctx, tournamentKey, player.ID)
	switch {
	case errors.Is(err, svc.ErrInvalidCountdownTournamentStatus):
		return ErrHttpInvalidCountdownState
	case errors.Is(err, svc.ErrTournamentCountdownPermissions):
		return ErrHttpCountdownPermissions
	case err != nil:
		return fmt.Errorf("begin tournament countdown by tournament id %s: %w", body.TournamentKey, err)
	}

	slog.InfoContext(ctx, "successfully started countdown for tournament", "countdownResult", result)

	go func() {
		detatchedCtx := context.WithoutCancel(ctx)
		err := api.broadcaster.BroadcastTournament(detatchedCtx, model.SerializeBeginTournamentCountdown(result.TournamentKey))
		if err != nil {
			slog.ErrorContext(detatchedCtx, "failed to broadcast tournament participant", "Err", err)
		}
	}()

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetTournamentResp model.FullTournament

func (api *API) HandleGetTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	tournamentKey, err := uuid.Parse(r.URL.Query().Get("tournamentKey"))
	if err != nil {
		return ofRespError("tournamentKey", err)
	}

	tournament, err := api.services.GetTournament(ctx, tournamentKey)
	if errors.Is(err, svc.ErrTournamentNotFound) {
		return ErrHttpNotFoundTournament
	} else if err != nil {
		return fmt.Errorf("get tournament by key: %w", err)
	}

	slog.Info("retrieved full tournament", "tournament", tournament)

	writeJSON(w, http.StatusOK, GetTournamentResp(tournament))
	return nil
}

type GetTournamentsResp struct {
	Tournaments []model.Tournament `json:"tournaments"`
}

func (api *API) HandleGetTournaments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseTournamentsQuery(r.URL.Query())
	if err != nil {
		return err
	}

	tournaments, err := api.services.GetTournaments(ctx, query.UserID, query.AfterID, defaultPaginationCount)
	if err != nil {
		return fmt.Errorf("get tournaments: %w", err)
	}
	slog.Info("retrieved tournaments", "tournaments", tournaments)

	writeJSON(w, http.StatusOK, GetTournamentsResp{Tournaments: tournaments})
	return nil
}

func (api *API) HandleLeaveTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	tournamentKey, err := uuid.Parse(r.URL.Query().Get("tournamentKey"))
	if err != nil {
		return ofRespError("tournamentKey", err)
	}

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	didLeave, err := api.services.LeaveTournament(ctx, tournamentKey, player.ID)
	if err != nil {
		return fmt.Errorf("leave tournament: %w", err)
	}
	slog.Info("attempted to leave tournament", "didLeave", didLeave, "tournamentKey", tournamentKey, "player", player)

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetUserActivityResp struct {
	IsUserActive bool `json:"isUserActive"`
}

func (api *API) HandleUserActivityCheck(w http.ResponseWriter, r *http.Request) error {
	userID := r.URL.Query().Get("userId")

	isActive := api.services.IsActiveUser(r.Context(), userID)

	writeJSON(w, http.StatusOK, GetUserActivityResp{IsUserActive: isActive})
	//w.Header().Set("Cache-Control", "public, max-age=30")
	return nil
}
