package tournament

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type RetrieveTournamentService struct {
	querier query.Querier
	redis   cache.Redis
}

func NewRetrieveTournamentService(querier query.Querier, redis cache.Redis) *RetrieveTournamentService {
	return &RetrieveTournamentService{querier: querier, redis: redis}
}

func (services *RetrieveTournamentService) GetTournament(ctx context.Context, tournamentKey uuid.UUID) (model.FullTournament, error) {
	defer perf.WithContext(ctx).Log()

	var tournamentRow query.SelectTournamentByIDRow
	var matchRows []query.SelectReplayMatchesByTournamentIDRow
	var participantRows []query.SelectParticipantsWithUserByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = services.querier.SelectTournamentByID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return serrors.New("select tournament by key", err, "tournamentKey", tournamentKey)
	})

	eg.Go(func() (err error) {
		participantRows, err = services.querier.SelectParticipantsWithUserByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return serrors.New("select participants by tournament key", err, "tournamentKey", tournamentKey)
	})

	eg.Go(func() (err error) {
		matchRows, err = services.querier.SelectReplayMatchesByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return serrors.New("select replay matches by tournament key", err, "tournamentKey", tournamentKey)
	})

	if err := eg.Wait(); err != nil {
		if database.IsErrNoRows(err) {
			return model.FullTournament{}, ErrTournamentNotFound
		} else {
			return model.FullTournament{}, err
		}
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournamentRow, "matchRows", matchRows, "participantRows", participantRows)

	tournament := mapFullTournament(tournamentRow, matchRows, participantRows)

	userLdbRanksMap, err := services.getParticipantsRank(ctx, tournament.Participants, tournament.Mode)
	if err != nil {
		return model.FullTournament{}, serrors.New("get participants leaderboard rank", err, "participantIDs", tournament.Participants)
	}
	for i := range tournament.Participants {
		tournament.Participants[i].Rank = userLdbRanksMap[tournament.Participants[i].ID]
	}

	slog.InfoContext(ctx, "retrieved full tournament", "tournament", tournament)
	return tournament, nil
}

func (services *RetrieveTournamentService) getParticipantsRank(ctx context.Context, participants []model.Participant, mode model.GameMode) (map[int64]int64, error) {
	type getExec struct {
		userID int64
		cmd    *redis.IntCmd
	}

	pipeline := services.redis.PrimaryClient.Pipeline()

	var getExecs []getExec
	for _, participant := range participants {
		modeLbZSet := cache.FmtLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			userID: participant.ID,
			cmd:    pipeline.ZRevRank(ctx, modeLbZSet, strconv.Itoa(int(participant.ID))),
		})
	}

	if err := cache.PipelineExec(ctx, pipeline); err != nil {
		return nil, err
	}

	leaderboardRanks := make(map[int64]int64)
	for _, exec := range getExecs {
		rank, err := exec.cmd.Result()
		if errors.Is(redis.Nil, err) {
			continue
		}
		if err != nil {
			return nil, serrors.New("get participant rank for user", err, "userID", exec.userID)
		}
		leaderboardRanks[exec.userID] = leaderboard.MapLeaderboardRank(rank)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "leaderboardRanks", leaderboardRanks, "mode", mode)
	return leaderboardRanks, nil
}

func (services *RetrieveTournamentService) GetTournaments(ctx context.Context, participantID opt.Option[int64], afterID opt.Option[int64], perPage int32) ([]model.Tournament, error) {
	defer perf.WithContext(ctx).Log()

	if !afterID.Present {
		afterID.Value = int64(math.MaxInt64)
	}

	var tournaments []model.Tournament

	if participantID.Present {
		tournamentRows, err := services.querier.SelectTournamentsByParticipant(ctx, query.SelectTournamentsByParticipantParams{
			UserID:  participantID.Value,
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, serrors.New("select tournaments by participant after id", err, "participantID", participantID, "afterID", afterID)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t query.SelectTournamentsByParticipantRow) model.Tournament {
			return mapTournamentByIdRow(query.SelectTournamentByIDRow(t))
		})
	} else {
		tournamentRows, err := services.querier.SelectTournaments(ctx, query.SelectTournamentsParams{
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, serrors.New("select tournaments after id", err, "afterID", afterID)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t query.SelectTournamentsRow) model.Tournament {
			return mapTournamentByIdRow(query.SelectTournamentByIDRow(t))
		})
	}

	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)
	return tournaments, nil
}
