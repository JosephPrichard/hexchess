package api

import (
	"encoding/json"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"log/slog"
	"net/http"
)

const defaultPaginationCount = 25

func LevelFromStatus(status int) slog.Level {
	level := slog.LevelInfo
	if status == http.StatusInternalServerError {
		level = slog.LevelError
	} else if status < 200 || status >= 300 {
		level = slog.LevelWarn
	}
	return level
}

var trans ut.Translator

func init() {
	locale := en.New()
	uni := ut.New(locale, locale)
	trans, _ = uni.GetTranslator("en")

	validate = validator.New()
	enTranslations.RegisterDefaultTranslations(validate, trans)
}

func parseJSON[Body any](r *http.Request, body *Body) error {
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ErrHttpInvalidJSON
	}

	return doValidation(body)
}

func transformJSON[Body any, Output any](r *http.Request, parse func(Body) (Output, error)) (Output, error) {
	defer r.Body.Close()

	var body Body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var o Output
		return o, ErrHttpInvalidJSON
	}

	return parse(body)
}

type ServiceView struct {
	Status  int                 `json:"status"`
	Message string              `json:"message"`
	Error   string              `json:"error,omitempty"`
	Errors  map[string]OneError `json:"errors,omitempty"`
}

type OneError struct {
	Message string `json:"message"`
	Error   string `json:"error"`
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
