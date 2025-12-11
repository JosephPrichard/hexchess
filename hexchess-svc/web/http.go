package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

type ServiceView struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func writeJSON[V any](w http.ResponseWriter, status int, data V) {
	v, err := json.Marshal(data)
	if err != nil {
		slog.Error("marshal json response", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(v); err != nil {
		slog.Error("write json response", "err", err)
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

func parseIntQuery(ctx context.Context, query url.Values, key string) (int, error) {
	str := query.Get(key)
	num, err := strconv.Atoi(str)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse integer query", "key", key, "str", str, "err", err)
		return 0, ErrHttpInvalidRequest
	}
	return num, nil
}

func parsePageQuery(ctx context.Context, query url.Values) (int, error) {
	strPage := query.Get("page")
	if strPage == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(strPage)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse page query", "page", strPage, "err", err)
		return 0, ErrHttpInvalidRequest
	}
	return page, nil
}

func parseCountQuery(ctx context.Context, query url.Values) (int, error) {
	strCount := query.Get("count")
	if strCount == "" {
		return PerPage, nil
	}
	count, err := strconv.Atoi(strCount)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse count query", "count", strCount, "err", err)
		return 0, ErrHttpInvalidRequest
	}
	return count, nil
}
