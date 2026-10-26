package service

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/database/query"
	"hexchess-svc/featureflag"
	"hexchess-svc/model"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"strconv"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"golang.org/x/sync/errgroup"
)

type UserSearchService struct {
	// infra
	querier  query.Querier
	esClient *elasticsearch.TypedClient

	// config
	FeatureFlags featureflag.Config
}

func NewUserSearchService(querier query.Querier, esClient *elasticsearch.TypedClient) *UserSearchService {
	return &UserSearchService{querier: querier, esClient: esClient, FeatureFlags: featureflag.FeatureFlagConfig}
}

const (
	MaxPostgresSearchOffset = 1000
	UserSearchEsKeepAlive   = "15s"
)

type SearchableUserDocument struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	UserCountry   string    `json:"userCountry"`
	UserBiography string    `json:"userBiography"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (service *UserSearchService) UpdateUserSearch(ctx context.Context, event model.UpdateUserEvent) (err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	userRow, err := service.querier.SelectUserByID(ctx, event.ID)
	if err != nil {
		return serrors.New("select user by id", err, "userID", event.ID)
	}

	userDocument := SearchableUserDocument{
		ID:            strconv.FormatInt(event.ID, 10),
		Username:      userRow.Username,
		UserCountry:   userRow.Country,
		UserBiography: userRow.Bio,
		CreatedAt:     userRow.JoinedOn.Time,
	}
	return service.insertSearchableUser(ctx, userDocument)
}

func (service *UserSearchService) insertSearchableUser(ctx context.Context, userDocument SearchableUserDocument) (err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	if !service.FeatureFlags.IsUserSearchElasticSearch {
		return
	}

	result, err := service.esClient.Index(cloud.UserEsIndex).
		Id(userDocument.ID).
		Request(userDocument).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return serrors.New("insert user search index", err, "userID", userDocument.ID)
	}

	slog.Info("successfully updated user in elasticsearch index", "result", result, "index", cloud.UserEsIndex)
	return nil
}

func (service *UserSearchService) SearchUsers(ctx context.Context, searchInput string, page, perPage int32) (_ []model.LbdUser, err error) {
	if searchInput == "" {
		return []model.LbdUser{}, nil
	}

	page = max(page, 1)
	offset := (page - 1) * perPage

	if service.FeatureFlags.IsUserSearchElasticSearch {
		return service.searchUsersEs(ctx, searchInput, offset, page, perPage)
	} else {
		return service.searchUsersPostgres(ctx, searchInput, offset, page, perPage)
	}
}

func (service *UserSearchService) doElasticUserSearch(ctx context.Context, searchInput string, offset, perPage int32) (_ []int64, err error) {
	esFrom := int(offset)
	esSearchSize := int(perPage)

	searchRequest := &search.Request{
		From: &esFrom,
		Size: &esSearchSize,
		Query: &types.Query{
			MultiMatch: &types.MultiMatchQuery{
				Query:  searchInput,
				Fields: []string{"username", "userBiography"},
			},
		},
		// A stable sort keeps pages consistent between calls
		//Sort: []types.SortCombinations{"_score", types.SortOptions{SortOptions: map[string]types.FieldSort{"_id": {}}}},
	}
	searchResponse, err := service.esClient.Search().
		Index(cloud.UserEsIndex).
		Request(searchRequest).
		Scroll(UserSearchEsKeepAlive).
		Do(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to search users in elasticsearch", "error", err, "searchInput", searchInput, "rootCause", elasticsearchRootCause(err))
		return nil, serrors.New("search users in elasticsearch", err)
	}

	slog.InfoContext(ctx, "retrieved users elasticsearch response", "searchInput", searchInput, "index", cloud.UserEsIndex, "searchResponse", searchResponse)

	scrollID := searchResponse.ScrollId_
	hits := searchResponse.Hits.Hits

	var allHits []types.Hit
	allHits = append(allHits, hits...)

	for len(hits) > 0 && scrollID != nil {
		next, err := service.esClient.Scroll().
			ScrollId(*scrollID).
			Scroll(UserSearchEsKeepAlive).
			Do(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to scroll users in elasticsearch", "error", err, "searchInput", searchInput, "scrollID", scrollID, "rootCause", elasticsearchRootCause(err))
			return nil, serrors.New("scroll users in elasticsearch", err)
		}

		scrollID = next.ScrollId_
		hits = next.Hits.Hits
		allHits = append(allHits, hits...)
	}

	strScrollID := fmt.Sprintf("%v", scrollID)
	slog.InfoContext(ctx, "finished scrolling users elasticsearch response", "searchInput", searchInput, "index", cloud.UserEsIndex, "hits", hits, "scrollID", strScrollID)

	if scrollID != nil {
		if _, err := service.esClient.ClearScroll().ScrollId(*scrollID).Do(ctx); err != nil {
			slog.ErrorContext(ctx, "failed to clear scroll for username elasticsearch request", "err", err, "scrollID", strScrollID)
		}
	}

	var userIDs []int64
	for _, hit := range allHits {
		if hit.Id_ == nil {
			continue
		}
		userID, err := strconv.ParseInt(*hit.Id_, 10, 64)
		if err != nil {
			slog.WarnContext(ctx, "users elasticsearch response: _id field must be an integer", "error", err, "_id", *hit.Id_)
			continue
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

func (service *UserSearchService) searchUsersEs(ctx context.Context, searchInput string, offset, page, perPage int32) (_ []model.LbdUser, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	userIDs, err := service.doElasticUserSearch(ctx, searchInput, offset, perPage)
	if err != nil {
		return nil, err
	}

	var userRows []query.SelectUsersByIDsRow
	var eloRows []query.UserModeElo

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userRows, err = service.querier.SelectUsersByIDs(egCtx, userIDs)
		return serrors.New("select users by user ids", err, "userIDs", userIDs)
	})
	eg.Go(func() (err error) {
		eloRows, err = service.querier.SelectUserElosByIDs(egCtx, userIDs)
		return serrors.New("select elos by user ids", err, "userIDs", userIDs)
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	statAggrMap := createUserStatTotalsMap(eloRows)

	leaderboardUsers := make([]model.LbdUser, 0, len(userRows))
	for i, userRow := range userRows {
		searchRank := getSearchRank(int(page), int(perPage), i)

		userStatAggr := statAggrMap[userRow.ID]

		leaderboardUsers = append(leaderboardUsers, model.LbdUser{
			ID:         userRow.ID,
			Username:   userRow.Username,
			Country:    userRow.Country,
			Elo:        userStatAggr.Elo,
			HighestElo: userStatAggr.HighestElo,
			Wins:       userStatAggr.Wins,
			Losses:     userStatAggr.Losses,
			Draws:      userStatAggr.Draws,
			Winrate:    userStatAggr.Winrate,
			Rank:       searchRank,
		})
	}

	slog.InfoContext(ctx, "finished users fuzzy search", "impl", "elasticsearch", "users", leaderboardUsers, "name", searchInput, "page", page, "limit", page, "offset", offset)
	return leaderboardUsers, nil
}

var ErrSearchLimit = errors.New("search limit exceeded")

func (service *UserSearchService) searchUsersPostgres(ctx context.Context, searchInput string, offset, page, perPage int32) (_ []model.LbdUser, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	if offset > MaxPostgresSearchOffset {
		slog.WarnContext(ctx, "failed to search offset exceeds maximum", "offset", offset, "maxOffset", MaxPostgresSearchOffset)
		return []model.LbdUser{}, ErrSearchLimit
	}

	userRows, err := service.querier.SelectUsersBySimilarity(ctx, query.SelectUsersBySimilarityParams{
		Username: searchInput,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		return nil, serrors.New("select users by similarity", err)
	}

	var userIDs []int64
	for _, row := range userRows {
		userIDs = append(userIDs, row.ID)
	}

	eloRows, err := service.querier.SelectUserElosByIDs(ctx, userIDs)
	if err != nil {
		return nil, serrors.New("select elos by user ids", err, "userIDs", userIDs)
	}

	statAggrMap := createUserStatTotalsMap(eloRows)

	leaderboardUsers := make([]model.LbdUser, 0, len(userRows))
	for i, userRow := range userRows {
		searchRank := getSearchRank(int(page), int(perPage), i)

		userStatAggr := statAggrMap[userRow.ID]

		leaderboardUsers = append(leaderboardUsers, model.LbdUser{
			ID:         userRow.ID,
			Username:   userRow.Username,
			Country:    userRow.Country,
			Elo:        userStatAggr.Elo,
			HighestElo: userStatAggr.HighestElo,
			Wins:       userStatAggr.Wins,
			Losses:     userStatAggr.Losses,
			Draws:      userStatAggr.Draws,
			Winrate:    userStatAggr.Winrate,
			Rank:       searchRank,
		})
	}

	slog.InfoContext(ctx, "finished users fuzzy search", "impl", "postgres", "users", leaderboardUsers, "name", searchInput, "page", page, "limit", page, "offset", offset)
	return leaderboardUsers, nil
}

func getSearchRank(page, perPage, index int) int64 {
	return int64((page-1)*perPage + index + 1)
}

func elasticsearchRootCause(err error) string {
	if err == nil {
		return ""
	}
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		for _, rc := range esErr.ErrorCause.RootCause {
			if rc.Reason != nil && *rc.Reason != "" {
				return *rc.Reason
			}
		}
	}
	return err.Error()
}

type userStatTotal struct {
	HighestElo float64
	Elo        float64
	Datapoints int
	Wins       int32
	Losses     int32
	Draws      int32
	Winrate    int64
}

func createUserStatTotalsMap(eloRows []query.UserModeElo) map[int64]userStatTotal {
	eloAggrMap := make(map[int64]userStatTotal)
	for _, row := range eloRows {
		aggr := eloAggrMap[row.UserID]

		aggr.Elo = average(aggr.Elo, aggr.Datapoints, row.Elo)
		aggr.HighestElo = max(aggr.HighestElo, row.Elo)
		aggr.Winrate = average(aggr.Winrate, aggr.Datapoints, model.CalcUserWinrate(row.Wins, row.Losses, row.Draws))

		aggr.Losses += row.Losses
		aggr.Wins += row.Wins
		aggr.Draws += row.Draws

		aggr.Datapoints += 1

		eloAggrMap[row.UserID] = aggr
	}
	return eloAggrMap
}
