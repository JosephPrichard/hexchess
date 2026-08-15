package cache

import (
	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/POLL_GAME_TIMERS.lua
var pollGameTimers string

var PollGameTimers = redis.NewScript(pollGameTimers)
