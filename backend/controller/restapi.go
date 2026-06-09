package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	svc "hexchess-svc/service"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func Rest(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		slog.InfoContext(ctx, "received REST call", "method", r.Method, "url", r.URL, "headers", r.Header)

		if err := h(w, r); err != nil {
			resp := ServiceViewFromErr(err)
			writeJSON(w, resp.Status, resp)

			logutil.RootLog(ctx, LevelFromStatus(resp.Status), "failed to handle REST call", err, "method", r.Method, "url", r.URL)
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
			slog.ErrorContext(r.Context(), "write json", "error", err)
		}
	}
}

type RegisterBody struct {
	Username        string `json:"username" validate:"required,min=5,max=35"`
	Password        string `json:"password" validate:"required,min=11,max=100"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (api *API) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body RegisterBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}
	if body.Password != body.ConfirmPassword {
		return ErrHttpConfirmPassword
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
		return serrors.New("insert user", err)
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
	if err := parseJSON(r, &body); err != nil {
		return err
	}

	user, err := api.services.VerifyUser(ctx, body.Username, body.Password)
	if err != nil {
		return serrors.New("verify user", err)
	}

	return api.handleLoginSession(ctx, w, user)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func (api *API) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body GoogleLoginBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}

	payload, err := api.services.ValidateGoogleIDToken(ctx, body.Token)
	if err != nil {
		return serrors.New("validate google login id token", err)
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	user, err := api.services.SelectOrInsertGoogleUser(ctx, payload.AccountID, svc.GoogleUserInst{
		Username: payload.Username,
		Country:  model.DefaultCountry,
	})
	if err != nil {
		return serrors.New("upsert verified google user", err)
	}
	return api.handleLoginSession(ctx, w, user)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword" validate:"required,min=11,max=100"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func (api *API) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	var body UpdatePasswordBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}
	if body.NewPassword != body.ConfirmNewPassword {
		return ErrHttpConfirmPassword
	}

	user, err := api.services.VerifyUser(ctx, player.Name, body.Password)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return serrors.New("verify user", err)
	}

	if err := api.services.UpdateUserPassword(ctx, user.ID, body.NewPassword); err != nil {
		return serrors.New("update user password", err)
	}
	slog.InfoContext(ctx, "user has updated password", "user", user)

	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type UpdateUserBody struct {
	NewUsername string `json:"newUsername" validate:"omitempty,min=5,max=100"`
	NewCountry  string `json:"newCountry" validate:"omitempty,countries"`
	NewBio      string `json:"newBio" validate:"omitempty,max=500"`
}

func (api *API) HandleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	var body UpdateUserBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}

	user, err := api.services.UpdateUser(ctx, player.ID, svc.UpdtUserParams{
		Username: body.NewUsername,
		Bio:      body.NewBio,
		Country:  body.NewCountry,
	})
	if err != nil {
		return serrors.New("update user", err)
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

	session, err := api.authenticator.GetSessionOptPlayer(ctx, r)
	if err != nil {
		return err
	}

	sessions, tempSessionID := issueTempSession(session, w)

	slog.InfoContext(ctx, "created sessions", "sessions", sessions)

	if err := api.services.SetSessions(ctx, sessions...); err != nil {
		return serrors.New("set sessions", err)
	}

	writeJSON(w, http.StatusOK, TempSessionResp{SessionID: tempSessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func (api *API) HandleRefreshSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	session, err := api.authenticator.GetSession(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return err
	}

	if err := api.services.UpdateSessionEx(ctx, session.Token, SessionMaxAge); err != nil {
		return serrors.New("update session with expiry", err, "expiry", SessionMaxAge)
	}

	w.Header().Set("Set-Cookie", FmtCookie(session.Token))
	slog.InfoContext(ctx, "refreshed user session", "user", session.Player)

	writeJSON(w, http.StatusOK, RefreshResp{
		Session: &SessionView{
			ID:       session.Player.ID,
			Username: session.Player.Name,
			Country:  session.Player.Country,
			TTLSecs:  SessionMaxAge,
		},
	})
	return nil
}

func (api *API) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return serrors.New("get cookie", err, "cookieKey", CookieKey)
	}
	sessionID := cookie.Value

	if err := api.services.DeleteSession(r.Context(), sessionID); err != nil {
		return serrors.New("logging ext session", err, "sessionID", sessionID)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
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
		return serrors.New("get user by id", err, "player", player)
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
		return serrors.New("get leaderboard page", err, "page", query.Page)
	}
	users, _, err := api.services.GetFullLeaderboardUsers(ctx, query.Mode, leaderboard.RankedUsers)
	if err != nil {
		return serrors.New("get full leaderboard users", err, "rankedUsers", leaderboard.RankedUsers)
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
		return respError("id", fmt.Errorf("failed to parse integer: '%s'", userIDStr))
	}

	fullUser, err := api.services.GetFullUser(ctx, int64(userID), defaultPaginationCount)
	if errors.Is(err, svc.ErrUserNotFound) {
		return ErrHttpNotFoundUser
	} else if err != nil {
		return serrors.New("get full user", err, "userID", userID)
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

	queryCtx := queryParseCtx{Values: r.URL.Query(), RespErr: &BadRequestError{}}

	page := parseDefaultInt(queryCtx, "page", 1)
	name := queryCtx.Values.Get("username")

	slog.InfoContext(ctx, "searching players", "page", page, "name", name)

	users, err := api.services.GetFuzzySearchLeaderboard(ctx, name, int32(page), defaultPaginationCount)
	if err != nil {
		return serrors.New("search users by name", err, "name", name)
	}

	writeJSON(w, http.StatusOK, SearchPlayersResp{UserList: users})
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
		return serrors.New("delete challenge", err)
	}

	var gameID string
	if body.Action == Accept {
		gameID, err = api.services.CreateGame(ctx, deleteResult.FirstColor, deleteResult.Mode, nil)
		if err != nil {
			return serrors.New("create game", err, "deleteResult", deleteResult)
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
	if err != nil {
		return serrors.New("insert challenge", err)
	}

	api.broadcaster.BroadcastChallenge(ctx, ret, pubsub.Async())

	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
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
		challengeList, err = api.services.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: player.ID, ChallengeeID: -1})
	case ReceivedParticipantTarget:
		challengeList, err = api.services.GetChallengesByParticipant(ctx, svc.ChallengeKey{ChallengerID: -1, ChallengeeID: player.ID})
	}
	if err != nil {
		return serrors.New("get challenges", err)
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
		return serrors.New("count user challenges", err)
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
		return serrors.New("create game", err)
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
	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: respMsg})
	return nil
}

type ChessMeta struct {
	GameID      string     `json:"gameid"`
	WhitePlayer model.User `json:"whitePlayer"`
	BlackPlayer model.User `json:"blackPlayer"`
	Mode        string     `json:"mode"`
	Ordering    int64      `json:"ordering"`
}

type ChessMetasResp struct {
	ChessList     []ChessMeta `json:"chessList"`
	SelfChessList []ChessMeta `json:"selfChessList"`
}

func (api *API) HandleGetGameMetadata(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseChessMetasQuery(r.URL.Query())
	if err != nil {
		return err
	}
	session, err := api.authenticator.GetSessionOptPlayer(ctx, r)
	if err != nil {
		return err
	}

	resp, err := api.services.GetGameMetadata(ctx, session, query.AfterOrdering, query.Count)
	if err != nil {
		return serrors.New("get chess metas", err)
	}

	writeJSON(w, http.StatusOK, ChessMetasResp{ChessList: mapChessMetas(resp.AllChessMetas), SelfChessList: mapChessMetas(resp.SelfChessMetas)})
	return nil
}

func (api *API) HandleGetGameChats(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := r.URL.Query().Get("gameId")

	chats, err := api.services.GetChats(ctx, gameID, 100)
	if err != nil {
		return serrors.New("get state chats", err)
	}

	bytes, err := model.MarshalChats(chats)
	if err != nil {
		return serrors.New("failed to marshal chats", err)
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
		return serrors.New("get replay by query", err, "replaysQuery", query)
	}

	writeJSON(w, http.StatusOK, GetReplayResp{Replay: replay})
	return nil
}

type SearchReplaysResp struct {
	ReplayList []model.FullReplay `json:"replayList"`
}

func (api *API) HandleSearchReplays(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseReplaysQuery(r.URL.Query())
	if err != nil {
		return err
	}

	replays, err := api.services.SearchReplaysByQuery(ctx, query)
	if err != nil {
		return serrors.New("get replays by query", err, "query", query)
	}

	writeJSON(w, http.StatusOK, SearchReplaysResp{ReplayList: replays})
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
		return serrors.New("retrieve elo histories buckets with params", err, "params", params)
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
		return respError("replayId", fmt.Errorf("failed to parse integer: '%s'", replayIDStr))
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
		return serrors.New("insert tournament", err)
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
	tournamentKey, err := transformJSON(r, parseTournamentKeyBody)
	if err != nil {
		return err
	}

	tournamentEvent, err := api.services.JoinTournament(ctx, svc.JoinTournamentInst{
		TournamentKey: tournamentKey,
		JoiningUserID: player.ID,
		InsertionTime: time.Now(),
	})
	if err != nil {
		return serrors.New("join tournament by tournament id", err, "tournamentKey", tournamentKey)
	}

	slog.InfoContext(ctx, "participant joined tournament", "joiningID", player.ID, "tournamentKey", tournamentKey)

	go api.services.BroadcastTournamentParticipant(context.WithoutCancel(ctx), player.ID, tournamentEvent)

	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (api *API) HandleBeginCountdownTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	tournamentKey, err := transformJSON(r, parseTournamentKeyBody)
	if err != nil {
		return err
	}

	result, err := api.services.BeginTournamentCountdown(ctx, tournamentKey, player.ID)
	if err != nil {
		return serrors.New("begin tournament countdown by tournament id", err, "tournamentKey", tournamentKey)
	}

	slog.InfoContext(ctx, "successfully started countdown for tournament", "countdownResult", result)

	api.broadcaster.BroadcastTournament(ctx, model.SerializeBeginTournamentCountdown(result.TournamentKey))

	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetTournamentResp model.FullTournament

func (api *API) HandleGetTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	tournamentKey, err := uuid.Parse(r.URL.Query().Get("tournamentKey"))
	if err != nil {
		return respError("tournamentKey", err)
	}

	tournament, err := api.services.GetTournament(ctx, tournamentKey)
	if errors.Is(err, svc.ErrTournamentNotFound) {
		return ErrHttpNotFoundTournament
	} else if err != nil {
		return serrors.New("get tournament by key", err)
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
		return serrors.New("get tournaments", err)
	}
	slog.Info("retrieved tournaments", "tournaments", tournaments)

	writeJSON(w, http.StatusOK, GetTournamentsResp{Tournaments: tournaments})
	return nil
}

func (api *API) HandleLeaveTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	tournamentKey, err := uuid.Parse(r.URL.Query().Get("tournamentKey"))
	if err != nil {
		return respError("tournamentKey", err)
	}

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	didLeave, err := api.services.LeaveTournament(ctx, tournamentKey, player.ID)
	if err != nil {
		return serrors.New("leave tournament", err)
	}
	slog.Info("attempted to leave tournament", "didLeave", didLeave, "tournamentKey", tournamentKey, "player", player)

	writeServiceResp(w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
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
