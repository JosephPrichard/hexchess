package network

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/challenges"
	"hexchess-svc/game"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/persona"
	"hexchess-svc/player"
	"hexchess-svc/replay"
	"hexchess-svc/session"
	"hexchess-svc/tournament"
	"hexchess-svc/user"
	"hexchess-svc/userdata"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"uuid"
)

type RegisterBody struct {
	Username        string `json:"username" validate:"required,min=5,max=35"`
	Password        string `json:"password" validate:"required,min=11,max=100"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (server *HttpServer) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body RegisterBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}
	if body.Password != body.ConfirmPassword {
		return ErrHttpConfirmPassword
	}

	u, err := server.service.InsertUser(ctx, user.UserInst{
		Username: body.Username,
		Password: body.Password,
		Country:  userdata.DefaultCountry,
		JoinedOn: time.Now(),
	})
	if errors.Is(err, user.ErrTakenUsername) {
		return ErrHttpDuplicateUsername
	} else if err != nil {
		return serrors.New("insert user", err)
	}

	ttl, err := server.authenticator.SetSessionPlayer(ctx, w, player.NewPlayer(u.ID, u.Username, u.Country))
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "registered user", "user", u)

	writeJSON(ctx, w, http.StatusOK, SessionView{
		ID:       u.ID,
		Username: u.Username,
		Country:  u.Country,
		TTLSecs:  int64(ttl.Seconds()),
	})
	return nil
}

func (server *HttpServer) handleLoginSession(ctx context.Context, w http.ResponseWriter, verifiedUser user.VerifiedUser) error {
	ttlDuration, err := server.authenticator.SetSessionPlayer(ctx, w, player.NewPlayer(verifiedUser.ID, verifiedUser.Username, verifiedUser.Country))
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "user has logged in", "user", verifiedUser)

	writeJSON(ctx, w, http.StatusOK, SessionView{
		ID:       verifiedUser.ID,
		Username: verifiedUser.Username,
		Country:  verifiedUser.Country,
		TTLSecs:  int64(ttlDuration.Seconds()),
	})
	return nil
}

type LoginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (server *HttpServer) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body LoginBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}
	u, err := server.service.VerifyUser(ctx, body.Username, body.Password)
	if err != nil {
		return serrors.New("verify user", err, "username", body.Username)
	}

	return server.handleLoginSession(ctx, w, u)
}

type GoogleLoginBody struct {
	Token string `json:"token"`
}

func (server *HttpServer) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body GoogleLoginBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}

	payload, err := server.service.ValidateGoogleIDToken(ctx, body.Token)
	if err != nil {
		return serrors.New("validate google login id token", err)
	}
	slog.InfoContext(ctx, "validated google account id token", "googleAccountID", payload.AccountID)

	u, err := server.service.SelectOrInsertGoogleUser(ctx, payload.AccountID, user.GoogleUserInst{
		Username: payload.Username,
		Country:  userdata.DefaultCountry,
	})
	if err != nil {
		return serrors.New("upsert verified google user", err)
	}
	return server.handleLoginSession(ctx, w, u)
}

type UpdatePasswordBody struct {
	Password           string `json:"password"`
	NewPassword        string `json:"newPassword" validate:"required,min=11,max=100"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

func (server *HttpServer) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
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

	u, err := server.service.VerifyUser(ctx, playerState.Name, body.Password)
	if errors.Is(err, user.ErrUserNotFound) {
		return ErrHttpInvalidLogin
	} else if err != nil {
		return serrors.New("verify user", err)
	}

	if err := server.service.UpdateUserPassword(ctx, u.ID, body.NewPassword); err != nil {
		return serrors.New("update user password", err)
	}
	slog.InfoContext(ctx, "user has updated password", "user", u)

	writeServiceResp(ctx, w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type UpdateUserBody struct {
	NewUsername string `json:"newUsername" validate:"omitempty,min=5,max=100"`
	NewCountry  string `json:"newCountry" validate:"omitempty,countries"`
	NewBio      string `json:"newBio" validate:"omitempty,max=500"`
}

func (server *HttpServer) HandleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	var body UpdateUserBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}

	u, err := server.service.UpdateUser(ctx, playerState.ID, user.UpdtUserParams{
		Username: body.NewUsername,
		Bio:      body.NewBio,
		Country:  body.NewCountry,
	})
	if err != nil {
		return serrors.New("update user", err)
	}
	slog.InfoContext(ctx, "user has updated username", "user", playerState)

	writeJSON(ctx, w, http.StatusOK, SessionView{
		ID:       u.ID,
		Username: u.Username,
		Country:  u.Country,
	})
	return nil
}

