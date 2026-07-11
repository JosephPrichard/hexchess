package timeutil

import "time"

func ToDayEpoch(time time.Time) int32 {
	return int32(time.Unix() / 86400)
}
