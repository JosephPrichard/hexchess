package perf

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type State struct {
	Context     context.Context
	PGPool      *pgxpool.Pool
	RedisClient redis.UniversalClient
}

func (state *State) WithValue(key any, value any) {
	state.Context = context.WithValue(state.Context, key, value)
}