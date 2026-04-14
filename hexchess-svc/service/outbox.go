package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/pb"
	"hexchess-svc/util/errutil"
	"log/slog"
	"sync"
	"time"
)

type queuePollHandler struct {
	kind         sqlc.OutboxQueueTypeEnum
	pollInterval time.Duration
	pollCount    int32
	fn           func(ctx context.Context, bytes []byte) error
}

type NonRetryableOutboxError struct {
	Err error
}

func (err NonRetryableOutboxError) Error() string {
	return fmt.Sprintf("non-retryable outbox error: %v", err.Err)
}

func makeOutboxQueueTable(services *HexchessServices) map[sqlc.OutboxQueueTypeEnum]queuePollHandler {
	handlerList := []queuePollHandler{
		{
			kind:         sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
			pollInterval: 5 * time.Second,
			pollCount:    64,
			fn:           services.handleCreateTournamentMatchesEvent,
		},
		{
			kind:         sqlc.OutboxQueueTypeEnumTOURNAMENTSCHEDULEDEVENT,
			pollInterval: 1 * time.Second,
			pollCount:    32,
			fn:           services.handleScheduledTournamentEvent,
		},
	}

	handlerTable := make(map[sqlc.OutboxQueueTypeEnum]queuePollHandler)
	for _, handler := range handlerList {
		handlerTable[handler.kind] = handler
	}

	return handlerTable
}

func pollOutboxQueueEvents(ctx context.Context, querier sqlc.Querier, handler queuePollHandler, getProcessedOn func() time.Time) error {
	// locks events for the duration of the function
	eventRows, err := querier.SelectOutboxQueueByPolling(ctx, sqlc.SelectOutboxQueueByPollingParams{
		Type:  handler.kind,
		Limit: handler.pollCount,
	})
	if err != nil {
		return fmt.Errorf("failed to select %d messages for event kind %s from outbox queue: %w", handler.pollCount, handler.kind, err)
	}

	type eventResult struct {
		eventID int64
		err     error
	}

	var wg sync.WaitGroup
	processedEvents := make([]eventResult, len(eventRows))

	for i, event := range eventRows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := handler.fn(ctx, event.Data)

			processedEvents[i] = eventResult{eventID: event.ID, err: err}
		}()
	}

	wg.Done()

	eventIDsToAck := make([]int64, 0, len(processedEvents))
	var errProcessedEvents []eventResult

	for _, event := range processedEvents {
		// acknowledge the event if there is no error or the error is not retryable
		if event.err == nil || errutil.IsType[NonRetryableOutboxError](event.err) {
			eventIDsToAck = append(eventIDsToAck, event.eventID)
		}
		// always collect all errors to be logged
		if event.err != nil {
			errProcessedEvents = append(errProcessedEvents, event)
		}
	}

	level := slog.LevelInfo
	if len(errProcessedEvents) > 0 {
		level = slog.LevelError
	}
	slog.Log(ctx, level, "handling outbox queue events", "errProcessedEvents", errProcessedEvents, "eventsIDsToAck", eventIDsToAck)

	if err := querier.UpdateOutboxQueueProcessedByID(ctx, sqlc.UpdateOutboxQueueProcessedByIDParams{
		Ids:           eventIDsToAck,
		ProcessedTime: pgtype.Timestamptz{Time: getProcessedOn(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to acknolwedge outbox queue messages %+v: %w", processedEvents, err)
	}

	return nil
}

func StartOutboxQueueConsumers(ctx context.Context, svc *HexchessServices) {
	handlerTable := makeOutboxQueueTable(svc)
	for kind, handler := range handlerTable {
		go PollOutboxQueueLoop(ctx, svc, handler)
		slog.InfoContext(ctx, "started outbox queue consumer for handler", "kind", kind)
	}
}

func PollOutboxQueueLoop(ctx context.Context, svc *HexchessServices, handler queuePollHandler) {
	ticker := time.NewTicker(handler.pollInterval)
	for {
		select {
		case <-ticker.C:
			err := svc.db.ExecTx(ctx, db.Tx{
				// ReadCommitted is used as a basic 'Default` isolation level, the primary purpose of the transaction is atomicity
				// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction, to allow retries
				Isolation: pgx.ReadCommitted,
				QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
					return pollOutboxQueueEvents(ctx, querier, handler, svc.entropy.GetNow)
				},
				RetryCount: 1,
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed to poll outbox queue", "err", err)
			}
		case <-ctx.Done():
			slog.InfoContext(ctx, "exiting outbox queue polling loop")
			return
		}
	}
}

type CreateTournamentMatchesEvent struct {
	TournamentKey uuid.UUID
	Matches       []CreateTournamentMatchDTO
}

func pushCreateTournamentMatchesEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, matches []CreateTournamentMatchDTO, createdOn time.Time) error {
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
		CreatedOn: pgtype.Timestamptz{Time: createdOn, Valid: true},
	}); err != nil {
		return fmt.Errorf("insert create tournament matches event into task queue: %w", err)
	}

	slog.InfoContext(ctx, "pushed create tournament matches event", "tournamentKey", tournamentKey, "matches", matches)

	return nil
}

