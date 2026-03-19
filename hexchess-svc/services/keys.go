package svc

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

func (svc *Services) getLeaderboardZSet(mode string) string {
	return svc.Redis.LeaderboardZSet + "/mode:" + mode
}

func (svc *Services) getUserGameZSet(id int64) string {
	return svc.Redis.GamesZSet + "/user/" + strconv.Itoa(int(id))
}

func (svc *Services) makeGameKey(gameID string) string {
	return "game/" + gameID
}

func (svc *Services) getGameChatsZSet(gameKey string) string {
	return svc.Redis.GameChatsZSet + "/" + gameKey
}

func (svc *Services) makeSessionKey(sessionID string) string {
	return "session/" + sessionID
}

func makeProfilePicPrefix(userID string) string {
	return fmt.Sprintf("%s/%s", ProfilePicPrefix, userID)
}

func MakeProfileNewPicKey(userID int64) string {
	return fmt.Sprintf("%s/%d/%s", ProfilePicPrefix, userID, uuid.NewString())
}
