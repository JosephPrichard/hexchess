package web

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/internal/enum"
	"hexchess-svc/model"
	"math/big"
	"net/http"
	"time"

	"hexchess-svc/internal/errutil"
	svc "hexchess-svc/service"

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

func MakeSessionID() string {
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

type Authenticator struct {
	services svc.HexchessAPI
}

type Session struct {
	Player model.PlayerState
	Token  string
}

func (auth *Authenticator) GetSession(ctx context.Context, r *http.Request) (Session, error) {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return Session{}, svc.ErrSessionNotFound
	}
	sessionToken := cookie.Value
	player, err := auth.services.GetSession(ctx, sessionToken)

	return Session{Player: player, Token: sessionToken}, errutil.Guardf(err, "get session player")
}

func (auth *Authenticator) GetSessionOptPlayer(ctx context.Context, r *http.Request) (enum.Optional[model.PlayerState], error) {
	session, err := auth.GetSession(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return enum.Nothing[model.PlayerState](), nil
	}
	return enum.Just(session.Player), err
}

func (auth *Authenticator) ExpectSessionPlayer(ctx context.Context, r *http.Request) (model.PlayerState, error) {
	session, err := auth.GetSession(ctx, r)
	if err != nil {
		return model.PlayerState{}, err
	}
	return session.Player, err
}

func (auth *Authenticator) SetSessionPlayer(ctx context.Context, w http.ResponseWriter, player model.PlayerState) (time.Duration, error) {
	sessionToken := MakeSessionID()
	if err := auth.services.SetSessions(ctx, svc.SessionInst{SessionID: sessionToken, Player: player, Expiry: SessionMaxAge}); err != nil {
		return 0, fmt.Errorf("set session player [%d]: %w", player.ID, err)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionToken))
	return SessionMaxAge, nil
}

func FmtCookie(sessionID string) string {
	return fmt.Sprintf("%s=%s; Max-Age=%d; Path=/", CookieKey, sessionID, int(SessionMaxAge.Seconds()))
}
