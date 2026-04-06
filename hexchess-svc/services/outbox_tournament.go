package svc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
)

type CreateTournamentMatchesEvent struct {
	TournamentKey uuid.UUID
	Matches       []CreateTournamentMatchDTO
}

func pushCreateTournamentMatchesEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, matches []CreateTournamentMatchDTO) error {
	event := CreateTournamentMatchesEvent{TournamentKey: tournamentKey, Matches: matches}
	bytes, err := proto.Marshal(SerializeCreateTournamentMatchesEvent(event))
	if err != nil {
		return fmt.Errorf("marshal create tournament matches event: %w", err)
	}
	if err := querier.InsertOutboxQueue(ctx, sqlc.InsertOutboxQueueParams{
		Type: sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
		Data: bytes,
	}); err != nil {
		return fmt.Errorf("insert create tournament matches into task queue: %w", err)
	}
	return nil
}

func (svc *Services) handleCreateTournamentMatchesEvent(ctx context.Context, bytes []byte) error {
	event, err := UnmarshalCreateTournamentMatchesEvent(bytes)
	if err != nil {
		return fmt.Errorf("unmarshal create tournament matches event: %w", err)
	}

	return nil
}
