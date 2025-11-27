package web

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/data"
	"math/big"
	"net/http"
	"time"
)

const CookieKey = "session"
const TempSessionMaxAge = time.Minute
const SessionMaxAge = time.Hour * 24 * 30

type SessionView struct {
	ID       int64         `json:"id"`
	Username string        `json:"username"`
	Country  string        `json:"country"`
	Elo      float64       `json:"elo"`
	TTLSecs  time.Duration `json:"ttlSecs,omitempty"`
}

var SessionViewCmpOpts = cmpopts.IgnoreFields(SessionView{}, "TTLSecs")

func MakeSessionID() (string, error) {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const length = 100

	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			return "", fmt.Errorf("error generating session id: %w", err)
		}
		bytes[i] = characters[n.Int64()]
	}
	return string(bytes), nil
}

func GetSessionPlayer(ctx context.Context, rdb *data.Redis, r *http.Request) (data.PlayerState, string, error) {
	cookie, err := r.Cookie(CookieKey)
	if err != nil {
		return data.PlayerState{}, "", data.ErrSessionNotFound
	}
	sessionID := cookie.Value
	player, err := data.GetSession(ctx, rdb, sessionID)
	if err != nil {
		return data.PlayerState{}, "", err
	}
	return player, sessionID, nil
}

func SetSessionPlayer(ctx context.Context, rdb *data.Redis, w http.ResponseWriter, player data.PlayerState) (time.Duration, error) {
	sessionID, err := MakeSessionID()
	if err != nil {
		return 0, err
	}
	if err := data.SetSession(ctx, rdb, sessionID, player, SessionMaxAge); err != nil {
		return 0, err
	}
	w.Header().Set("Set-Cookie", FmtCookie(sessionID))
	return SessionMaxAge, nil
}

func FmtCookie(sessionID string) string {
	return fmt.Sprintf("%s=%s; Max-Age=%d; Path=/", CookieKey, sessionID, int(SessionMaxAge.Seconds()))
}

func EmptyCookie(sessionID string) string {
	return fmt.Sprintf("%s=%s; Max-Age=%d; Path=/", CookieKey, sessionID, 0)
}
