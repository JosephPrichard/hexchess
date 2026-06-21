package model

import (
	"math/rand/v2"
	"strings"
)

type PlayerState struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Present bool   `json:"present"`
}

func (p PlayerState) IsGuest() bool {
	return IsGuestID(p.ID)
}

func (p PlayerState) NonGuest() bool {
	return !IsGuestID(p.ID) && p.Present
}

func (p PlayerState) IsSame(p1 PlayerState) bool {
	return p.Present && p1.Present && p.ID == p1.ID
}

func IsNonGuestID(id int64) bool {
	return id > 0
}

func IsGuestID(id int64) bool {
	return id < 0
}

const GuestNumLen = 8

// Use the constructor functions to create games so the boolean flags will be properly initialized - as opposed to remembering to flag them

func NewGuestPlayer() PlayerState {
	const characters = "0123456789"

	var name strings.Builder
	name.WriteString("Guest")
	for range GuestNumLen {
		n := rand.IntN(len(characters))
		name.WriteRune(rune(characters[n]))
	}

	// concurrency safe to use rand - we are also using random negative integers for guests so we will never have a collision with an actual player
	return PlayerState{ID: -rand.Int64(), Name: name.String(), Present: true}
}

func NewPlayer(id int64, name string, country string) PlayerState {
	return PlayerState{ID: id, Name: name, Country: country, Present: true}
}
