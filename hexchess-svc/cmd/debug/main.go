package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/cmd"
	"hexchess-svc/egress"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
)

// scripts to easily view any protobuf serialized record in the database in text format for debugging

var mode = flag.String("mode", "move-sequence", "dump mode to execute")
var value = flag.String("value", "1", "the value to fetch")

func main() {
	logutil.InitLoggers(nil)
	cmd.InitEnv()

	awsSecretID := os.Getenv("AWS_SECRET_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_KEY")
	awsDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")

	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	aws, err := egress.MakeAwsClients(context.Background(), egress.AWSConfig{
		AWSDefaultRegion: awsDefaultRegion,
		AWSSecretKey:     awsSecretKey,
		AWSSecretID:      awsSecretID,
		AWSEndpoint:      awsEndpoint,
	})
	if err != nil {
		logutil.FatalErr("load aws config", err)
	}

	switch *mode {
	case "move-sequence":
		id, err := strconv.Atoi(*value)
		if err != nil {
			logutil.FatalErr("parse ID os arg", err)
		}

		services := svc.Services{AWS: aws}
		v, err := services.GetMovesHistory(ctx, id)
		if err != nil {
			logutil.FatalErr("get replay move seq", err)
		}
		var pbMoveReplay pb.MoveHistory
		if err := proto.Unmarshal(v, &pbMoveReplay); err != nil {
			logutil.FatalErr("unmarshal move history", err)
		}

		for _, pbStep := range pbMoveReplay.Steps {
			fmt.Printf("game with move: %+v:\n", pbStep)
		}
	}
}
