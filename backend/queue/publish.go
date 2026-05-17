package queue

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"
	"time"
)

func PublishCreateTournamentMatchesEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, matches []model.TournamentMatchCreation) error {
	bytes, err := proto.Marshal(model.SerializeCreateTournamentMatchesEvent(model.CreateTournamentMatchesEvent{
		TournamentKey: tournamentKey,
		Matches:       matches,
	}))
	if err != nil {
		return fmt.Errorf("marshal create tournament matches event: %w", err)
	}

	if err := querier.InsertOutboxQueue(ctx, sqlc.InsertOutboxQueueParams{
		Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
		Data:      bytes,
		CreatedOn: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("insert create tournament matches event into task queue: %w", err)
	}

	slog.InfoContext(ctx, "pushed create tournament matches event", "tournamentKey", tournamentKey, "matches", matches)

	return nil
}

func PublishScheduledTournamentEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, scheduledOn time.Time) error {
	bytes, err := proto.Marshal(&pb.ScheduledTourmmentEvent{
		TournamentKey: tournamentKey.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal advance tournament event: %w", err)
	}

	if err := querier.InsertOutboxQueue(ctx, sqlc.InsertOutboxQueueParams{
		Type:        sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
		Data:        bytes,
		CreatedOn:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ScheduledOn: pgtype.Timestamptz{Time: scheduledOn, Valid: !scheduledOn.IsZero()},
	}); err != nil {
		return fmt.Errorf("insert advance tournament event into task queue: %w", err)
	}

	slog.InfoContext(ctx, "pushed advance tournament event", "tournamentKey", tournamentKey, "scheduledOn", scheduledOn)

	return nil
}

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

func PublishFinishGameEvent(ctx context.Context, xadder RedisXAdder, finishGameStreamKey string, finishedGame model.FinishedGame) error {
	bytes, err := model.MarshalFinishedGame(finishedGame)
	if err != nil {
		return fmt.Errorf("marshal finish game event: %w", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: finishGameStreamKey,
		Values: map[string]any{"payload": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return fmt.Errorf("xadd finished game event %+v: %w", finishedGame, err)
	}

	slog.InfoContext(ctx, "pushed finished game event", "existingID", msgID, "gameID", finishedGame.GameID)
	return nil
}
