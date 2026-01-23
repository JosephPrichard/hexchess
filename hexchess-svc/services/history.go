package svc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"io"
	"log/slog"
	"strconv"
	"time"
)

const ReplayMoveListPrefix = "replays/moves"

func MakeReplayMoveListKey(replayID string) string {
	return fmt.Sprintf("%s/%s", ReplayMoveListPrefix, replayID)
}

func (s *State) PutReplayMoveSeq(ctx context.Context, replayID int64, initialBoard chess.Board, moves []chess.HistMove) error {
	slog.InfoContext(ctx, "uploading move history to s3", "replayID", replayID)
	start := time.Now()

	moveHistBytes, err := chess.MarshalMoveHistory(initialBoard, moves)
	if err != nil {
		return fmt.Errorf("marshal move history to s3: %w", err)
	}
	key := MakeReplayMoveListKey(strconv.Itoa(int(replayID)))

	if _, err := s.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.S3ReplayBucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(moveHistBytes),
	}); err != nil {
		return fmt.Errorf("put move history=%s: to s3 bucket: %s: %w", key, s.S3ReplayBucket, err)
	}

	slog.InfoContext(ctx, "finished uploading move history to s3", "key", key, "replayID", replayID, "took", time.Since(start))
	return nil
}

func (s *State) GetMoveReplay(ctx context.Context, replayID string) ([]byte, error) {
	key := MakeReplayMoveListKey(replayID)

	object, err := s.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.S3ReplayBucket),
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

	var pbMoveHist pb.MoveHistory
	if err := proto.Unmarshal(bReplay, &pbMoveHist); err != nil {
		return nil, fmt.Errorf("unmarshal move history with  key=%s: %w", key, err)
	}
	bResp, err := chess.MarshalMoveReplay(&pbMoveHist)
	if err != nil {
		return nil, fmt.Errorf("marshal move replay with key=%s move seq : %w", key, err)
	}
	return bResp, nil
}
