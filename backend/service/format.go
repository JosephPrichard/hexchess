package svc

import (
	"fmt"
	"hexchess-svc/db"
)

func fmtUserGameZSet(redis db.Redis, id int64) string {
	return fmt.Sprintf("%s/user/%d", redis.GamesZSet, id)
}

func fmtGameKey(gameID string) string {
	return fmt.Sprintf("game/%s", gameID)
}

func fmtLeaderboardZSet(redis db.Redis, mode string) string {
	return fmt.Sprintf("%s/mode:%s", redis.LeaderboardZSet, mode)
	//return fmt.Sprintf("{%s}%s/mode:%s", mode, redis.LeaderboardZSet, mode)
}

func fmtGameChatsZSet(redis db.Redis, gameKey string) string {
	return fmt.Sprintf("{%s}%s/%s", gameKey, redis.GameChatsZSet, gameKey)
}

func makeSessionKey(sessionID string) string {
	return fmt.Sprintf("{%s}session/%s", sessionID, sessionID)
}

func makeProfilePicPrefix(userID string) string {
	return fmt.Sprintf("%s/%s", ProfilePicPrefix, userID)
}

func makeProfileNewPicKey(userID int64, id string) string {
	return fmt.Sprintf("%s/%d/%s", ProfilePicPrefix, userID, id)
}
