package web

import (
	"errors"
	"fmt"
)

// HTTP error codes
var (
	ErrHttpFatal                = errors.New("ERROR_FATAL")
	ErrHttpInvalidPassword      = errors.New("ERROR_PASSWORD_LENGTH")
	ErrHttpConfirmPassword      = errors.New("ERROR_CONFIRM_PASSWORD")
	ErrHttpInvalidUsername      = errors.New("ERROR_USERNAME_LENGTH")
	ErrHttpInvalidBio           = errors.New("ERROR_BIO_LENGTH")
	ErrHttpUnsafeUsername       = errors.New("ERROR_UNSAFE_USERNAME")
	ErrHttpInvalidParticipants  = errors.New("ERROR_INVALID_PARTICIPANTS")
	ErrHttpDuplicateUsername    = errors.New("ERROR_DUPLICATE_USERNAME")
	ErrHttpInvalidCountry       = errors.New("ERROR_INVALID_COUNTRY")
	ErrHttpInvalidLogin         = errors.New("ERROR_INVALID_LOGIN")
	ErrHttpTooManyLoginAttempts = errors.New("ERROR_TOO_MANY_LOGIN_ATTEMPTS")
	ErrHttpRequiredLogin        = errors.New("ERROR_REQUIRED_LOGIN")
	ErrHttpSessionExpired       = errors.New("ERROR_SESSION_EXPIRED")
	ErrHttpNotFoundChallenge    = errors.New("ERROR_NOT_FOUND_CHALLENGE")
	ErrHttpSelfChallenge        = errors.New("ERROR_SELF_CHALLENGE")
	ErrHttpDuplicateChallenge   = errors.New("ERROR_DUPLICATE_CHALLENGE")
	ErrHttpUpdateChallenge      = errors.New("ERROR_UPDATE_CHALLENGE")
	ErrHttpUserNotFound         = errors.New("ERROR_NOT_FOUND_USER")
	ErrHttpInvalidMode          = errors.New("ERROR_INVALID_MODE")
	ErrHttpSearchLimit          = errors.New("ERROR_SEARCH_LIMIT")
	ErrInvalidFen               = errors.New("ERROR_INVALID_FEN")
	ErrHttpInvalidCount         = errors.New("ERR_HTTP_INVALID_COUNT")
	ErrHttpInvalidPage          = errors.New("ERR_HTTP_INVALID_PAGE")
	ErrHttpInvalidID            = errors.New("ERR_HTTP_INVALID_ID")
	ErrHttpInvalidJSON          = errors.New("ERR_HTTP_INVALID_JSON")
	ErrHttpInvalidTimeframe     = errors.New("ERR_HTTP_INVALID_TIMEFRAME")
	ErrHttpInvalidAction        = errors.New("ERR_HTTP_INVALID_ACTION")
)

// WebSocket response codes
var (
	ErrWsFatal        = errors.New("ERROR_FATAL")
	ErrWsMessageType  = errors.New("ERROR_MESSAGE_TYPE")
	ErrWsTurn         = errors.New("ERROR_TURN")
	ErrWsInvalidMove  = errors.New("ERROR_INVALID_MOVE")
	ErrWsFinishedGame = errors.New("ERROR_FINISHED_GAME")
	ErrWsInvalidGame  = errors.New("ERROR_INVALID_GAME")
	ErrWsExpiration   = errors.New("ERROR_EXPIRED_GAME")
)

// ErrorMap is a utility for storing errors for each field in a request body in a map, inspired by multierror
type ErrorMap struct {
	Errors map[string]error
}

func PutErrorMap(left error, key string, right error) error {
	if right == nil {
		return left
	}
	if left == nil {
		left = &ErrorMap{}
	}
	var errm, ok = left.(*ErrorMap)
	if ok {
		if errm.Errors == nil {
			errm.Errors = make(map[string]error)
		}
		errm.Errors[key] = right
	}
	return left
}

func (m *ErrorMap) Error() string {
	return fmt.Sprintf("%+v", m.Errors)
}