type TempSessionResp struct {
	SessionID string `json:"sessionId"`
}

func (server *HttpServer) HandleCreateTempSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	authSession, err := server.authenticator.GetSessionOptPlayer(ctx, r)
	if err != nil {
		return err
	}

	sessions, tempSessionID := issueTempSession(authSession, w)
	slog.InfoContext(ctx, "created sessions", "sessions", sessions, "tempSessionID", tempSessionID)

	if err := server.service.SetSessions(ctx, sessions...); err != nil {
		return serrors.New("set sessions", err)
	}

	writeJSON(ctx, w, http.StatusOK, TempSessionResp{SessionID: tempSessionID})
	return nil
}

type RefreshResp struct {
	Session *SessionView `json:"session,omitempty"`
}

func (server *HttpServer) HandleRefreshSession(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	authSession, err := server.authenticator.GetSession(ctx, r)
	if errors.Is(err, session.ErrSessionNotFound) {
		writeJSON(ctx, w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return err
	}

	if err := server.service.UpdateSessionEx(ctx, authSession.Token, SessionMaxAge); err != nil {
		return serrors.New("update session with expiry", err, "expiry", SessionMaxAge)
	}

	w.Header().Set("Set-Cookie", FmtCookie(authSession.Token))
	slog.InfoContext(ctx, "refreshed user session", "user", authSession.Player)

	writeJSON(ctx, w, http.StatusOK, RefreshResp{
		Session: &SessionView{
			ID:       authSession.Player.ID,
			Username: authSession.Player.Name,
			Country:  authSession.Player.Country,
			TTLSecs:  int64(SessionMaxAge.Seconds()),
		},
	})
	return nil
}

func (server *HttpServer) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return serrors.New("get cookie", err, "cookieKey", CookieKey)
	}
	sessionID := cookie.Value

	if err := server.service.DeleteSession(r.Context(), sessionID); err != nil {
		return serrors.New("logging ext session", err, "sessionID", sessionID)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))

	writeServiceResp(r.Context(), w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (server *HttpServer) HandleGetSelf(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if errors.Is(err, session.ErrSessionNotFound) {
		writeJSON(ctx, w, http.StatusOK, RefreshResp{Session: nil})
		return nil
	} else if err != nil {
		return err
	}

	u, err := server.service.GetUserByID(ctx, playerState.ID)
	if err != nil {
		return serrors.New("get user by id", err, "playerState", playerState)
	}
	slog.InfoContext(ctx, "retrieved user", "user", u)

	writeJSON(ctx, w, http.StatusOK, u)
	return nil
}

type LeaderboardQuery struct {
	Page int
	Mode gamedata.GameMode
}

type LeaderboardResp struct {
	TotalPages int            `json:"totalPages"`
	UserList   []user.LbdUser `json:"userList"`
}

func (server *HttpServer) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseLeaderboardQuery(r.URL.Query())
	if err != nil {
		return err
	}

	leaderboard, err := server.service.GetLeaderboardPage(ctx, query.Mode, int64(query.Page), defaultPaginationCount)
	if err != nil {
		return serrors.New("get leaderboard page", err, "page", query.Page)
	}
	users, _, err := server.service.GetFullLeaderboardUsers(ctx, query.Mode, leaderboard.RankedUsers)
	if err != nil {
		return serrors.New("get full leaderboard users", err, "rankedUsers", leaderboard.RankedUsers)
	}
	slog.InfoContext(ctx, "retrieved leaderboard", "users", users)

	if users == nil {
		users = []user.LbdUser{}
	}
	writeJSON(ctx, w, http.StatusOK, LeaderboardResp{TotalPages: leaderboard.PageCount, UserList: users})
	return nil
}

