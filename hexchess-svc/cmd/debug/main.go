package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/cmd"
	"hexchess-svc/ext"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"os"
	"strconv"
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

	aws, err := ext.MakeAwsClients(context.Background(), ext.AwsConfig{
		AwsDefaultRegion: awsDefaultRegion,
		AwsSecretKey:     awsSecretKey,
		AwsSecretID:      awsSecretID,
		AwsEndpoint:      awsEndpoint,
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

		s := svc.State{Aws: aws}
		v, err := s.GetMoveReplay(ctx, strconv.Itoa(id))
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
