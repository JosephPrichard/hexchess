package web

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/ptr"
	"hexchess-svc/services"
	"io"
	"strings"
	"testing"
)

var TestSessionID1 = "testing-session-id-1"
var TestSessionID2 = "testing-session-id-2"
var TestGameID1 = "game1"

func asJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json.Marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}

var TestStates = []svc.ChessState{
	svc.MakeChess(svc.StateSetup{
		ID:         TestGameID1,
		Mode:       svc.ModeCorrespondence1,
		FirstColor: svc.Random,
		Black:      ptr.New(svc.MakePlayer(2, "user2", "us")),
	}),
	svc.MakeChess(svc.StateSetup{ID: "game2", Mode: svc.ModeCorrespondence1, FirstColor: svc.Random}),
	svc.MakeChess(svc.StateSetup{ID: "game3", Mode: svc.ModeCorrespondence1, FirstColor: svc.Random}),
}

func createTestSessions(t *testing.T, s svc.State) {
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-testing-session-1")
	if err := s.SetSessions(ctx,
		svc.SessInst{SessionID: TestSessionID1, Player: svc.MakePlayer(1, "user1", "us"), Expiry: SessionMaxAge},
		svc.SessInst{SessionID: TestSessionID2, Player: svc.MakePlayer(2, "user2", "us"), Expiry: SessionMaxAge},
	); err != nil {
		t.Fatalf("create testing session: %v", err)
	}
}

func createTestChessStates(t *testing.T, s svc.State) {
	ctx := context.WithValue(context.Background(), logutil.Trace, "testing-update-password")
	for _, state := range TestStates {
		if err := s.SetChessState(ctx, state.ID, &state); err != nil {
			t.Fatalf("create testing states: %v", err)
		}
	}
}

func putS3Object(t *testing.T, state *svc.State, key string, b []byte) {
	_, err := state.S3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(state.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(b),
	})
	require.NoError(t, err)
}

func getS3Object(t *testing.T, state *svc.State, key string) string {
	object, err := state.S3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(state.S3Bucket),
		Key:    aws.String(key),
	})
	require.NoError(t, err)
	defer object.Body.Close()

	b, err := io.ReadAll(object.Body)
	require.NoError(t, err)
	return string(b)
}
