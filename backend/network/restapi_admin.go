package network

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"hexchess-svc/cache"
	"hexchess-svc/queue"
	"hexchess-svc/queue/producers"
	"net/http"
)

type HttpAdminServer struct {
	riverProducer *producers.RiverProducer
	redisClient   cache.Redis
}

type ProduceEventBody struct {
	EventKind string         `json:"eventKind"`
	EventBody jsontext.Value `json:"eventBody"`
}

const (
	AdvanceTournamentJobName  = "ADVANCE_TOURNAMENT"
	UpdateUserJobName         = "UPDATE_USER"
	FinishGameJobName         = "FINISH_GAME"
	UpdateGameMetadataJobName = "UPDATE_GAME_METADATA"
)

func (server *HttpAdminServer) HandleProduceEvent(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var body ProduceEventBody
	if err := parseJSON(r, &body); err != nil {
		return err
	}

	var err error

	switch body.EventKind {
	case AdvanceTournamentJobName:
		var job producers.AdvanceTournamentArgs
		if err := json.Unmarshal(body.EventBody, &job); err != nil {
			return err
		}
		err = server.riverProducer.ProduceAdvanceTournamentJob(ctx, nil, job)
	case UpdateUserJobName:
		var job producers.UpdateUserArgs
		if err := json.Unmarshal(body.EventBody, &job); err != nil {
			return err
		}
		err = server.riverProducer.ProduceUpdateUserJob(ctx, nil, job)
	case FinishGameJobName:
		var job queue.FinishGameJob
		if err := json.Unmarshal(body.EventBody, &job); err != nil {
			return err
		}
		err = producers.ProduceFinishGameJob(ctx, server.redisClient.PrimaryClient, job)
	case UpdateGameMetadataJobName:
		var job queue.UpdateGameMetadataJob
		if err := json.Unmarshal(body.EventBody, &job); err != nil {
			return err
		}
		err = producers.ProduceUpdateGameMetadataJob(ctx, server.redisClient.PrimaryClient, job)
	}
	if err != nil {
		return err
	}

	writeJSON(ctx, w, http.StatusOK, ServiceResp{Message: "OK"})
	return nil
}
