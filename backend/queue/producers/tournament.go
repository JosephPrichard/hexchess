package producers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/pb"
	"log/slog"
	"time"
)

func PublishAdvanceTournamentEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, scheduledOn time.Time) error {
	bytes, err := proto.Marshal(&pb.AdvanceTournamentEvent{
		TournamentKey: tournamentKey.String(),
		EventId:       uuid.NewString(),
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
