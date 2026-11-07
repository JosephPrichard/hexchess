package network

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/player"
	"hexchess-svc/session"
	"hexchess-svc/utils/serrors"
	"math/big"
	"net/http"
	"time"
)

const CookieKey = "session"
const TempSessionMaxAge = time.Minute
const SessionMaxAge = time.Hour * 24 * 30
const SessionIDCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
const SessionIDLength = 64

type SessionView struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Country  string `json:"country"`
	TTLSecs  int64  `json:"ttlSecs,omitzero"`
}

func NewSessionID() string {
	bytes := make([]byte, SessionIDLength)
	for i := range SessionIDLength {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(SessionIDCharset))))
		if err != nil {
			panic(fmt.Sprintf("error generating session id: %v", err))
		}
		bytes[i] = SessionIDCharset[n.Int64()]
	}
	return string(bytes)
}

func issueTempSession(sessionPlayer player.PlayerState, w http.ResponseWriter) ([]session.SessionInst, string) {
	var sessions []session.SessionInst
	var tempSessionID string

	if sessionPlayer.Present {
		playerState := sessionPlayer

		tempSessionID = NewSessionID()

		sessions = []session.SessionInst{
			{SessionID: tempSessionID, Player: playerState, Expiry: TempSessionMaxAge},
		}
	} else {
		playerState := player.NewGuestPlayer()

		tempSessionID = NewSessionID()
		guestSessionID := NewSessionID()

		sessions = []session.SessionInst{
			{SessionID: tempSessionID, Player: playerState, Expiry: TempSessionMaxAge},
			{SessionID: guestSessionID, Player: playerState, Expiry: SessionMaxAge},
		}

		w.Header().Set("Set-Cookie", FmtCookie(guestSessionID))
	}

	return sessions, tempSessionID
}

type HttpAuthenticator struct {
	service *session.SessionService
}

type Session struct {
	Player player.PlayerState
	Token  string
}

func (auth *HttpAuthenticator) GetSession(ctx context.Context, r *http.Request) (Session, error) {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return Session{}, session.ErrSessionNotFound
	}
	sessionToken := cookie.Value
	playerState, err := auth.service.GetSession(ctx, sessionToken)
	return Session{Player: playerState, Token: sessionToken}, serrors.New("get session player", err)
}

func (auth *HttpAuthenticator) GetSessionOptPlayer(ctx context.Context, r *http.Request) (player.PlayerState, error) {
	sess, err := auth.GetSession(ctx, r)
	if errors.Is(err, session.ErrSessionNotFound) {
		return player.PlayerState{}, nil
	}
	return sess.Player, err
}

func (auth *HttpAuthenticator) GetSessionPlayer(ctx context.Context, r *http.Request) (player.PlayerState, error) {
	sess, err := auth.GetSession(ctx, r)
	if err != nil {
		return player.PlayerState{}, err
	}
	return sess.Player, err
}

func (auth *HttpAuthenticator) SetSessionPlayer(ctx context.Context, w http.ResponseWriter, player player.PlayerState) (time.Duration, error) {
	sessionToken := NewSessionID()
	if err := auth.service.SetSessions(ctx, session.SessionInst{
		SessionID: sessionToken,
		Player:    player,
		Expiry:    SessionMaxAge,
	}); err != nil {
		return 0, serrors.New("set session player", err, "playerID", player.ID)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionToken))
	return SessionMaxAge, nil
}

func FmtCookie(sessionID string) string {
	return fmt.Sprintf("%s=%s; Max-Age=%d; Path=/", CookieKey, sessionID, int(SessionMaxAge.Seconds()))
}
