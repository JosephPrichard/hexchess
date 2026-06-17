package queue

import (
	"fmt"
	"hexchess-svc/model"
)

func FmtStreamKey(streamKey string, partitionKey string) string {
	return fmt.Sprintf("%s:{%s}", streamKey, partitionKey)
}

func FmtGameStreamKey(streamKey string, gameID model.GameID) string {
	return FmtStreamKey(streamKey, string(gameID.Partition()))
}
