package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

func (services *HexchessServices) UpdateGameMetadata(ctx context.Context, updt model.GameMetadataUpdt) error {
	updtResult, err := services.querier.UpdateGameMeta(ctx, sqlc.UpdateGameMetaParams{
		GameID:    updt.GameID.String(),
		WhiteID:   pgtype.Int8{Int64: updt.WhitePlayer, Valid: true},
		BlackID:   pgtype.Int8{Int64: updt.BlackPlayer, Valid: true},
		Mode:      sqlc.ModeEnum(updt.Mode.String()),
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

func (services *HexchessServices) GetGameMetadata(ctx context.Context, player optional.Maybe[model.PlayerState], afterOrdering optional.Maybe[int64], count int32) (ChessMetasResp, error) {
	var allChessMetas []model.ChessMeta
	var userChessMetas []model.ChessMeta

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		allChessMetas, err = services.getGameMetadata(egCtx, optional.Nothing[int64](), afterOrdering, optional.Just[int32](count))
		return serrors.New("get all game metadata after ordering", err, "afterOrdering", afterOrdering)
	})
	if player.Present {
		userID := player.Value.ID
		eg.Go(func() (err error) {
			userChessMetas, err = services.getGameMetadata(ctx, optional.Just[int64](userID), optional.Nothing[int64](), optional.Nothing[int32]())
			return serrors.New("get user game metadata", err, "userID", player.Value.ID)
		})
	}
	if err := eg.Wait(); err != nil {
		return ChessMetasResp{}, err
	}

	return ChessMetasResp{AllChessMetas: allChessMetas, SelfChessMetas: userChessMetas}, nil
}

func (services *HexchessServices) GetGameMetadataCount(ctx context.Context) (int64, error) {
	count, err := services.querier.SelectGameMetasCount(ctx)
	if err != nil {
		return 0, serrors.New("count chess metadatas", err)
	}
	slog.InfoContext(ctx, "selected chess metadatas count", "count", count)
	return count, nil
}

func (services *HexchessServices) getGameMetadata(ctx context.Context, userID optional.Maybe[int64], afterOrdering optional.Maybe[int64], count optional.Maybe[int32]) ([]model.ChessMeta, error) {
	rows, err := services.querier.SelectGameMetas(ctx, sqlc.SelectGameMetasParams{
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
		var whitePlayer, blackPlayer model.User
		if row.WhiteID.Valid {
			whitePlayer = model.User{ID: row.WhiteID.Int64, Username: row.WhiteName.String, Country: row.WhiteCountry.String}
		}
		if row.BlackID.Valid {
			blackPlayer = model.User{ID: row.BlackID.Int64, Username: row.BlackName.String, Country: row.BlackCountry.String}
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
