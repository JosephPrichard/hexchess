package svc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/db/sqlc"
)

func pushAdvanceTournamentEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID) (err error) {
	bytes, err := MarshalAdvanceTournamentEvent(tournamentKey)
	if err != nil {
		return fmt.Errorf("marshal advance tournament event: %w", err)
	}
	if err := querier.InsertOutboxQueue(ctx, sqlc.InsertOutboxQueueParams{
		Type: sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCE,
		Data: bytes,
	}); err != nil {
		return fmt.Errorf("insert start tournament event into task queue: %w", err)
	}
	return nil
}
