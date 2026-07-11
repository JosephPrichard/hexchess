package perftest

import (
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	TournamentStatus = "SCHEDULED"
	MaxCount         = 10000
)

type PreconditionData struct {
	TournamentKeys []string
}

func GetPreconditionData(state State) (PreconditionData, error) {
	tournamentKeys, err := SelectInputTournamentKeys(state)
	if err != nil {
		return PreconditionData{}, fmt.Errorf("select input tournament keys: %w", err)
	}
	return PreconditionData{
		TournamentKeys: tournamentKeys,
	}, nil
}

func SelectInputTournamentKeys(state State) ([]string, error) {
	rows, err := state.PGPool.Query(state.Context, "SELECT tournament_key as tkey FROM tournaments WHERE status = $1 LIMIT $2;", TournamentStatus, MaxCount)
	if err != nil {
		return nil, fmt.Errorf("select tournament keys: %w", err)
	}
	defer rows.Close()

	eventRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[struct {
		TournamentKey string `db:"tkey"`
	}])

	var tournamentKeys []string
	for _, row := range eventRows {
		tournamentKeys = append(tournamentKeys, row.TournamentKey)
	}
	return tournamentKeys, err
}