func pushScheduledTournamentEvent(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, scheduledOn time.Time, createdOn time.Time) error {
	bytes, err := proto.Marshal(&pb.ScheduledTourmmentEvent{
		TournamentKey: tournamentKey.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal scheduled tournament event: %w", err)
	}

	if err := querier.InsertOutboxQueue(ctx, sqlc.InsertOutboxQueueParams{
		Type:        sqlc.OutboxQueueTypeEnumTOURNAMENTSCHEDULEDEVENT,
		Data:        bytes,
		CreatedOn:   pgtype.Timestamptz{Time: createdOn, Valid: true},
		ScheduledOn: pgtype.Timestamptz{Time: scheduledOn, Valid: true},
	}); err != nil {
		return fmt.Errorf("insert scheduled tournament event into task queue: %w", err)
	}

	slog.InfoContext(ctx, "pushed scheduled tournament event", "tournamentKey", tournamentKey, "scheduledOn", scheduledOn)

	return nil
}

func (svc *HexchessServices) handleCreateTournamentMatchesEvent(ctx context.Context, bytes []byte) error {
	event, err := UnmarshalCreateTournamentMatchesEvent(bytes)
	if err != nil {
		return fmt.Errorf("unmarshal create tournament matches event: %w", err)
	}

	userIDs := make([]int64, 0, len(event.Matches)*2)
	for _, match := range event.Matches {
		userIDs = append(userIDs, match.WhiteID, match.BlackID)
	}
	playerDataRows, err := svc.querier.SelectUserPlayerDataByIDs(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("select user player data by ids: %w", err)
	}
	playerDataMap := make(map[int64]sqlc.SelectUserPlayerDataByIDsRow)
	for _, row := range playerDataRows {
		playerDataMap[row.ID] = row
	}

	updtTime := svc.entropy.GetNow()

	pipe := svc.redis.GameStore.TxPipeline()
	for _, match := range event.Matches {
		whitePlayerData, okWhite := playerDataMap[match.WhiteID]
		blackPlayerData, okBlack := playerDataMap[match.BlackID]

		if !okWhite || !okBlack {
			// invariant: white and black should be valid IDs if they have been pushed to the queue
			return fmt.Errorf("missing player data for match: %+v", match)
		}

		state := MakeChessState(StateSetup{
			ID:         match.GameID,
			Mode:       match.GameMode,
			FirstColor: White,
			White:      MakePlayer(match.WhiteID, whitePlayerData.Username, whitePlayerData.Country),
			Black:      MakePlayer(match.BlackID, blackPlayerData.Username, blackPlayerData.Country),
		})

		svc.setChessState(ctx, pipe, match.GameID, state, updtTime)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		// starting tournament games is atomic, job queue will retry until all creates are created in one shot
		return fmt.Errorf("set tournament games [%+v]: %w", event.Matches, err)
	}

	return nil
}

func (svc *HexchessServices) handleScheduledTournamentEvent(ctx context.Context, bytes []byte) error {
	var pbEvent pb.CreateTournamentMatchesEvent
	if err := proto.Unmarshal(bytes, &pbEvent); err != nil {
		return fmt.Errorf("unmarshal create scheduled tournament event: %w", err)
	}

	tournamentKey, err := uuid.Parse(pbEvent.TournamentKey)
	if err != nil {
		return fmt.Errorf("parse tournament key: %w", err)
	}

	// attempt to start the tournament, broadcast the wantResult (successful or otherwise)
	err = svc.StartTournamentTx(ctx, tournamentKey)

	var matchStateError MatchInvariantError
	switch {
	case errors.As(err, &matchStateError):
		// known error: state issue: log, signal failure to subscribers
		slog.ErrorContext(ctx, "failed to start tournament due to match state invariant error", "tournamentKey", tournamentKey, "err", err)

		err := svc.BroadcastTournament(ctx, SerializeTournamentError(tournamentKey, ErrStartTournamentTaskQueue))
		if err != nil {
			return fmt.Errorf("broadcast tournament error: %w", err)
		}

		return NonRetryableOutboxError{matchStateError}
	case err != nil:
		// unknown error: propagate (triggers retry)
		return fmt.Errorf("start tournament %s: %w", tournamentKey, err)
	default:
		// successful: signal success to subscribers
		err := svc.BroadcastTournament(ctx, SerializeStartTournament(tournamentKey))
		if err != nil {
			return fmt.Errorf("broadcast start tournament event: %w", err)
		}
		return nil
	}
}
