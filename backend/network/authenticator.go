package network

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/service"
	"hexchess-svc/utils/serrors"
	"math/big"
	"net/http"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
)

const CookieKey = "session"
const TempSessionMaxAge = time.Minute
const SessionMaxAge = time.Hour * 24 * 30
const SessionIDCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
const SessionIDLength = 64

type SessionView struct {
	ID       int64         `json:"id"`
	Username string        `json:"username"`
	Country  string        `json:"country"`
	TTLSecs  time.Duration `json:"ttlSecs,omitempty"`
}

var testSessionViewCmpOpts = cmpopts.IgnoreFields(SessionView{}, "ID", "TTLSecs")

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

func issueTempSession(sessionPlayer model.PlayerState, w http.ResponseWriter) ([]service.SessionInst, string) {
	var sessions []service.SessionInst
	var tempSessionID string

	if sessionPlayer.Present {
		player := sessionPlayer

		tempSessionID = NewSessionID()

		sessions = []service.SessionInst{
			{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge},
		}
	} else {
		player := model.NewGuestPlayer()

		tempSessionID = NewSessionID()
		guestSessionID := NewSessionID()

		sessions = []service.SessionInst{
			{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge},
			{SessionID: guestSessionID, Player: player, Expiry: SessionMaxAge},
		}

		w.Header().Set("Set-Cookie", FmtCookie(guestSessionID))
	}

	return sessions, tempSessionID
}

type HttpAuthenticator struct {
	services *service.SessionService
}

type Session struct {
	Player model.PlayerState
	Token  string
}

func (auth *HttpAuthenticator) GetSession(ctx context.Context, r *http.Request) (Session, error) {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return Session{}, service.ErrSessionNotFound
	}
	sessionToken := cookie.Value
	player, err := auth.services.GetSession(ctx, sessionToken)
	return Session{Player: player, Token: sessionToken}, serrors.New("get session player", err)
}

func (auth *HttpAuthenticator) GetSessionOptPlayer(ctx context.Context, r *http.Request) (model.PlayerState, error) {
	sess, err := auth.GetSession(ctx, r)
	if errors.Is(err, service.ErrSessionNotFound) {
		return model.PlayerState{}, nil
	}
	return sess.Player, err
}

func (auth *HttpAuthenticator) GetSessionPlayer(ctx context.Context, r *http.Request) (model.PlayerState, error) {
	sess, err := auth.GetSession(ctx, r)
	if err != nil {
		return model.PlayerState{}, err
	}
	return sess.Player, err
}

func (auth *HttpAuthenticator) SetSessionPlayer(ctx context.Context, w http.ResponseWriter, player model.PlayerState) (time.Duration, error) {
	sessionToken := NewSessionID()
	if err := auth.services.SetSessions(ctx, service.SessionInst{
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
