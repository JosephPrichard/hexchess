package svc

import (
	"math/rand/v2" // concurrency safe
)

type PlayerState struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Elo     float64 `json:"elo"`
	IsGuest bool    `json:"isGuest"`
	Present bool    `json:"present"`
}

// Use the constructor functions to create games so the boolean flags will be properly initialized - as opposed to remembering to flag them

func MakeGuest() PlayerState {
	// concurrency safe to use rand - we are also using random negative integers for guests so we will never have a collision with an actual player
	return PlayerState{ID: -rand.Int64(), Name: "Guest", IsGuest: true, Present: true}
}

func MakeIDPlayer(id int64) PlayerState {
	return PlayerState{ID: id, Present: true}
}

func MakeNamePlayer(id int64, name string) PlayerState {
	return PlayerState{ID: id, Name: name, Present: true}
}

func MakePlayer(id int64, name string, country string) PlayerState {
	return PlayerState{ID: id, Name: name, Country: country, Present: true}
}
