package producers

import (
	"context"
	"hexchess-svc/database"
	"hexchess-svc/queue"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type NoopRiverClient struct{}

func (_ *NoopRiverClient) InsertTx(_ context.Context, _ pgx.Tx, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return nil, nil
}

func (_ *NoopRiverClient) Insert(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return nil, nil
}

func (_ *NoopRiverClient) Stop(_ context.Context) error {
	return nil
}

type RiverProducer struct {
	riverClient database.RiverClientAPI
}

func NewRiverProducer(riverClient database.RiverClientAPI) *RiverProducer {
	if riverClient == nil {
		slog.Warn("NewRiverProducer: river client is not provided, setting noop producer")
		riverClient = &NoopRiverClient{}
	}
	return &RiverProducer{riverClient: riverClient}
}

type AdvanceTournamentArgs struct {
	TournamentKey uuid.UUID
	ScheduledOn   time.Time
}

func (p *RiverProducer) ProduceAdvanceTournament(ctx context.Context, txn pgx.Tx, args AdvanceTournamentArgs) error {
	opts := &river.InsertOpts{}
	if !args.ScheduledOn.IsZero() {
		opts.ScheduledAt = args.ScheduledOn
	}

	job := queue.AdvanceTournamentJob{TournamentKey: args.TournamentKey, EventID: uuid.New()}

	var err error
	if txn != nil {
		_, err = p.riverClient.InsertTx(ctx, txn, job, opts)
	} else {
		_, err = p.riverClient.Insert(ctx, job, opts)
	}
	if err != nil {
		return serrors.New("produced advance tournament", err, "key", args.TournamentKey, "args", args)
	}

	slog.InfoContext(ctx, "produced advance tournament event", "args", args, "job", job)
	return nil
}

type UpdateUserArgs = queue.UpdateUserJob

func (p *RiverProducer) ProduceUpdateUser(ctx context.Context, txn pgx.Tx, args UpdateUserArgs) error {
	opts := &river.InsertOpts{}

	job := args

	_, err := p.riverClient.InsertTx(ctx, txn, job, opts)
	if err != nil {
		return serrors.New("produced advance tournament", err, "userID", args.ID, "args", args)
	}

	slog.InfoContext(ctx, "produced update user event", "job", job)
	return nil
}
