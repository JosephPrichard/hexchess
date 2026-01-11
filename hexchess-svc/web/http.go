package web

import (
	"encoding/json"
	"errors"
	"hexchess-svc/pkg/errmap"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

func HttpStatusFromErr(err error) (int, string) {
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
		ErrHttpInvalidFen:
		return http.StatusBadRequest, err.Error()

	// 401 — Unauthorized
	case ErrHttpRequiredLogin,
		ErrHttpInvalidLogin,
		ErrHttpSessionExpired,
		ErrHttpTooManyLoginAttempts:
		return http.StatusUnauthorized, err.Error()

	// 404 — Not Found
	case ErrHttpUserNotFound,
		ErrHttpNotFoundChallenge:
		return http.StatusNotFound, err.Error()

	// 500 — Internal Server Error
	case ErrHttpFatal:
		return http.StatusInternalServerError, err.Error()

	// fallback
	default:
		return http.StatusInternalServerError, ErrHttpFatal.Error()
	}
}

func HttpStatusFromErrs(err error) ServiceView {
	var errs map[string]error
	var merr *errmap.ErrorMap
	if ok := errors.As(err, &merr); ok {
		errs = merr.Errors
	} else {
		status, errStr := HttpStatusFromErr(err)
		return ServiceView{Status: status, Errors: errStr}
	}

	errStatus := 0
	errStrs := make(map[string]string)

	for key, err := range errs {
		status, errStr := HttpStatusFromErr(err)
		// yields the most 'severe' status. 500 is worse than 400, which is worse than 200
		if status > errStatus {
			errStatus = status
		}
		errStrs[key] = errStr
	}

	if errStatus == 0 {
		errStatus = http.StatusInternalServerError
	}
	return ServiceView{Status: errStatus, Errors: errStrs}
}

func parseJSON[B any](r *http.Request, body *B, validate func(B) error) error {
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

func transformJSON[B any, O any](r *http.Request, transform func(B) (O, error)) (O, error) {
	var body B
	err := json.NewDecoder(r.Body).Decode(&body)
	defer r.Body.Close()
	if err != nil {
		var o O
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
		slog.Error("failed to marshal json response", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(v); err != nil {
		slog.Error("failed towrite json response", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeBytes(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		slog.Error("internal server error", "err", err)
	}
}

func intQuery(values url.Values, key string) (int, error) {
	return strconv.Atoi(values.Get(key))
}

func intQueryDefault(values url.Values, key string, def int) (int, error) {
	v := values.Get(key)
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func queryDefault(values url.Values, key, def string) string {
	v := values.Get(key)
	if v == "" {
		return def
	}
	return v
}
