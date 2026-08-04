package svc

import (
	"context"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type TournamentProgressService struct {
	tournamentService *TournamentService
	userService       *UserService
	gameplayService   *GamePlayService

	broadcaster pubsub.Broadcaster
}

func NewTournamentProgressService(
	tournamentService *TournamentService,
	userService *UserService,
	gameplayService *GamePlayService,
	broadcaster pubsub.Broadcaster,
) *TournamentProgressService {
	return &TournamentProgressService{
		tournamentService: tournamentService,
		userService:       userService,
		gameplayService:   gameplayService,
		broadcaster:       broadcaster,
	}
}

func (services *TournamentProgressService) ProgressTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.GameID, error) {
	defer perf.WithContext(ctx).Log()

	matches, err := services.tournamentService.AdvanceTournament(ctx, tournamentKey, eventID)
	if err != nil {
		return nil, serrors.New("advance tournament", err, "tournamentKey", tournamentKey)
	}
	if err := services.createTournamentMatches(ctx, matches); err != nil {
		return nil, serrors.New("create tournament matches", err)
	}

	// TODO: we need to select FullMatch information and broadcast that
	services.broadcaster.BroadcastTournament(ctx, model.TournamentOutput{
		Key:  tournamentKey.String(),
		Kind: model.TournamentMatchmakingKind,
		// Matches: matches,
	})

	var matchGameIDs []model.GameID
	for _, match := range matches {
		matchGameIDs = append(matchGameIDs, match.GameID)
	}
	return matchGameIDs, nil
}

func (services *TournamentProgressService) createTournamentMatches(ctx context.Context, matches []model.MatchCreation) error {
	defer perf.WithContext(ctx).Log()

	userIDs := make([]int64, 0, len(matches)*2)
	for _, match := range matches {
		userIDs = append(userIDs, match.WhiteID, match.BlackID)
	}
	users, err := services.userService.selectUsersByIDs(ctx, userIDs)
	if err != nil {
		return serrors.New("select user player data by ids", err)
	}
	userDataMap := make(map[int64]model.User)
	for _, user := range users {
		userDataMap[user.ID] = user
	}

	eg, egCtx := errgroup.WithContext(ctx)

	for i, match := range matches {
		whitePlayerData, okWhite := userDataMap[match.WhiteID]
		blackPlayerData, okBlack := userDataMap[match.BlackID]

		if !okWhite || !okBlack {
			// invariant: white and black should be valid ids if they have been pushed to the queue
			return fmt.Errorf("missing player data for match: %+v", match)
		}

		eg.Go(func() error {
			err := services.gameplayService.SetupGame(egCtx, model.StateSetup{
				ID:         match.GameID,
				Mode:       match.GameMode,
				FirstColor: model.White,
				White:      model.NewPlayer(match.WhiteID, whitePlayerData.Username, whitePlayerData.Country),
				Black:      model.NewPlayer(match.BlackID, blackPlayerData.Username, blackPlayerData.Country),
			})
			return serrors.New("create game", err, "index", i, "gameID", match.GameID)
		})
	}

	return eg.Wait()
}
