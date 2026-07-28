package cache

import (
	"fmt"
	"hexchess-svc/model"
)

func FmtGameKey(gameID model.GameID) string {
	return fmt.Sprintf("game:%s{%c}", gameID, gameID.Partition())
}

func (redis *Redis) FmtLeaderboardZSet(mode string) string {
	return fmt.Sprintf("%s/mode:{%s}", redis.LeaderboardZSet, mode)
}

func (redis *Redis) FmtGameChatsZSet(gameID model.GameID) string {
	return fmt.Sprintf("%s/game:%s{%c}", redis.GameChatsZSet, gameID, gameID.Partition())
}

func FmtSessionKey(sessionID string) string {
	return fmt.Sprintf("session:{%s}", sessionID)
}
