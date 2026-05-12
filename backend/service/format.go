package svc

import (
	"fmt"
	"hexchess-svc/db"
)

func fmtLeaderboardZSet(redis db.Redis, mode string) string {
	return fmt.Sprintf("%s/mode:%s", redis.LeaderboardZSet, mode)
}

func fmtUserGameZSet(redis db.Redis, id int64) string {
	return fmt.Sprintf("%s/user/%d", redis.GamesZSet, id)
}

func fmtGameKey(redis db.Redis, gameID string) string {
	return fmt.Sprintf("game/%s", gameID)
}

func fmtGameChatsZSet(redis db.Redis, gameKey string) string {
	return fmt.Sprintf("%s/%s", redis.GameChatsZSet, gameKey)
}

func makeSessionKey(sessionID string) string {
	return fmt.Sprintf("session/%s", sessionID)
}

func makeProfilePicPrefix(userID string) string {
	return fmt.Sprintf("%s/%s", ProfilePicPrefix, userID)
}

func makeProfileNewPicKey(userID int64, id string) string {
	return fmt.Sprintf("%s/%d/%s", ProfilePicPrefix, userID, id)
}
