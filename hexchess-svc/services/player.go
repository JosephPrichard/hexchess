package svc

import (
	"math/rand/v2" // concurrency safe
	"strings"
)

type PlayerState struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	IsGuest bool   `json:"isGuest"`
	Present bool   `json:"present"`
}

const GuestNumLen = 8

// Use the constructor functions to create games so the boolean flags will be properly initialized - as opposed to remembering to flag them

func MakeGuest() PlayerState {
	const characters = "0123456789"

	var name strings.Builder
	name.WriteString("Guest")
	for range GuestNumLen {
		n := rand.IntN(len(characters))
		name.WriteRune(rune(characters[n]))
	}

	// concurrency safe to use rand - we are also using random negative integers for guests so we will never have a collision with an actual player
	return PlayerState{ID: -rand.Int64(), Name: name.String(), IsGuest: true, Present: true}
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
