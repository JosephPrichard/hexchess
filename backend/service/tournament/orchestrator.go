package tournament

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

type TournamentOrchestrator struct {
	tournament  *UpdateTournamentService
	user        UserGetter
	gameplay    GameCreator
	broadcaster pubsub.Broadcaster
}

type UserGetter interface {
	SelectUsersByIDs(ctx context.Context, ids []int64) ([]model.User, error)
}

type GameCreator interface {
	SetupGame(ctx context.Context, setup model.StateSetup) error
}

func NewTournamentOrchestrator(
	tournament *UpdateTournamentService,
	user UserGetter,
	gameplay GameCreator,
	broadcaster pubsub.Broadcaster,
) *TournamentOrchestrator {
	return &TournamentOrchestrator{tournament: tournament, user: user, gameplay: gameplay, broadcaster: broadcaster}
}

func (services *TournamentOrchestrator) ProgressTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.GameID, error) {
	defer perf.WithContext(ctx).Log()

	// step 1: advance tournament on the database
	matches, err := services.tournament.AdvanceTournament(ctx, tournamentKey, eventID)
	if err != nil {
		return nil, serrors.New("advance tournament", err, "tournamentKey", tournamentKey)
	}
	var matchGameIDs []model.GameID
	for _, match := range matches {
		matchGameIDs = append(matchGameIDs, match.GameID)
	}

	// step 2: create playable games correlated with the committed matches
	userIDs := make([]int64, 0, len(matches)*2)
	for _, match := range matches {
		userIDs = append(userIDs, match.WhiteID, match.BlackID)
	}
	users, err := services.user.SelectUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, serrors.New("select user player data by ids", err)
	}
	userDataMap := make(map[int64]model.User)
	for _, userAccount := range users {
		userDataMap[userAccount.ID] = userAccount
	}

	eg, egCtx := errgroup.WithContext(ctx)

	for i, match := range matches {
		eg.Go(func() error {
			whitePlayerData, okWhite := userDataMap[match.WhiteID]
			blackPlayerData, okBlack := userDataMap[match.BlackID]

			if !okWhite || !okBlack {
				// invariant: white and black should be valid ids if they have been pushed to the queue
				return fmt.Errorf("missing player data for match: %+v", match)
			}

			err := services.gameplay.SetupGame(egCtx, model.StateSetup{
				ID:         match.GameID,
				Mode:       match.GameMode,
				FirstColor: model.White,
				White:      model.NewPlayer(match.WhiteID, whitePlayerData.Username, whitePlayerData.Country),
				Black:      model.NewPlayer(match.BlackID, blackPlayerData.Username, blackPlayerData.Country),
			})
			return serrors.New("create game", err, "index", i, "gameID", match.GameID)
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// step 3: notify tournament participants that tournament has been progressed with new matches
	// TODO: we need to select FullMatch information and broadcast that
	services.broadcaster.BroadcastTournament(ctx, model.TournamentOutput{
		Key:  tournamentKey.String(),
		Kind: model.TournamentMatchmakingKind,
		// Matches: matches,
	})

	return matchGameIDs, nil
}
