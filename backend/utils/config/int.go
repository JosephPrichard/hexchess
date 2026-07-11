package config

import "strconv"

func MustParseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}

func ParseDefaultInt(s string, d int) int {
	if s == "" {
		return d
	}
	return MustParseInt(s)
}
