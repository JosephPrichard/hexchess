package svc

import (
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/model"
)

// redis keys

func fmtGameKey(gameID model.GameID) string {
	return fmt.Sprintf("game:%s{%c}", gameID, gameID.Partition())
}

func fmtLeaderboardZSet(redis db.Redis, mode string) string {
	return fmt.Sprintf("%s/mode:{%s}", redis.LeaderboardZSet, mode)
}

func fmtGameChatsZSet(redis db.Redis, gameID model.GameID) string {
	return fmt.Sprintf("%s/game:%s{%c}", redis.GameChatsZSet, gameID, gameID.Partition())
}

func fmtSessionKey(sessionID string) string {
	return fmt.Sprintf("session:{%s}", sessionID)
}
