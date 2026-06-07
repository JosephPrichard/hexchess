package controller

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"math/big"
	"net/http"
	"time"

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

func issueTempSession(session enum.Optional[model.PlayerState], w http.ResponseWriter) ([]svc.SessionInst, string) {
	var sessions []svc.SessionInst
	var tempSessionID string

	if session.IsPresent {
		player := session.Value

		tempSessionID = MakeSessionID()

		sessions = []svc.SessionInst{
			{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge},
		}
	} else {
		player := model.MakeGuestPlayer()

		tempSessionID = MakeSessionID()
		guestSessionID := MakeSessionID()

		sessions = []svc.SessionInst{
			{SessionID: tempSessionID, Player: player, Expiry: TempSessionMaxAge},
			{SessionID: guestSessionID, Player: player, Expiry: SessionMaxAge},
		}

		w.Header().Set("Set-Cookie", FmtCookie(guestSessionID))
	}

	return sessions, tempSessionID
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

	return Session{Player: player, Token: sessionToken}, serrors.Format("get session player", err)
}

func (auth *Authenticator) GetSessionOptPlayer(ctx context.Context, r *http.Request) (enum.Optional[model.PlayerState], error) {
	session, err := auth.GetSession(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return enum.Nothing[model.PlayerState](), nil
	}
	return enum.Just(session.Player), err
}

func (auth *Authenticator) GetSessionPlayer(ctx context.Context, r *http.Request) (model.PlayerState, error) {
	session, err := auth.GetSession(ctx, r)
	if err != nil {
		return model.PlayerState{}, err
	}
	return session.Player, err
}

func (auth *Authenticator) SetSessionPlayer(ctx context.Context, w http.ResponseWriter, player model.PlayerState) (time.Duration, error) {
	sessionToken := MakeSessionID()
	if err := auth.services.SetSessions(ctx, svc.SessionInst{SessionID: sessionToken, Player: player, Expiry: SessionMaxAge}); err != nil {
		return 0, serrors.Format("set session player", err, "playerID", player.ID)
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionToken))
	return SessionMaxAge, nil
}

func FmtCookie(sessionID string) string {
	return fmt.Sprintf("%s=%s; Max-Age=%d; Path=/", CookieKey, sessionID, int(SessionMaxAge.Seconds()))
}
