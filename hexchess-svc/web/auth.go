package web

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"hexchess-svc/services"

	"github.com/google/go-cmp/cmp/cmpopts"
)

const CookieKey = "session"
const TempSessionMaxAge = time.Minute
const SessionMaxAge = time.Hour * 24 * 30

type SessionView struct {
	ID       int64         `json:"id"`
	Username string        `json:"username"`
	Country  string        `json:"country"`
	TTLSecs  time.Duration `json:"ttlSecs,omitempty"`
}

var testSessionViewCmpOpts = cmpopts.IgnoreFields(SessionView{}, "key", "TTLSecs")

func MakeSessionID() string {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const length = 100

	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			panic(fmt.Sprintf("error generating session id: %v", err))
		}
		bytes[i] = characters[n.Int64()]
	}
	return string(bytes)
}

func (server *Server) GetSessionPlayer(ctx context.Context, r *http.Request) (svc.PlayerState, string, error) {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return svc.PlayerState{}, "", svc.ErrSessionNotFound
	}
	sessionID := cookie.Value
	player, err := server.GetSession(ctx, sessionID)
	if err != nil {
		return svc.PlayerState{}, "", err
	}
	return player, sessionID, nil
}

func (server *Server) SetSessionPlayer(ctx context.Context, w http.ResponseWriter, player svc.PlayerState) (time.Duration, error) {
	sessionID := MakeSessionID()
	if err := server.SetSessions(ctx, svc.SessInst{SessionID: sessionID, Player: player, Expiry: SessionMaxAge}); err != nil {
		return 0, err
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))
	return SessionMaxAge, nil
}

func FmtCookie(sessionID string) string {
	return fmt.Sprintf("%s=%s; Max-Age=%d; Path=/", CookieKey, sessionID, int(SessionMaxAge.Seconds()))
}
