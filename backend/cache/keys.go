package cache

import (
	"fmt"
	"hexchess-svc/model"
)

func FmtGameKey(gameID model.GameID) string {
	return fmt.Sprintf("game:%s{%c}", gameID, gameID.Partition())
}

func FmtLeaderboardZSet(mode string) string {
	return fmt.Sprintf("%s/mode:{%s}", Contants.LeaderboardZSet, mode)
}

func FmtGameChatsZSet(gameID model.GameID) string {
	return fmt.Sprintf("%s/game:%s{%c}", Contants.GameChatsZSet, gameID, gameID.Partition())
}

func FmtSessionKey(sessionID string) string {
	return fmt.Sprintf("session:{%s}", sessionID)
}
