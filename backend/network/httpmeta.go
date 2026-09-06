package network

import (
	"hexchess-svc/utils/slogutil"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hellofresh/health-go/v5"
)

type HealthConfig struct {
	PostgresCheck     health.CheckFunc
	PostgresReadCheck health.CheckFunc
	RedisPrimaryCheck health.CheckFunc
	RedisPubSubCheck  health.CheckFunc
}

const (
	PostgresLabel     = "Postgres"
	PostgresReadLabel = "ReadPostgres"
	RedisPubsubLabel  = "RedisPubsub"
	RedisPrimaryLabel = "RedisPrimary"
)

func NewHealthCheck(config HealthConfig) func(*chi.Mux) {
	return func(mux *chi.Mux) {
		healthChecks := []health.Config{
			{
				Name:    PostgresLabel,
				Timeout: time.Second * 5,
				Check:   config.PostgresCheck,
			},
			{
				Name:    PostgresReadLabel,
				Timeout: time.Second * 5,
				Check:   config.PostgresReadCheck,
			},
			{
				Name:      RedisPubsubLabel,
				Timeout:   time.Second * 5,
				SkipOnErr: true,
				Check:     config.RedisPubSubCheck,
			},
			{
				Name:      RedisPrimaryLabel,
				Timeout:   time.Second * 5,
				SkipOnErr: true,
				Check:     config.RedisPrimaryCheck,
			},
		}

		healthcheckHandler, err := health.New(
			health.WithComponent(health.Component{Name: "hexchess-svc", Version: "v1.0"}),
			health.WithChecks(healthChecks...),
		)
		if err != nil {
			slogutil.Fatal("failed to create health check handler", err)
		}

		mux.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
			//slog.InfoContext(r.Context(), "health check", "method", r.Method, "url", r.URL.String())
			healthcheckHandler.HandlerFunc(w, r)
		})
	}
}

func info(w http.ResponseWriter, _ *http.Request) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	resp := struct {
		CommitHash     string `json:"commitHash"`
		LastCommitTime string `json:"lastCommitTime"`
		Modified       string `json:"modified"`
	}{}

	for _, setting := range bi.Settings {
		if setting.Key == "vcs.revision" {
			resp.CommitHash = setting.Value
		}
		if setting.Key == "vcs.time" {
			resp.LastCommitTime = setting.Value
		}
		if setting.Key == "vcs.modified" {
			resp.Modified = setting.Value
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
