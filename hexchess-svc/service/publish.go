package svc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/pb"
	"log/slog"
	"time"
)

type CreateTournamentMatchesEvent struct {
	TournamentKey uuid.UUID
	Matches       []TournamentMatchCreation
}

func sendCreateTourneytMatchesEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, matches []TournamentMatchCreation) error {
	bytes, err := proto.Marshal(SerializeCreateTournamentMatchesEvent(CreateTournamentMatchesEvent{
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

func sendScheduledTournamentEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, scheduledOn time.Time) error {
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

func (svc *HexchessServices) pushFinishGameEvent(ctx context.Context, xadder RedisXAdder, event FinishGameEvent) error {
	streamKey := svc.redis.FinishGameStreamKey

	bytes, err := MarshalFinishGameEvent(event)
	if err != nil {
		return fmt.Errorf("marshal finish game event: %w", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return fmt.Errorf("xadd finished game event: %w", err)
	}

	slog.InfoContext(ctx, "pushed finished game event", "id", msgID, "gameID", event.GameID)
	return nil
}
