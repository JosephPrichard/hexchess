package svc

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"log/slog"
	"math"
)

func (services *HexchessServices) UpdateGameMetadata(ctx context.Context, updt model.GameMetadataUpdt) error {
	updtResult, err := services.querier.UpdateGameMeta(ctx, sqlc.UpdateGameMetaParams{
		GameID:    updt.GameID,
		WhiteID:   db.MapOptInt8(updt.WhitePlayer),
		BlackID:   db.MapOptInt8(updt.BlackPlayer),
		Mode:      sqlc.ModeEnum(updt.Mode.String()),
		UpdatedOn: pgtype.Timestamptz{Time: services.entropy.GetTime(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update game metadata %+v: %w", updt, err)
	}
	slog.InfoContext(ctx, "updated game metadata", "update", updt, "updtResult", updtResult)

	if updtResult.IsNewRow {
		services.broadcaster.BroadcastGameCount(context.WithoutCancel(ctx), updtResult.Count, pubsub.Async())
	}
	return nil
}

type ChessMetasResp struct {
	AllChessMetas  []model.ChessMeta
	SelfChessMetas []model.ChessMeta
}

func (services *HexchessServices) GetGameMetadata(ctx context.Context, player enum.Optional[model.PlayerState], afterOrdering enum.Optional[int64], count int32) (ChessMetasResp, error) {
	var allChessMetas []model.ChessMeta
	var userChessMetas []model.ChessMeta

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		allChessMetas, err = services.getGameMetadata(egCtx, enum.Nothing[int64](), afterOrdering, enum.Just[int32](count))
		return errutil.Guardf(err, "get all game metadata after ordering %+v", afterOrdering)
	})
	if player.IsPresent {
		userID := player.Value.ID
		eg.Go(func() (err error) {
			userChessMetas, err = services.getGameMetadata(ctx, enum.Just[int64](userID), enum.Nothing[int64](), enum.Nothing[int32]())
			return errutil.Guardf(err, "get user %d game metadata", player.Value.ID)
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
		return 0, fmt.Errorf("count chess metadatas: %w", err)
	}
	slog.InfoContext(ctx, "selected chess metadatas count", "count", count)
	return count, nil
}

func (services *HexchessServices) getGameMetadata(ctx context.Context, userID enum.Optional[int64], afterOrdering enum.Optional[int64], count enum.Optional[int32]) ([]model.ChessMeta, error) {
	rows, err := services.querier.SelectGameMetas(ctx, sqlc.SelectGameMetasParams{
		ParticipantID: db.MapOptInt8(userID),
		AfterOrdering: afterOrdering.OrElse(math.MaxInt64),
		PerPage:       db.MapOptInt4(count),
	})
	if err != nil {
		return nil, fmt.Errorf("select game metas by %+v: %w", userID, err)
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
			GameID:      row.GameID,
			Mode:        mode,
			WhitePlayer: whitePlayer,
			BlackPlayer: blackPlayer,
			Ordering:    row.Ordering,
		})
	}

	slog.InfoContext(ctx, "retrieved chess metadatas", "chessMetadata", chessMetas, "afterOrdering", afterOrdering, "count", count)
	return chessMetas, nil
}
