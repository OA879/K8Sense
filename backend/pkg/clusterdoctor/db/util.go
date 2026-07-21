package db

import "time"

func unixToTime(sec int64) time.Time {
	return time.Unix(sec, 0).UTC()
}
