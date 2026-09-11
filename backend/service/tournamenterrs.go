package service

import (
	"errors"
	"fmt"
)

var (
	ErrTournamentNotLobby               = fmt.Errorf("tournament is not in lobby status")
	ErrTooManyParticipants              = fmt.Errorf("tournament is full")
	ErrTournamentAlreadyJoined          = fmt.Errorf("user is already a participant of this tournament")
	ErrInvalidTournamentParticipant     = fmt.Errorf("tournament participant is invalid")
	ErrTournamentNotFound               = fmt.Errorf("tournament does not exist")
	ErrTournamentCountdownPermissions   = errors.New("only the creating user can begin the tournament countdown")
	ErrInvalidCountdownTournamentStatus = fmt.Errorf("tournament must be in LOBBY status to begin the countdown")
	ErrEmptyMatchesTournament           = errors.New("tournament has no matches")
	ErrMatchRoundCount                  = errors.New("tournament has an invalid completed match count in round")
)
