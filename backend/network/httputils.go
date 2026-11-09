package network

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
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
	if err := enTranslations.RegisterDefaultTranslations(validate, trans); err != nil {
		panic(fmt.Sprintf("register translations: %v", err))
	}
}

func decodeJson[Body any](r *http.Request, body *Body) error {
	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// note: it's safe to log a JSON request body, since we are assuming it is small enough to hold in memory - this is intentionally kept from the other loggers to avoid logging files
	slog.InfoContext(r.Context(), "received JSON request body", "request", string(bodyBytes))

	// note: json.Unmarshal fine relative to Decoder for small JSON body objects, and it provides much better error handling and UX
	return json.Unmarshal(bodyBytes, body)
}

func parseJSON[Body any](r *http.Request, body *Body) error {
	if err := decodeJson(r, body); err != nil {
		return ErrHttpInvalidJSON
	}
	return doValidation(body)
}

func mapJSON[Body any, Output any](r *http.Request, parse func(Body) (Output, error)) (o Output, _ error) {
	var body Body
	if err := decodeJson(r, &body); err != nil {
		return o, err
	}
	return parse(body)
}

type ServiceResp struct {
	Status  int                 `json:"status"`
	Message string              `json:"message,omitempty"`
	Error   string              `json:"error,omitempty"`
	Errors  map[string]OneError `json:"errors,omitempty"`
}

type OneError struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func writeJSON[V any](ctx context.Context, w http.ResponseWriter, status int, data V) {
	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal json response", "error", err)
		http.Error(w, "internal server error: failed to marshal response", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "writing JSON response body", "response", string(bytes), "status", status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(bytes); err != nil {
		slog.Error("failed to write json response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeServiceResp(ctx context.Context, w http.ResponseWriter, view ServiceResp) {
	writeJSON(ctx, w, view.Status, view)
}

func writeBytes(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		slog.Error("internal server error", "Err", err)
	}
}
