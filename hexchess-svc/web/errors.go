package web

import (
	"errors"
	"net/http"
)

// HTTP error codes
var (
	ErrHttpFatal                = errors.New("ERROR_FATAL")
	ErrHttpInvalidPassword      = errors.New("ERROR_PASSWORD_LENGTH")
	ErrHttpConfirmPassword      = errors.New("ERROR_CONFIRM_PASSWORD")
	ErrHttpInvalidUsername      = errors.New("ERROR_USERNAME_LENGTH")
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
	ErrHttpInvalidRequest       = errors.New("ERROR_INVALID_REQUEST")
	ErrHttpInvalidMode          = errors.New("ERROR_INVALID_MODE")
	ErrHttpSearchLimit          = errors.New("ERROR_SEARCH_LIMIT")
	ErrInvalidFen               = errors.New("ERROR_INVALID_FEN")
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

func HttpStatusFromErr(err error) (int, string) {
	switch err {
	case ErrHttpInvalidPassword,
		ErrHttpConfirmPassword,
		ErrHttpInvalidUsername,
		ErrHttpUnsafeUsername,
		ErrHttpInvalidParticipants,
		ErrHttpInvalidRequest,
		ErrHttpSelfChallenge,
		ErrHttpUpdateChallenge,
		ErrHttpSearchLimit,
		ErrHttpInvalidCountry,
		ErrHttpDuplicateUsername,
		ErrInvalidFen,
		ErrHttpDuplicateChallenge:
		return http.StatusBadRequest, err.Error()
	case ErrHttpRequiredLogin,
		ErrHttpInvalidLogin,
		ErrHttpSessionExpired,
		ErrHttpTooManyLoginAttempts:
		return http.StatusUnauthorized, err.Error()
	case ErrHttpUserNotFound,
		ErrHttpNotFoundChallenge:
		return http.StatusNotFound, err.Error()
	case ErrHttpFatal:
		return http.StatusInternalServerError, err.Error()
	default:
		return http.StatusInternalServerError, ErrHttpFatal.Error()
	}
}

func errsOr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
