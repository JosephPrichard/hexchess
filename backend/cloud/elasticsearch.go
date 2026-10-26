package cloud

import (
	"context"
	"fmt"
	"hexchess-svc/utils/slogutil"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const UserEsIndex = "user-search-index"

var EsIndices = []string{
	UserEsIndex,
}

func NewElasticSearchClient(address string) *elasticsearch.TypedClient {
	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{address},
		// Username: "elastic", Password: "...",
	})
	if err != nil {
		slogutil.Fatal("start elastic search client", err)
	}
	return esClient
}

type ElasticSearchIndexConfig struct {
	Shards   string `json:"shards"`
	Replicas string `json:"replicas"`
}

var DefaultElasticSearchIndexConfig = ElasticSearchIndexConfig{Shards: "1", Replicas: "1"}

func CreateElasticSearchIndices(ctx context.Context, cfgs map[string]ElasticSearchIndexConfig, esClient *elasticsearch.TypedClient) ([]create.Response, error) {
	if cfgs == nil {
		cfgs = make(map[string]ElasticSearchIndexConfig)
	}

	var createResults []create.Response

	for _, index := range EsIndices {
		indexCfg, exists := cfgs[index]
		if !exists {
			indexCfg = DefaultElasticSearchIndexConfig
		}

		createResult, err := esClient.Indices.Create(index).
			Request(&create.Request{
				Settings: &types.IndexSettings{
					NumberOfShards:   &indexCfg.Shards,
					NumberOfReplicas: &indexCfg.Replicas,
				},
				Mappings: &types.TypeMapping{
					Properties: map[string]types.Property{
						"id":            types.NewKeywordProperty(), // exact match, sortable (the tiebreaker field)
						"username":      types.NewTextProperty(),    // analyzed for full-text match
						"userBiography": types.NewTextProperty(),    // analyzed for full-text match
						"createdAt":     types.NewDateProperty(),
					},
				},
			}).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create elastic search indices: %v", err)
		}

		createResults = append(createResults, *createResult)
	}

	return createResults, nil
}
