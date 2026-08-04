package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/db/mutator"
	"hexchess-svc/db/query"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/perf"

	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

type ChessMetaService struct {
	mutator     mutator.Querier
	querier     query.Querier
	entropy     entropy.Generator
	broadcaster pubsub.Broadcaster
}

func NewChessMetaService(
	mutator mutator.Querier,
	querier query.Querier,
	entropy entropy.Generator,
	broadcaster pubsub.Broadcaster,
) *ChessMetaService {
	return &ChessMetaService{mutator: mutator, querier: querier, entropy: entropy, broadcaster: broadcaster}
}

func (services *ChessMetaService) UpdateGameMetadata(ctx context.Context, updt model.GameMetadataUpdt) error {
	defer perf.WithContext(ctx).Log()

	updtResult, err := services.mutator.UpdateGameMeta(ctx, mutator.UpdateGameMetaParams{
		GameID:    updt.GameID.String(),
		WhiteID:   pgtype.Int8{Int64: updt.WhitePlayer.Value, Valid: updt.WhitePlayer.Present},
		BlackID:   pgtype.Int8{Int64: updt.BlackPlayer.Value, Valid: updt.BlackPlayer.Present},
		Mode:      mutator.ModeEnum(updt.Mode.String()),
		UpdatedOn: pgtype.Timestamptz{Time: services.entropy.GetTime(), Valid: true},
	})
	if err != nil {
		return serrors.New("update game metadata", err, "updt", updt)
	}
	slog.InfoContext(ctx, "updated game metadata", "update", updt, "updtResult", updtResult)

	if updtResult.IsNewRow {
		services.broadcaster.BroadcastGameCount(context.WithoutCancel(ctx), updtResult.Count)
	}
	return nil
}

type ChessMetasResp struct {
	AllChessMetas  []model.ChessMeta
	SelfChessMetas []model.ChessMeta
}

func (services *ChessMetaService) GetGameMetadata(ctx context.Context, player optional.Maybe[model.PlayerState], afterOrdering optional.Maybe[int64], count int32) (ChessMetasResp, error) {
	defer perf.WithContext(ctx).Log()

	var allChessMetas []model.ChessMeta
	var userChessMetas []model.ChessMeta

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		allChessMetas, err = services.getGameMetadata(egCtx,
			optional.Nothing[int64](),
			afterOrdering,
			optional.Just[int32](count))

		return serrors.New("get all game metadata after ordering", err, "afterOrdering", afterOrdering)
	})
	if player.Present {
		userID := player.Value.ID
		eg.Go(func() (err error) {
			userChessMetas, err = services.getGameMetadata(ctx,
				optional.Just[int64](userID),
				optional.Nothing[int64](),
				optional.Nothing[int32]())

			return serrors.New("get user game metadata", err, "userID", player.Value.ID)
		})
	}
	if err := eg.Wait(); err != nil {
		return ChessMetasResp{}, err
	}

	return ChessMetasResp{AllChessMetas: allChessMetas, SelfChessMetas: userChessMetas}, nil
}

func (services *ChessMetaService) GetGameMetadataCount(ctx context.Context) (int64, error) {
	defer perf.WithContext(ctx).Log()

	count, err := services.querier.SelectGameMetasCount(ctx)
	if err != nil {
		return 0, serrors.New("count chess metadatas", err)
	}
	slog.InfoContext(ctx, "selected chess metadatas count", "count", count)
	return count, nil
}

func (services *ChessMetaService) getGameMetadata(ctx context.Context, userID optional.Maybe[int64], afterOrdering optional.Maybe[int64], count optional.Maybe[int32]) ([]model.ChessMeta, error) {
	rows, err := services.querier.SelectGameMetas(ctx, query.SelectGameMetasParams{
		ParticipantID: db.MapOptInt8(userID),
		AfterOrdering: afterOrdering.OrElse(math.MaxInt64),
		PerPage:       db.MapOptInt4(count),
	})
	if err != nil {
		return nil, serrors.New("select game metas", err, "userID", userID)
	}

	var chessMetas []model.ChessMeta
	for _, row := range rows {
		mode := enum.Expect(row.Mode, model.GameModeEnums)

		// invariant: if the user id is present, all other fields also will be.
		var whitePlayer, blackPlayer optional.Maybe[model.User]

		if row.WhiteID.Valid {
			whitePlayer = optional.Just(model.User{
				ID:       row.WhiteID.Int64,
				Username: row.WhiteName.String,
				Country:  row.WhiteCountry.String,
			})
		}
		if row.BlackID.Valid {
			blackPlayer = optional.Just(model.User{
				ID:       row.BlackID.Int64,
				Username: row.BlackName.String,
				Country:  row.BlackCountry.String,
			})
		}

		chessMetas = append(chessMetas, model.ChessMeta{
			GameID:      model.GameID(row.GameID),
			Mode:        mode,
			WhitePlayer: whitePlayer,
			BlackPlayer: blackPlayer,
			Ordering:    row.Ordering,
		})
	}

	slog.InfoContext(ctx, "retrieved chess metadatas", "chessMetadata", chessMetas, "afterOrdering", afterOrdering, "count", count)
	return chessMetas, nil
}