type GetPersonaResp struct {
	persona.Persona
}

func (server *HttpServer) HandleGetPersona(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	userIDStr := r.URL.Query().Get("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return respError("id", fmt.Errorf("failed to parse integer: '%s'", userIDStr))
	}

	personaData, err := server.service.GetPersona(ctx, int64(userID), defaultPaginationCount)
	if errors.Is(err, user.ErrUserNotFound) {
		return ErrHttpNotFoundUser
	} else if err != nil {
		return serrors.New("get full user", err, "userID", userID)
	}

	playerStatesResp := GetPersonaResp{Persona: personaData}

	slog.InfoContext(ctx, "retrieved user with replays", "fullUser", playerStatesResp)

	writeJSON(ctx, w, http.StatusOK, playerStatesResp)
	return nil
}

type SearchPlayersResp struct {
	UserList []user.LbdUser `json:"userList"`
}

func (server *HttpServer) HandleSearchPlayers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	queryCtx := queryParseCtx{Values: r.URL.Query(), RespErr: &BadRequestError{}}

	page := parseDefaultInt(queryCtx, "page", 1)
	searchInput := queryCtx.Values.Get("searchInput")

	slog.InfoContext(ctx, "searching players", "page", page, "searchInput", searchInput)

	users, err := server.service.SearchUsers(ctx, searchInput, int32(page), defaultPaginationCount)
	if err != nil {
		return serrors.New("search users by name", err, "searchInput", searchInput)
	}

	writeJSON(ctx, w, http.StatusOK, SearchPlayersResp{UserList: users})
	return nil
}

type UpdateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeID"`
	ChallengerID int64  `json:"challengerID"`
	Action       string `json:"action"`
}

type UpdateChallengeResp struct {
	GameID gameid.Identifier `json:"challengeID"`
}

func (server *HttpServer) HandleUpdateChallenge(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	body, err := mapJSON(r, parseUpdateChallengeBody)
	if err != nil {
		return err
	}
	if playerState.ID != body.TargetID {
		return ErrHttpUpdateChallenge
	}

	deleteResult, err := server.service.DeleteChallenge(ctx, body.ChallengerID, body.ChallengeeID)
	if err != nil {
		return serrors.New("delete challenge", err)
	}

	var gameID gameid.Identifier
	if body.Action == Accept {
		gameID, err = server.service.CreateGame(ctx, game.ChessStateArgs{Mode: deleteResult.Mode, FirstColor: deleteResult.FirstColor})
		if err != nil {
			return serrors.New("create game", err, "deleteResult", deleteResult)
		}
	}

	writeJSON(ctx, w, http.StatusOK, UpdateChallengeResp{GameID: gameID})
	return nil
}

type CreateChallengeBody struct {
	ChallengeeID int64  `json:"challengeeID"`
	StartColor   string `json:"startColor"`
	Mode         string `json:"mode"`
}

func (server *HttpServer) HandleCreateChallenge(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	body, err := mapJSON(r, parseCreateChallengeBody)
	if err != nil {
		return err
	}

	challenge, err := server.service.InsertChallenge(ctx, challenges.ChallengeInst{
		ChallengerID: playerState.ID,
		ChallengeeID: body.ChallengeeID,
		Mode:         body.Mode,
		StartColor:   body.StartColor,
		MadeOn:       time.Now(),
	})
	if err != nil {
		return serrors.New("insert challenge", err)
	}

	server.broadcaster.BroadcastUserMessage(ctx, challenges.UserMessage{
		Kind:      challenges.ChallengeKind,
		Challenge: challenge,
	})

	writeServiceResp(ctx, w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetChallengesResp struct {
	ChallengeList []challenges.Challenge `json:"challengeList"`
}

const (
	SentParticipantsTarget    = "sent"
	ReceivedParticipantTarget = "received"
)

func (server *HttpServer) HandleGetChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	participants := r.URL.Query().Get("participants")

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}

	var challengeList []challenges.Challenge

	switch participants {
	case SentParticipantsTarget:
		challengeList, err = server.service.GetChallengesByParticipant(ctx, challenges.Key{ChallengerID: playerState.ID, ChallengeeID: -1})
	case ReceivedParticipantTarget:
		challengeList, err = server.service.GetChallengesByParticipant(ctx, challenges.Key{ChallengerID: -1, ChallengeeID: playerState.ID})
	}
	if err != nil {
		return serrors.New("get challenges", err)
	}

	slog.InfoContext(ctx, "retrieved challenges", "challengeList", challengeList)
	writeJSON(ctx, w, http.StatusOK, GetChallengesResp{ChallengeList: challengeList})
	return nil
}

