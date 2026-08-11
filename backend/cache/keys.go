package cache

import (
	"fmt"
	"hexchess-svc/model"
)

func FmtGameKey(gameID model.GameID) string {
	return fmt.Sprintf("game:%s{%c}", gameID, gameID.Partition())
}

func FmtLeaderboardZSet(mode string) string {
	return fmt.Sprintf("%s/mode:{%s}", Constants.LeaderboardZSet, mode)
}

func FmtGameChatsZSet(gameID model.GameID) string {
	return fmt.Sprintf("%s/game:%s{%c}", Constants.GameChatsZSet, gameID, gameID.Partition())
}

func FmtGameTimersZSet(partitionKey rune) string {
	return fmt.Sprintf("%s/game:{%c}", Constants.GameTimersZSet, partitionKey)
}

func FmtSessionKey(sessionID string) string {
	return fmt.Sprintf("session:{%s}", sessionID)
}

func FmtStreamKey(streamKey string, partitionKey string) string {
	return fmt.Sprintf("%s:{%s}", streamKey, partitionKey)
}

func FmtGameStreamKey(streamKey string, gameID model.GameID) string {
	return fmt.Sprintf("%s:{%c}", streamKey, gameID.Partition())
}
