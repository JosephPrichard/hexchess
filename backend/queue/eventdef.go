package queue

import "hexchess-svc/model"

type AdvanceTournamentJob model.AdvanceTournamentEvent

func (args AdvanceTournamentJob) Kind() string {
	return "advance_tournament"
}
