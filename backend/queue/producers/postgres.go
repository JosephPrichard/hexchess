package producers

import (
	"context"
	"fmt"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/pb"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func PublishAdvanceTournamentEvent(ctx context.Context, querier primarydb.Querier, tournamentKey uuid.UUID, scheduledOn time.Time) error {
	pbEvent := &pb.AdvanceTournamentEvent{
		TournamentKey: tournamentKey.String(),
		EventId:       uuid.NewString(),
	}
	bytes, err := pbEvent.MarshalVT()
	if err != nil {
		return fmt.Errorf("marshal advance tournament event: %w", err)
	}

	if err := querier.InsertQueue(ctx, primarydb.InsertQueueParams{
		Type:        primarydb.QueueTypeEnumTOURNAMENTADVANCEEVENT,
		Data:        bytes,
		CreatedOn:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ScheduledOn: pgtype.Timestamptz{Time: scheduledOn, Valid: !scheduledOn.IsZero()},
	}); err != nil {
		return fmt.Errorf("insert advance tournament event into task queue: %w", err)
	}

	slog.InfoContext(ctx, "pushed advance tournament event", "tournamentKey", tournamentKey, "scheduledOn", scheduledOn)
	return nil
}
