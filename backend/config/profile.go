package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Profile int

const (
	Local Profile = iota
	Test
	Prod
)

var profileTable = [...]string{"local", "test", "prod"}

func ParseProfile(s string) Profile {
	if s == "" {
		return Local
	}
	s = strings.ToLower(s)
	for i, p := range profileTable {
		if p == s {
			return Profile(i)
		}
	}
	slog.Error("unknown profile", "profile", s)
	os.Exit(1)
	return Local
}

func (p Profile) String() string {
	return profileTable[p]
}

func (p Profile) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", p)), nil
}
