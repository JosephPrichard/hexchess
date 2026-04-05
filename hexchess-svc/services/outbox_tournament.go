package svc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
)

func pushAdvanceTournamentEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, matches []AdvanceTournamentMatchDTO) error {
	bytes, err := proto.Marshal(SerializeAdvanceTournamentEvent(tournamentKey, matches))
	if err != nil {
		return fmt.Errorf("marshal advance tournament event: %w", err)
	}
	if err := querier.InsertOutboxQueue(ctx, sqlc.InsertOutboxQueueParams{
		Type: sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
		Data: bytes,
	}); err != nil {
		return fmt.Errorf("insert advance tournament event into task queue: %w", err)
	}
	return nil
}

func (svc *Services) handleAdvanceTournamentEvent(ctx context.Context, bytes []byte) error {
	tournamentKey, matches, err := UnmarshalAdvanceTournamentEvent(bytes)
	if err != nil {
		return fmt.Errorf("unmarshal advance tournament event: %w", err)
	}

	return nil
}
