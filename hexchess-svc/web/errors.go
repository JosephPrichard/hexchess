package web

import (
	"errors"
	"net/http"
)

// HTTP error codes
var (
	ErrHttpUnknown             = errors.New("ERROR_UNKNOWN")
	ErrHttpInvalidPassword     = errors.New("ERROR_PASSWORD_LENGTH")
	ErrHttpConfirmPassword     = errors.New("ERROR_CONFIRM_PASSWORD")
	ErrHttpInvalidUsername     = errors.New("ERROR_USERNAME_LENGTH")
	ErrHttpUnsafeUsername      = errors.New("ERROR_UNSAFE_USERNAME")
	ErrHttpInvalidParticipants = errors.New("ERROR_INVALID_PARTICIPANTS")
	ErrHttpDuplicateUsername   = errors.New("ERROR_DUPLICATE_USERNAME")
	ErrHttpInvalidLogin        = errors.New("ERROR_INVALID_LOGIN")
	ErrHttpRequiredLogin       = errors.New("ERROR_REQUIRED_LOGIN")
	ErrHttpSessionExpired      = errors.New("ERROR_SESSION_EXPIRED")
	ErrHttpNotFoundChallenge   = errors.New("ERROR_NOT_FOUND_CHALLENGE")
	ErrHttpSelfChallenge       = errors.New("ERROR_SELF_CHALLENGE")
	ErrHttpDuplicateChallenge  = errors.New("ERROR_DUPLICATE_CHALLENGE")
	ErrHttpUpdateChallenge     = errors.New("ERROR_UPDATE_CHALLENGE")
	ErrHttpUserNotFound        = errors.New("ERROR_NOT_FOUND_USER")
	ErrHttpInvalidRequest      = errors.New("ERROR_INVALID_REQUEST")
)

// WebSocket response codes
var (
	ErrWsMessageType  = errors.New("ERROR_MESSAGE_TYPE")
	ErrWsTurn         = errors.New("ERROR_TURN")
	ErrWsInvalidMove  = errors.New("ERROR_INVALID_MOVE")
	ErrWsFinishedGame = errors.New("ERROR_FINISHED_GAME")
	ErrWsInvalidGame  = errors.New("ERROR_INVALID_GAME")
)

func HttpStatusFromError(err error) (int, string) {
	switch err {
	case ErrHttpInvalidPassword,
		ErrHttpConfirmPassword,
		ErrHttpInvalidUsername,
		ErrHttpUnsafeUsername,
		ErrHttpInvalidParticipants,
		ErrHttpInvalidLogin,
		ErrHttpInvalidRequest:
		return http.StatusBadRequest, err.Error()
	case ErrHttpDuplicateUsername,
		ErrHttpDuplicateChallenge:
		return http.StatusConflict, err.Error()
	case ErrHttpRequiredLogin,
		ErrHttpSessionExpired:
		return http.StatusUnauthorized, err.Error()
	case ErrHttpUserNotFound,
		ErrHttpNotFoundChallenge:
		return http.StatusNotFound, err.Error()
	case ErrHttpSelfChallenge,
		ErrHttpUpdateChallenge:
		return http.StatusForbidden, err.Error()
	case ErrHttpUnknown:
		return http.StatusInternalServerError, err.Error()
	default:
		return http.StatusInternalServerError, ErrHttpUnknown.Error()
	}
}
