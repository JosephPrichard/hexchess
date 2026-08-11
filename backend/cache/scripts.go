package cache

import (
	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/ZDEQUEUE_XADD.lua
var zDequeueXAddScript string

var ZDequeueXAdd = redis.NewScript(zDequeueXAddScript)
