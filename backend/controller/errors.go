package controller

import (
	"errors"
	"fmt"
	svc "hexchess-svc/service"
	"net/http"
)

// HTTP error codes
var (
	ErrHttpFatal                 = errors.New("ERROR_FATAL")
	ErrHttpInvalidJSON           = errors.New("ERROR_INVALID_JSON")
	ErrHttpInvalidInput          = errors.New("ERROR_INVALID_INPUT")
	ErrHttpInvalidPassword       = errors.New("ERROR_PASSWORD_LENGTH")
	ErrHttpConfirmPassword       = errors.New("ERROR_CONFIRM_PASSWORD")
	ErrHttpInvalidUsername       = errors.New("ERROR_USERNAME_LENGTH")
	ErrHttpInvalidBio            = errors.New("ERROR_BIO_LENGTH")
	ErrHttpUnsafeUsername        = errors.New("ERROR_UNSAFE_USERNAME")
	ErrHttpInvalidParticipants   = errors.New("ERROR_INVALID_PARTICIPANTS")
	ErrHttpDuplicateUsername     = errors.New("ERROR_DUPLICATE_USERNAME")
	ErrHttpInvalidCountry        = errors.New("ERROR_INVALID_COUNTRY")
	ErrHttpInvalidLogin          = errors.New("ERROR_INVALID_LOGIN")
	ErrHttpTooManyLoginAttempts  = errors.New("ERROR_TOO_MANY_LOGIN_ATTEMPTS")
	ErrHttpSessionExpired        = errors.New("ERROR_SESSION_EXPIRED")
	ErrHttpNotFoundChallenge     = errors.New("ERROR_NOT_FOUND_CHALLENGE")
	ErrHttpSelfChallenge         = errors.New("ERROR_SELF_CHALLENGE")
	ErrHttpDuplicateChallenge    = errors.New("ERROR_DUPLICATE_CHALLENGE")
	ErrHttpUpdateChallenge       = errors.New("ERROR_UPDATE_CHALLENGE")
	ErrHttpNotFoundReplay        = errors.New("ERROR_NOT_FOUND_REPLAY")
	ErrHttpNotFoundUser          = errors.New("ERROR_NOT_FOUND_USER")
	ErrHttpNotFoundTournament    = errors.New("ERROR_NOT_FOUND_TOURNAMENT")
	ErrHttpTournamentNotLobby    = errors.New("ERROR_NOT_LOBBY")
	ErrHttpTooManyParticipants   = errors.New("ERROR_TOO_MANY_PARTICIPANTS")
	ErrHttpInvalidCountdownState = errors.New("ERROR_INVALID_COUNTDOWN_STATE")
	ErrHttpCountdownPermissions  = errors.New("ERROR_COUNTDOWN_PERMISSIONS")
	ErrHttpInvalidRounds         = errors.New("ERROR_INVALID_ROUNDS")
)

// WebSocket response codes
var (
	ErrWsFatal          = errors.New("ERROR_FATAL")
	ErrWsMessageType    = errors.New("ERROR_MESSAGE_TYPE")
	ErrWsTurn           = errors.New("ERROR_TURN")
	ErrWsInvalidMove    = errors.New("ERROR_INVALID_MOVE")
	ErrWsFinishedGame   = errors.New("ERROR_FINISHED_GAME")
	ErrWsStartedGame    = errors.New("ERROR_STARTED_GAME")
	ErrWsForfeitPlayer  = errors.New("ERROR_FORFEIT_PLAYER")
	ErrWsInvalidGame    = errors.New("ERROR_INVALID_GAME")
	ErrWsExpiration     = errors.New("ERROR_EXPIRED_GAME")
	ErrWsUndoCurrPlayer = errors.New("ERROR_UNDO_CURR_PLAYER")
	ErrWsUndoAction     = errors.New("ERR_UNDO_ACTION")
)

func mapBadRequestError(err error) (ServiceView, bool) {
	var respErr *BadRequestError
	ok := errors.As(err, &respErr)
	if !ok {
		return ServiceView{}, false
	}
	strMap := make(map[string]OneError)

	for k, v := range respErr.Errors {
		var targetErr error
		fieldErr := ValidationFieldErrorMap[k]
		if fieldErr != nil {
			targetErr = fieldErr
		} else {
			targetErr = ErrHttpInvalidInput
		}
		strMap[k] = OneError{Message: v.Error(), Error: targetErr.Error()}
	}

	return ServiceView{Status: http.StatusBadRequest, Errors: strMap}, true
}

