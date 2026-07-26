package config

import (
	"fmt"
	"log/slog"
)

type Profile int

const (
	Local Profile = iota
	Test
	Prod
)

var profileTable = [...]string{"local", "test", "prod"}

func ParseProfile(s string) Profile {
	for i, p := range profileTable {
		if p == s {
			return Profile(i)
		}
	}

	slog.Info("defaulting to local profile")
	return Local
}

func (p Profile) String() string {
	return profileTable[p]
}

func (p Profile) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", p)), nil
}
