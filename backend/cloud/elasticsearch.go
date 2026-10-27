package cloud

import (
	"context"
	"fmt"
	"hexchess-svc/config"
	"hexchess-svc/utils/slogutil"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const UserEsIndex = "user-search-index"
const ChatsEsIndex = "chats-index"

var EsIndices = []string{
	UserEsIndex,
	ChatsEsIndex,
}

type EsConfig struct {
	ActiveProfile config.Profile
	Address       string
	CreateIndices bool
}

func NewElasticSearchClient(cfg EsConfig) *elasticsearch.TypedClient {
	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{cfg.Address},
	})
	if err != nil {
		slogutil.Fatal("start elastic search client", err)
	}

	slog.Info("created elasticsearch client", "config", cfg)

	if cfg.CreateIndices {
		if _, err := CreateElasticSearchIndices(context.Background(), esClient); err != nil {
			slogutil.Fatal("failed to create elastic search indices", err)
		}
	}
	return esClient
}

var ElasticSearchIndexConfig = map[string]struct {
	Shards     string                    `json:"shards"`
	Replicas   string                    `json:"replicas"`
	Properties map[string]types.Property `json:"properties"`
}{
	UserEsIndex: {
		Shards:   "1",
		Replicas: "1",
		Properties: map[string]types.Property{
			"id":            types.NewKeywordProperty(),
			"username":      types.NewTextProperty(),
			"userBiography": types.NewTextProperty(),
			"createdAt":     types.NewDateProperty(),
		},
	},
	ChatsEsIndex: {
		Shards:   "1",
		Replicas: "1",
		Properties: map[string]types.Property{
			"id":         types.NewKeywordProperty(),
			"gameID":     types.NewKeywordProperty(),
			"playerId":   types.NewKeywordProperty(),
			"playerName": types.NewTextProperty(),
			"message":    types.NewTextProperty(),
			"createdAt":  types.NewDateProperty(),
		},
	},
}

func CreateElasticSearchIndices(ctx context.Context, esClient *elasticsearch.TypedClient) ([]create.Response, error) {
	var createResults []create.Response

	for _, index := range EsIndices {
		indexCfg, exists := ElasticSearchIndexConfig[index]
		if !exists {
			return nil, fmt.Errorf("missing elastic search index config for index: %s", index)
		}

		createResult, err := esClient.Indices.Create(index).
			Request(&create.Request{
				Settings: &types.IndexSettings{
					NumberOfShards:   &indexCfg.Shards,
					NumberOfReplicas: &indexCfg.Replicas,
				},
				Mappings: &types.TypeMapping{
					Properties: indexCfg.Properties,
				},
			}).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create elastic search indices: %v", err)
		}

		createResults = append(createResults, *createResult)
	}

	slog.Info("created elastic search indices", "createResults", createResults)
	return createResults, nil
}
