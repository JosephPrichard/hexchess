package producers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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
