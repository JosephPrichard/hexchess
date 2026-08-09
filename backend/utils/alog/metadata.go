package alog

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bytedance/sonic"
)

type ecsTaskMetadataBody struct {
	Cluster string `json:"Cluster"`
	//ServiceName string `json:"ServiceName"`
	TaskARN  string `json:"TaskARN"`
	Family   string `json:"Family"`
	Revision string `json:"Revision"`
}

var ecsMetadataClient = &http.Client{
	Timeout: 5 * time.Second,
}

func getEcsMetadata() *ecsTaskMetadataBody {
	metadataEndpoint := os.Getenv("ECS_CONTAINER_METADATA_URI")

	resp, err := ecsMetadataClient.Get(metadataEndpoint + "/task")
	if err != nil {
		slog.Error("failed to extract data from metadata endpoint", "error", err, "metadataEndpoint", metadataEndpoint)
		return nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read ecs metadata repsonse body", "error", err, "metadataEndpoint", metadataEndpoint)
		return nil
	}

	var ecsTaskMetadata ecsTaskMetadataBody
	if err := sonic.Unmarshal(bodyBytes, &ecsTaskMetadata); err != nil {
		slog.Error("failed to parse ecs metadata repsonse body", "error", err, "metadataEndpoint", metadataEndpoint)
		return nil
	}

	return &ecsTaskMetadata
}
