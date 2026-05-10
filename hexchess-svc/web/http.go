package web

import (
	"context"
	"encoding/json"
	"errors"
	svc "hexchess-svc/service"
	"hexchess-svc/util/enum"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func LevelFromStatus(status int) slog.Level {
	level := slog.LevelInfo
	if status == http.StatusInternalServerError {
		level = slog.LevelError
	} else if status < 200 || status >= 300 {
		level = slog.LevelWarn
	}
	return level
}

func HttpStatusFromErr(err error) (int, string) {
	// map service errors to http errors (some service error mapping is common to every endpoint rather than case by case)
	switch {
	case errors.Is(err, svc.ErrSessionNotFound):
		err = ErrHttpSessionExpired
	default:
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
		ErrHttpInvalidMode,
		ErrHttpInvalidColor,
		ErrHttpInvalidCount,
		ErrHttpInvalidPage,
		ErrHttpInvalidID,
		ErrHttpInvalidJSON,
		ErrHttpInvalidTimeframe,
		ErrHttpInvalidAction,
		ErrHttpSearchLimit,
		ErrHttpInvalidFen,
		ErrHttpInvalidRounds,
		ErrHttpCountdownPermissions:
		return http.StatusBadRequest, err.Error()

	// 401 — Unauthorized
	case ErrHttpInvalidLogin,
		ErrHttpSessionExpired,
		ErrHttpTooManyLoginAttempts:
		return http.StatusUnauthorized, err.Error()

	// 404 — Not Found
	case ErrHttpNotFoundUser,
		ErrHttpNotFoundReplay,
		ErrHttpNotFoundChallenge,
		ErrHttpNotFoundTournament:
		return http.StatusNotFound, err.Error()

	// 412 - Precondition
	case ErrHttpTooManyParticipants,
		ErrHttpTournamentNotLobby,
		ErrHttpInvalidCountdownState:
		return http.StatusPreconditionFailed, err.Error()

	// 500 — Internal API Error
	case ErrHttpFatal:
		return http.StatusInternalServerError, err.Error()

	// fallback
	default:
		return http.StatusInternalServerError, ErrHttpFatal.Error()
	}
}

func HttpStatusFromErrs(err error) ServiceView {
	var errMap map[string]error
	var respErr *ResponseError

	if ok := errors.As(err, &respErr); ok {
		errMap = respErr.Errors
	} else {
		status, errStr := HttpStatusFromErr(err)
		return ServiceView{Status: status, Errors: errStr}
	}

	errStatus := 0
	errStrs := make(map[string]string)

	for key, err := range errMap {
		status, errStr := HttpStatusFromErr(err)
		// yields the most 'severe' status
		if status > errStatus {
			errStatus = status
		}
		errStrs[key] = errStr
	}

	if errStatus == 0 {
		slog.Warn("empty error map reached http status error mapper")
		errStatus = http.StatusInternalServerError
	}
	return ServiceView{Status: errStatus, Errors: errStrs}
}

func parseJSON[Body any](r *http.Request, body *Body, validate func(Body) error) error {
	err := json.NewDecoder(r.Body).Decode(&body)
	defer r.Body.Close()
	if err != nil {
		return ErrHttpInvalidJSON
	}
	if validate != nil {
		if err := validate(*body); err != nil {
			return err
		}
	}
	return nil
}

func transformJSON[Body any, Output any](r *http.Request, transform func(Body) (Output, error)) (Output, error) {
	var body Body
	err := json.NewDecoder(r.Body).Decode(&body)
	defer r.Body.Close()
	if err != nil {
		var o Output
		return o, ErrHttpInvalidJSON
	}
	return transform(body)
}

type ServiceView struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Errors  any    `json:"errors,omitempty"`
}

func writeJSON[V any](w http.ResponseWriter, status int, data V) {
	v, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal json response", "Err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(v); err != nil {
		slog.Error("failed to write json response", "Err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeBytes(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		slog.Error("internal server error", "Err", err)
	}
}

func parseDefaultInt(values url.Values, key string, def int) (int, error) {
	v := values.Get(key)
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func parseOptionalFloat(values url.Values, key string, respError *ResponseError) enum.Optional[float64] {
	v := values.Get(key)
	if v == "" {
		return enum.Optional[float64]{}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		respError.Put(key, ErrHttpInvalidFloat)
		return enum.Optional[float64]{}
	}
	return enum.OptionalOf(f)
}

func parseDefaultString(values url.Values, key, def string) string {
	v := values.Get(key)
	if v == "" {
		return def
	}
	return v
}

func parseOptionalString(values url.Values, key string) enum.Optional[string] {
	v := values.Get(key)
	if v == "" {
		return enum.Optional[string]{}
	}
	return enum.OptionalOf(v)
}

func parseOptionalDatetime(ctx context.Context, values url.Values, key string, respError *ResponseError) enum.Optional[time.Time] {
	v := values.Get(key)
	if v == "" {
		return enum.Optional[time.Time]{}
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		slog.WarnContext(ctx, "invalid date while transforming replays query", "err", err, "key", key)
		respError.Put(key, ErrHttpInvalidDateTime)
	}
	return enum.OptionalOf(t)
}

func parseOptionalInt(values url.Values, key string, respError *ResponseError, err error) enum.Optional[int64] {
	v := values.Get(key)
	if v == "" {
		return enum.Optional[int64]{}
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		respError.Put(key, err)
	}
	return enum.OptionalOf(int64(i))
}