type CountChallengesResp struct {
	Count int64 `json:"count"`
}

func (server *HttpServer) HandleCountUserChallenges(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	count, err := server.service.CountUserChallenges(ctx, playerState.ID)
	if err != nil {
		return serrors.New("count user challenges", err)
	}

	writeJSON(ctx, w, http.StatusOK, CountChallengesResp{Count: count})
	return nil
}

type CreateGameBody struct {
	FirstColor string `json:"firstColor"`
	Mode       string `json:"mode"`
	InitialFEN string `json:"initialFen"`
}

type CreateGameResp struct {
	GameID gameid.Identifier `json:"gameId"`
}

func (server *HttpServer) HandleCreateGame(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	body, err := mapJSON(r, parseCreateGameBody)
	if err != nil {
		return err
	}

	gameID, err := server.service.CreateGame(ctx, game.ChessStateArgs{Mode: body.Mode, FirstColor: body.FirstColor, InitialBoard: &body.InitialBoard})
	if err != nil {
		return serrors.New("create game", err)
	}
	slog.InfoContext(ctx, "created game", "gameID", gameID)

	writeJSON(ctx, w, http.StatusOK, CreateGameResp{GameID: gameID})
	return nil
}

const (
	GameExists    = "GAME_EXISTS"
	GameNotExists = "GAME_NOT_EXISTS"
)

func (server *HttpServer) HandleGameExistence(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	gameIDStr := r.URL.Query().Get("gameId")
	gameID := gameid.Identifier(gameIDStr)

	exists := server.service.IsGameAccessible(ctx, gameID)

	resp := ServiceResp{Status: http.StatusOK}
	if exists {
		resp.Message = GameExists
	} else {
		resp.Message = GameNotExists
	}
	writeServiceResp(ctx, w, resp)
	return nil
}

type ChessMeta struct {
	GameID      gameid.Identifier     `json:"gameId"`
	WhitePlayer opt.Option[user.User] `json:"whitePlayer"`
	BlackPlayer opt.Option[user.User] `json:"blackPlayer"`
	Mode        string                `json:"mode"`
	Ordering    int64                 `json:"ordering"`
}

type ChessMetasResp struct {
	ChessList     []ChessMeta `json:"chessList"`
	SelfChessList []ChessMeta `json:"selfChessList"`
}

func (server *HttpServer) HandleGetGameMetadata(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseChessMetasQuery(r.URL.Query())
	if err != nil {
		return err
	}
	authSession, err := server.authenticator.GetSessionOptPlayer(ctx, r)
	if err != nil {
		return err
	}
	resp, err := server.service.GetGameMetadata(ctx, authSession, query.AfterOrdering, query.Count)
	if err != nil {
		return serrors.New("get chess metas", err)
	}

	writeJSON(ctx, w, http.StatusOK, ChessMetasResp{ChessList: mapChessMetas(resp.ChessMetas), SelfChessList: mapChessMetas(resp.UserChessMetas)})
	return nil
}

func (server *HttpServer) HandleGetGameChats(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameIDStr := r.URL.Query().Get("gameId")
	gameID := gameid.Identifier(gameIDStr)

	chats, err := server.service.GetChats(ctx, gameID, 100)
	if err != nil {
		return serrors.New("get state chats", err)
	}

	bytes, err := game.MarshalChats(chats)
	if err != nil {
		return serrors.New("failed to marshal chats", err)
	}

	writeBytes(w, http.StatusOK, bytes)
	return nil
}

type GetReplayResp struct {
	Replay replay.FullReplay `json:"replay"`
}

