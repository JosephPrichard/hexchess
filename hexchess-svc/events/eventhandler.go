package events

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/domain"
	"hexchess-svc/pb"
	svc "hexchess-svc/service"
	"log/slog"
)

type EventHandler struct {
	Services svc.HexchessAPI
	Querier  sqlc.Querier
}

func (h EventHandler) HandleCreateTournamentMatchesEvent(ctx context.Context, bytes []byte) error {
	event, err := svc.UnmarshalCreateTournamentMatchesEvent(bytes)
	if err != nil {
		return fmt.Errorf("unmarshal create tournament matches event: %w", err)
	}

	slog.InfoContext(ctx, "handling create tournament matches event", "event", event)

	userIDs := make([]int64, 0, len(event.Matches)*2)
	for _, match := range event.Matches {
		userIDs = append(userIDs, match.WhiteID, match.BlackID)
	}
	playerDataRows, err := h.Querier.SelectUserPlayerDataByIDs(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("select user player data by ids: %w", err)
	}
	playerDataMap := make(map[int64]sqlc.SelectUserPlayerDataByIDsRow)
	for _, row := range playerDataRows {
		playerDataMap[row.ID] = row
	}

	var chessStates []svc.ChessState

	for _, match := range event.Matches {
		whitePlayerData, okWhite := playerDataMap[match.WhiteID]
		blackPlayerData, okBlack := playerDataMap[match.BlackID]

		if !okWhite || !okBlack {
			// invariant: white and black should be valid ids if they have been pushed to the queue
			return fmt.Errorf("missing player data for match: %+v", match)
		}

		chessStates = append(chessStates, svc.MakeChessStateVal(svc.StateSetup{
			ID:         match.GameID,
			Mode:       match.GameMode,
			FirstColor: domain.White,
			White:      domain.MakePlayer(match.WhiteID, whitePlayerData.Username, whitePlayerData.Country),
			Black:      domain.MakePlayer(match.BlackID, blackPlayerData.Username, blackPlayerData.Country),
		}))
	}

	// starting tournament games is atomic, job queue will retry until all creates are created in one shot
	if err := h.Services.SetManyChessStates(ctx, chessStates); err != nil {
		return fmt.Errorf("set many chess states: %w", err)
	}

	slog.InfoContext(ctx, "created tournament games")
	return nil
}

func (h EventHandler) HandleAdvanceTournamentEvent(ctx context.Context, bytes []byte) error {
	var pbEvent pb.ScheduledTourmmentEvent
	if err := proto.Unmarshal(bytes, &pbEvent); err != nil {
		return fmt.Errorf("unmarshal create scheduled tournament event: %w", err)
	}
	tournamentKey, err := uuid.Parse(pbEvent.TournamentKey)
	if err != nil {
		return fmt.Errorf("parse tournament Key: %w", err)
	}

	// attempt to advance the tournament, broadcast the result (successful or otherwise)
	err = h.Services.AdvanceTournamentTx(ctx, tournamentKey)

	var matchStateError svc.MatchInvariantError
	switch {
	case errors.As(err, &matchStateError):
		slog.WarnContext(ctx, "failed to start tournament due to match state invariant error", "tournamentKey", tournamentKey, "err", err)

		if err := h.Services.BroadcastTournament(ctx, svc.SerializeTournamentError(tournamentKey, svc.ErrAdvanceTournamentCode)); err != nil {
			return fmt.Errorf("broadcast tournament error: %w", err)
		}
		return NonRetryableOutboxError{Err: matchStateError}
	case err != nil:
		return fmt.Errorf("start tournament %s: %w", tournamentKey, err)
	default:
		if err := h.Services.BroadcastTournament(ctx, svc.SerializeStartTournament(tournamentKey)); err != nil {
			return fmt.Errorf("broadcast start tournament event: %w", err)
		}
		return nil
	}
}
