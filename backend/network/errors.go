package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/service"
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
	ErrHttpInvalidChecksum       = errors.New("ERROR_INVALID_CHECKSUM")
	ErrHttpErrProfilePicTooBig   = errors.New("ERROR_PROFILE_PIC_TOO_BIG")
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

func mapBadRequestError(err error) (ServiceResp, bool) {
	var respErr *BadRequestError
	ok := errors.As(err, &respErr)
	if !ok {
		return ServiceResp{}, false
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

	return ServiceResp{Status: http.StatusBadRequest, Errors: strMap}, true
}

func ServiceViewFromErr(err error) ServiceResp {
	var msg string

	if resp, ok := mapBadRequestError(err); ok {
		return resp
	}

	// map business logic errors to http errors
	err, msg = mapServiceErrors(err)

	// map specific stdlib errors with messages safe for http responses to http errors
	switch checkErr := err.(type) {
	case *json.UnmarshalTypeError:
		err, msg = ErrHttpInvalidJSON, checkErr.Error()
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
		ErrHttpInvalidRounds,
		ErrHttpInvalidChecksum,
		ErrHttpErrProfilePicTooBig:
		return ServiceResp{Status: http.StatusBadRequest, Message: msg, Error: err.Error()}
	// 401 — Unauthorized
	case ErrHttpInvalidLogin,
		ErrHttpSessionExpired,
		ErrHttpTooManyLoginAttempts:
		return ServiceResp{Status: http.StatusUnauthorized, Message: msg, Error: err.Error()}
	// 404 — Not Found
	case ErrHttpNotFoundUser,
		ErrHttpNotFoundReplay,
		ErrHttpNotFoundChallenge,
		ErrHttpNotFoundTournament:
		return ServiceResp{Status: http.StatusNotFound, Message: msg, Error: err.Error()}
	// 412 - Precondition
	case ErrHttpTooManyParticipants,
		ErrHttpTournamentNotLobby,
		ErrHttpInvalidCountdownState:
		return ServiceResp{Status: http.StatusPreconditionFailed, Message: msg, Error: err.Error()}
	// 500 — Internal API Error
	case ErrHttpFatal:
		return ServiceResp{Status: http.StatusInternalServerError, Message: msg, Error: err.Error()}
	// fallback
	default:
		return ServiceResp{Status: http.StatusInternalServerError, Error: ErrHttpFatal.Error()}
	}
}

func mapServiceErrors(err error) (error, string) {
	switch {
	case errors.Is(err, service.ErrSessionNotFound):
		return ErrHttpSessionExpired, err.Error()
	case errors.Is(err, service.ErrUserNotFound):
		return ErrHttpInvalidLogin, err.Error()
	case errors.Is(err, service.ErrTooManyLoginAttempts):
		return ErrHttpTooManyLoginAttempts, err.Error()
	case errors.Is(err, service.ErrDuplicateChallenge):
		return ErrHttpDuplicateChallenge, err.Error()
	case errors.Is(err, service.ErrInvalidChallengeMember):
		return ErrHttpInvalidParticipants, err.Error()
	case errors.Is(err, service.ErrSelfChallenge):
		return ErrHttpSelfChallenge, err.Error()
	case errors.Is(err, service.ErrTournamentNotFound):
		return ErrHttpNotFoundTournament, err.Error()
	case errors.Is(err, service.ErrTooManyParticipants):
		return ErrHttpTooManyParticipants, err.Error()
	case errors.Is(err, service.ErrTournamentNotLobby):
		return ErrHttpTournamentNotLobby, err.Error()
	case errors.Is(err, service.ErrInvalidCountdownTournamentStatus):
		return ErrHttpInvalidCountdownState, err.Error()
	case errors.Is(err, service.ErrTournamentCountdownPermissions):
		return ErrHttpCountdownPermissions, err.Error()
	case errors.Is(err, service.ErrChallengeNotFound):
		return ErrHttpNotFoundChallenge, err.Error()
	case errors.Is(err, service.ErrTournamentNotFound):
		return ErrHttpNotFoundTournament, err.Error()
	case errors.Is(err, service.InvalidChecksum):
		return ErrHttpInvalidChecksum, err.Error()
	case errors.Is(err, service.ErrProfilePicTooBig):
		return ErrHttpErrProfilePicTooBig, err.Error()
	}
	return err, ""
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
