package model

import "time"

type Chat struct {
	ID      string      `json:"id"`
	Player  PlayerState `json:"player"`
	Message string      `json:"message"`
	SentAt  time.Time   `json:"sentAt"`
}