func (server *HttpServer) HandleGetReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseReplayQueryBody(r.URL.Query())
	if err != nil {
		return err
	}

	var fullReplay replay.FullReplay
	if query.HasGameID {
		fullReplay, err = server.service.GetReplayByGameID(ctx, query.GameID)
	} else {
		fullReplay, err = server.service.GetReplay(ctx, query.ReplayID)
	}
	if errors.Is(err, replay.ErrNoReplay) {
		return ErrHttpNotFoundReplay
	} else if err != nil {
		return serrors.New("get replay by query", err, "replaysQuery", query)
	}

	writeJSON(ctx, w, http.StatusOK, GetReplayResp{Replay: fullReplay})
	return nil
}

type SearchReplaysResp struct {
	ReplayList []replay.FullReplay `json:"replayList"`
}

func (server *HttpServer) HandleSearchReplays(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseReplaysQuery(r.URL.Query())
	if err != nil {
		return err
	}
	replays, err := server.service.SearchReplaysByQuery(ctx, query)
	if err != nil {
		return serrors.New("get replays by query", err, "query", query)
	}

	writeJSON(ctx, w, http.StatusOK, SearchReplaysResp{ReplayList: replays})
	return nil
}

type EloHistoriesResp struct {
	Buckets replay.EloHistoryBuckets `json:"buckets"`
}

var GetEloHistoriesCacheControl = fmt.Sprintf("public, max-age=%f", replay.ShortBucketDuration.Seconds())

func (server *HttpServer) HandleGetEloHistories(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseEloHistoriesQuery(r.URL.Query())
	if err != nil {
		return err
	}

	params := replay.EloHistoriesParams{
		UserID:    int64(query.UserID),
		Months:    query.Months,
		TimeUntil: time.Now(),
	}
	eloBucketsResp, err := server.service.RetrieveEloHistoryBuckets(ctx, params)
	if err != nil {
		return serrors.New("retrieve elo histories buckets with params", err, "params", params)
	}
	writeJSON(ctx, w, http.StatusOK, EloHistoriesResp{Buckets: eloBucketsResp.EloHistories})

	//w.Header().Set("Cache-Control", GetEloHistoriesCacheControl)
	return nil
}

func (server *HttpServer) HandleGetMoveReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	replayIDStr := r.URL.Query().Get("replayId")
	replayID, err := strconv.Atoi(replayIDStr)
	if err != nil {
		return respError("replayId", fmt.Errorf("failed to parse integer: '%s'", replayIDStr))
	}

	bytes, err := server.service.GetMovesHistory(ctx, replayID)
	if err != nil {
		return err
	}

	writeBytes(w, http.StatusOK, bytes)
	// aggressive cache control because this resource does not change, but the algorithm we are using to parse it might if requirements change.
	//w.Header().Set("Cache-Control", "public, max-age=86400")
	return nil
}

type CreateTournamentBody struct {
	Name            string `json:"name"`
	Mode            string `json:"mode"`
	Ruleset         string `json:"ruleset"`
	Rounds          int32  `json:"rounds"`
	CountdownMillis int64  `json:"countdownMillis"`
}

type CreateTournamentResp struct {
	TournamentKey string `json:"tournamentKey"`
}

func (server *HttpServer) HandleCreateTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	body, err := mapJSON(r, parseCreateTournamentBody)
	if err != nil {
		return err
	}

	tournamentKey := uuid.NewV7()
	tournamentID, err := server.service.CreateTournament(ctx, tournament.TournamentInst{
		Key:             tournamentKey,
		Name:            body.Name,
		Rounds:          body.Rounds,
		Mode:            body.Mode,
		Ruleset:         body.Ruleset,
		CountdownMillis: body.CountdownMillis,
		CreatedOn:       time.Now(),
		CreatedBy:       playerState.ID,
	})
	if errors.Is(err, tournament.ErrInvalidRounds) {
		return ErrHttpInvalidRounds
	} else if err != nil {
		return serrors.New("insert tournament", err)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentID", tournamentID, "key", tournamentKey)

	writeJSON(ctx, w, http.StatusCreated, CreateTournamentResp{TournamentKey: tournamentKey.String()})
	return nil
}

