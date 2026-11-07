package game

import (
	"context"
	"hexchess-svc/database"
	"hexchess-svc/database/mutate"
	"hexchess-svc/database/query"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/player"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/user"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

type ChessMetadataService struct {
	database.PostgreSQL
	entropy     entropy.Generator
	broadcaster publishers.Broadcaster
}

func NewChessMetadataService(database database.PostgreSQL, entropy entropy.Generator, broadcaster publishers.Broadcaster) *ChessMetadataService {
	return &ChessMetadataService{PostgreSQL: database, entropy: entropy, broadcaster: broadcaster}
}

func (service *ChessMetadataService) UpdateGameMetadata(ctx context.Context, updt UpdtGameMetadataEvent) (err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	result, err := service.QuerierMutator.UpdateGameMeta(ctx, mutate.UpdateGameMetaParams{
		GameID:    updt.GameID.String(),
		WhiteID:   pgtype.Int8{Int64: updt.WhitePlayer.Value, Valid: updt.WhitePlayer.Present},
		BlackID:   pgtype.Int8{Int64: updt.BlackPlayer.Value, Valid: updt.BlackPlayer.Present},
		Mode:      mutate.ModeEnum(updt.Mode.String()),
		UpdatedOn: pgtype.Timestamptz{Time: service.entropy.GetTime(), Valid: true},
	})
	if err != nil {
		return serrors.New("update game metadata", err, "updt", updt)
	}
	slog.InfoContext(ctx, "updated game metadata", "update", updt, "result", result)

	if result.IsNewRow {
		service.broadcaster.BroadcastGameCount(context.WithoutCancel(ctx), result.Count)
	}
	return nil
}

type ChessMetasResp struct {
	ChessMetas     []ChessMeta
	UserChessMetas []ChessMeta
}

func (service *ChessMetadataService) GetGameMetadata(ctx context.Context, player player.PlayerState, afterOrdering opt.Option[int64], count int32) (_ ChessMetasResp, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	var chessMetas []ChessMeta
	var userChessMetas []ChessMeta

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userID := opt.None[int64]()
		perPage := opt.Some[int32](count)

		chessMetas, err = service.getGameMetadata(egCtx, userID, afterOrdering, perPage)

		return serrors.New("get all game metadata after ordering", err, "afterOrdering", afterOrdering)
	})
	if player.Present {
		eg.Go(func() (err error) {
			userID := opt.Some[int64](player.ID)
			afterOrdering := opt.None[int64]()
			perPage := opt.None[int32]()

			userChessMetas, err = service.getGameMetadata(ctx, userID, afterOrdering, perPage)

			return serrors.New("get user game metadata", err, "userID", player.ID)
		})
	}

	if err := eg.Wait(); err != nil {
		return ChessMetasResp{}, err
	}

	slog.InfoContext(ctx, "completed get game metadata", "chessMetas", chessMetas, "userChessMetas", userChessMetas)
	return ChessMetasResp{ChessMetas: chessMetas, UserChessMetas: userChessMetas}, nil
}

func (service *ChessMetadataService) GetGameMetadataCount(ctx context.Context) (_ int64, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	count, err := service.Querier.SelectGameMetasCount(ctx)
	if err != nil {
		return 0, serrors.New("count chess metadatas", err)
	}

	slog.InfoContext(ctx, "selected chess metadatas count", "count", count)
	return count, nil
}

func (service *ChessMetadataService) getGameMetadata(ctx context.Context, userID opt.Option[int64], afterOrdering opt.Option[int64], count opt.Option[int32]) ([]ChessMeta, error) {
	afterOrderingDB := int64(math.MaxInt64)
	if afterOrdering.Present {
		afterOrderingDB = afterOrdering.Value
	}

	rows, err := service.Querier.SelectGameMetas(ctx, query.SelectGameMetasParams{
		ParticipantID: database.MapOptInt8(userID),
		AfterOrdering: afterOrderingDB,
		PerPage:       database.MapOptInt4(count),
	})
	if err != nil {
		return nil, serrors.New("select game metas", err, "userID", userID)
	}

	var chessMetas []ChessMeta
	for _, row := range rows {
		mode := enum.Expect(row.Mode, gamedata.GameModeEnums)

		// invariant: if the user id is present, all other fields also will be.
		var whitePlayer, blackPlayer opt.Option[user.User]

		if row.WhiteID.Valid {
			whitePlayer = opt.Some(user.User{ID: row.WhiteID.Int64, Username: row.WhiteName.String, Country: row.WhiteCountry.String})
		}
		if row.BlackID.Valid {
			blackPlayer = opt.Some(user.User{ID: row.BlackID.Int64, Username: row.BlackName.String, Country: row.BlackCountry.String})
		}

		chessMetas = append(chessMetas, ChessMeta{
			GameID:      gameid.Identifier(row.GameID),
			Mode:        mode,
			WhitePlayer: whitePlayer,
			BlackPlayer: blackPlayer,
			Ordering:    row.Ordering,
		})
	}

	slog.InfoContext(ctx, "retrieved chess metadatas", "chessMetadata", chessMetas, "afterOrdering", afterOrdering, "count", count)
	return chessMetas, nil
}
