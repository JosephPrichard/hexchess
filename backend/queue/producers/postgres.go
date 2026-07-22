package producers

import (
	"context"
	"hexchess-svc/queue"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type RiverProducer struct {
	riverClient *river.Client[pgx.Tx]
}

func NewRiverProducer(riverClient *river.Client[pgx.Tx]) RiverProducer {
	return RiverProducer{riverClient: riverClient}
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

	job := queue.AdvanceTournamentJob{
		TournamentKey: args.TournamentKey,
		EventID:       uuid.New(),
	}

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
