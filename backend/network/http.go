package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/bytedance/sonic"
	// "github.com/bytedance/sonic"
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

	// note(Joseph): sonic.Unmarshal fine relative to Decoder for small JSON body objects, and it provides much better error handling and UX

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return sonic.Unmarshal(bodyBytes, body)
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

func writeServiceResp(w http.ResponseWriter, view ServiceResp) {
	writeJSON(w, view.Status, view)
}

func writeBytes(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		slog.Error("internal server error", "Err", err)
	}
}