type TournamentKeyBody struct {
	TournamentKey string `json:"tournamentKey"`
}

func (server *HttpServer) HandleJoinTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	tournamentKey, err := mapJSON(r, parseTournamentKeyBody)
	if err != nil {
		return err
	}

	tournamentEvent, err := server.service.JoinTournament(ctx, tournament.JoinTournamentInst{
		TournamentKey: tournamentKey,
		JoiningUserID: playerState.ID,
		InsertionTime: time.Now(),
	})
	if err != nil {
		return serrors.New("join tournament by tournament id", err, "key", tournamentKey)
	}

	slog.InfoContext(ctx, "participant joined tournament", "joiningID", playerState.ID, "key", tournamentKey)

	server.dispatcher.Go(func() {
		server.service.NotifyTournamentParticipant(context.WithoutCancel(ctx), playerState.ID, tournamentEvent)
	})

	writeServiceResp(ctx, w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

func (server *HttpServer) HandleBeginCountdownTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	tournamentKey, err := mapJSON(r, parseTournamentKeyBody)
	if err != nil {
		return err
	}

	result, err := server.service.BeginTournamentCountdown(ctx, tournamentKey, playerState.ID)
	if err != nil {
		return serrors.New("begin tournament countdown by tournament id", err, "key", tournamentKey)
	}

	slog.InfoContext(ctx, "successfully started countdown for tournament", "countdownResult", result)

	server.broadcaster.BroadcastTournament(ctx, tournament.TournamentOutput{
		Key:  result.TournamentKey.String(),
		Kind: tournament.TournamentCountdownKind,
	})

	writeServiceResp(ctx, w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetTournamentResp tournament.FullTournament

func (server *HttpServer) HandleGetTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	tournamentKey, err := uuid.Parse(r.URL.Query().Get("tournamentKey"))
	if err != nil {
		return respError("tournamentKey", err)
	}

	tourney, err := server.service.GetTournament(ctx, tournamentKey)
	if errors.Is(err, tournament.ErrTournamentNotFound) {
		return ErrHttpNotFoundTournament
	} else if err != nil {
		return serrors.New("get tournament by key", err)
	}

	slog.Info("retrieved full tournament", "tournament", tourney)

	writeJSON(ctx, w, http.StatusOK, GetTournamentResp(tourney))
	return nil
}

type GetTournamentsResp struct {
	Tournaments []tournament.Tournament `json:"tournaments"`
}

func (server *HttpServer) HandleGetTournaments(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	query, err := parseTournamentsQuery(r.URL.Query())
	if err != nil {
		return err
	}

	tournaments, err := server.service.GetTournaments(ctx, query.UserID, query.AfterID, defaultPaginationCount)
	if err != nil {
		return serrors.New("get tournaments", err)
	}
	slog.Info("retrieved tournaments", "tournaments", tournaments)

	writeJSON(ctx, w, http.StatusOK, GetTournamentsResp{Tournaments: tournaments})
	return nil
}

func (server *HttpServer) HandleLeaveTournament(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	tournamentKey, err := uuid.Parse(r.URL.Query().Get("tournamentKey"))
	if err != nil {
		return respError("tournamentKey", err)
	}

	playerState, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	didLeave, err := server.service.LeaveTournament(ctx, tournamentKey, playerState.ID)
	if err != nil {
		return serrors.New("leave tournament", err)
	}
	slog.Info("attempted to leave tournament", "didLeave", didLeave, "key", tournamentKey, "playerState", playerState)

	writeServiceResp(ctx, w, ServiceResp{Status: http.StatusOK, Message: "SUCCESS"})
	return nil
}

type GetUserActivityResp struct {
	IsUserActive bool `json:"isUserActive"`
}

func (server *HttpServer) HandleUserActivityCheck(w http.ResponseWriter, r *http.Request) error {
	userID := r.URL.Query().Get("userId")

	isActive := server.service.IsActiveUser(r.Context(), userID)

	writeJSON(r.Context(), w, http.StatusOK, GetUserActivityResp{IsUserActive: isActive})
	//w.Header().Set("Cache-Control", "public, max-age=30")
	return nil
}