func ServiceViewFromErr(err error) ServiceView {
	var msg string

	if resp, ok := mapBadRequestError(err); ok {
		return resp
	}

	switch {
	case errors.Is(err, svc.ErrSessionNotFound):
		err, msg = ErrHttpSessionExpired, err.Error()
	case errors.Is(err, svc.ErrUserNotFound):
		err, msg = ErrHttpInvalidLogin, err.Error()
	case errors.Is(err, svc.ErrTooManyLoginAttempts):
		err, msg = ErrHttpTooManyLoginAttempts, err.Error()
	case errors.Is(err, svc.ErrDuplicateChallenge):
		err, msg = ErrHttpDuplicateChallenge, err.Error()
	case errors.Is(err, svc.ErrInvalidChallengeMember):
		err, msg = ErrHttpInvalidParticipants, err.Error()
	case errors.Is(err, svc.ErrSelfChallenge):
		err, msg = ErrHttpSelfChallenge, err.Error()
	case errors.Is(err, svc.ErrTournamentNotFound):
		err, msg = ErrHttpNotFoundTournament, err.Error()
	case errors.Is(err, svc.ErrTooManyParticipants):
		err, msg = ErrHttpTooManyParticipants, err.Error()
	case errors.Is(err, svc.ErrTournamentNotLobby):
		err, msg = ErrHttpTournamentNotLobby, err.Error()
	case errors.Is(err, svc.ErrInvalidCountdownTournamentStatus):
		err, msg = ErrHttpInvalidCountdownState, err.Error()
	case errors.Is(err, svc.ErrTournamentCountdownPermissions):
		err, msg = ErrHttpCountdownPermissions, err.Error()
	case errors.Is(err, svc.ErrChallengeNotFound):
		err, msg = ErrHttpNotFoundChallenge, err.Error()
	case errors.Is(err, svc.ErrTournamentNotFound):
		err, msg = ErrHttpNotFoundTournament, err.Error()
	}

	// map http error codes to status codes
	switch err {
	// 400 — Bad Request
	case ErrHttpInvalidPassword,
		ErrHttpConfirmPassword,
		ErrHttpInvalidUsername,
		ErrHttpInvalidBio,
		ErrHttpUnsafeUsername,
		ErrHttpInvalidParticipants,
		ErrHttpInvalidCountry,
		ErrHttpDuplicateUsername,
		ErrHttpSelfChallenge,
		ErrHttpDuplicateChallenge,
		ErrHttpUpdateChallenge,
		ErrHttpInvalidJSON,
		ErrHttpCountdownPermissions,
		ErrHttpInvalidRounds:
		return ServiceView{Status: http.StatusBadRequest, Message: msg, Error: err.Error()}
	// 401 — Unauthorized
	case ErrHttpInvalidLogin,
		ErrHttpSessionExpired,
		ErrHttpTooManyLoginAttempts:
		return ServiceView{Status: http.StatusUnauthorized, Message: msg, Error: err.Error()}
	// 404 — Not Found
	case ErrHttpNotFoundUser,
		ErrHttpNotFoundReplay,
		ErrHttpNotFoundChallenge,
		ErrHttpNotFoundTournament:
		return ServiceView{Status: http.StatusNotFound, Message: msg, Error: err.Error()}
	// 412 - Precondition
	case ErrHttpTooManyParticipants,
		ErrHttpTournamentNotLobby,
		ErrHttpInvalidCountdownState:
		return ServiceView{Status: http.StatusPreconditionFailed, Message: msg, Error: err.Error()}
	// 500 — Internal API Error
	case ErrHttpFatal:
		return ServiceView{Status: http.StatusInternalServerError, Message: msg, Error: err.Error()}
	// fallback
	default:
		return ServiceView{Status: http.StatusInternalServerError, Error: ErrHttpFatal.Error()}
	}
}

type BadRequestError struct {
	Errors map[string]error
}

func (re *BadRequestError) Put(key string, newErr error) {
	if re.Errors == nil {
		re.Errors = make(map[string]error)
	}
	re.Errors[key] = newErr
}

func respError(key string, err error) error {
	return &BadRequestError{Errors: map[string]error{key: err}}
}

func (re *BadRequestError) HasErrors() bool {
	return len(re.Errors) > 0
}

func (re *BadRequestError) Error() string {
	return fmt.Sprintf("%+v", re.Errors)
}

func (re *BadRequestError) Inner() error {
	if re.HasErrors() {
		return re
	}
	return nil
}
