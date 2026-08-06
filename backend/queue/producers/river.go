package producers

import (
	"context"
	"hexchess-svc/queue"
	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

type RiverClientAPI interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type NoopRiverClient struct{}

func (_ *NoopRiverClient) InsertTx(_ context.Context, _ pgx.Tx, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return nil, nil
}

func (_ *NoopRiverClient) Insert(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return nil, nil
}

func NewRiverClient(ctx context.Context, pgxPool *pgxpool.Pool, config *river.Config) RiverClientAPI {
	riverClient, err := river.NewClient(riverpgxv5.New(pgxPool), config)
	if err != nil {
		logutil.Fatal("create river queue client", err)
	}
	if err := riverClient.Start(ctx); err != nil {
		logutil.Fatal("start river client consumers", err)
	}
	return riverClient
}

type RiverProducer struct {
	riverClient RiverClientAPI
}

func NewRiverProducer(riverClient RiverClientAPI) *RiverProducer {
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
		return serrors.New("publish advance tournament", err)
	}

	slog.InfoContext(ctx, "pushed advance tournament event", "args", args)
	return nil
}
