package svc

import (
	"fmt"
	"strconv"
)

func (svc *HexchessServices) getLeaderboardZSet(mode string) string {
	return svc.redis.LeaderboardZSet + "/mode:" + mode
}

func (svc *HexchessServices) getUserGameZSet(id int64) string {
	return svc.redis.GamesZSet + "/user/" + strconv.Itoa(int(id))
}

func (svc *HexchessServices) makeGameKey(gameID string) string {
	return "game/" + gameID
}

func (svc *HexchessServices) getGameChatsZSet(gameKey string) string {
	return svc.redis.GameChatsZSet + "/" + gameKey
}

func (svc *HexchessServices) makeSessionKey(sessionID string) string {
	return "session/" + sessionID
}

func makeProfilePicPrefix(userID string) string {
	return fmt.Sprintf("%s/%s", ProfilePicPrefix, userID)
}

func (svc *HexchessServices) makeProfileNewPicKey(userID int64) string {
	return fmt.Sprintf("%s/%d/%s", ProfilePicPrefix, userID, svc.entropy.MakeID())
}
