package timeutil

import "time"

func DaysEpoch(time time.Time) int32 {
	return int32(time.Unix() / 86400)
}
