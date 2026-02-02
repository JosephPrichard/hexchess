package svc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"hexchess-svc/chess"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const ReplayMoveListPrefix = "replays/moves"

func MakeMoveHistoryKey(replayID string) string {
	return fmt.Sprintf("%s/%s", ReplayMoveListPrefix, replayID)
}

func ParseMoveHistoryKey(key string) (int64, error) {
	tokens := strings.Split(key, "/")
	if len(tokens) != 3 {
		return 0, fmt.Errorf("invalid history key, wrong number of tokens: %s", key)
	}
	replayID, err := strconv.ParseInt(tokens[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("history key replayID is not a valid integer: %s: %w", key, err)
	}
	return replayID, nil
}

func (svc *Services) PutMovesHistory(ctx context.Context, replayID int64, initialBoard chess.Board, moves []chess.HistMove) error {
	slog.InfoContext(ctx, "uploading move history to s3", "replayID", replayID)
	start := time.Now()

	moveHistBytes, err := chess.MarshalMoveHistory(initialBoard, moves)
	if err != nil {
		return fmt.Errorf("marshal move history to s3: %w", err)
	}
	key := MakeMoveHistoryKey(strconv.Itoa(int(replayID)))

	if _, err := svc.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(svc.S3ReplayBucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(moveHistBytes),
	}); err != nil {
		return fmt.Errorf("put move history=%s: to s3 Bucket: %s: %w", key, svc.S3ReplayBucket, err)
	}

	slog.InfoContext(ctx, "finished uploading move history to s3", "key", key, "replayID", replayID, "took", time.Since(start))
	return nil
}

func (svc *Services) GetMovesHistory(ctx context.Context, replayID string) ([]byte, error) {
	key := MakeMoveHistoryKey(replayID)

	object, err := svc.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(svc.S3ReplayBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get move history by key=%s from s3: %w", key, err)
	}
	defer object.Body.Close()

	bReplay, err := io.ReadAll(object.Body)
	if err != nil {
		return nil, fmt.Errorf("read move history bytes with  key=%s: %w", key, err)
	}

	slog.InfoContext(ctx, "retrieved move history from s3", "key", key, "size", fmt.Sprintf("%dKB", len(bReplay)/1000))
	return bReplay, nil
}
