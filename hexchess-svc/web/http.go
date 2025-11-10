package web

import (
	"encoding/json"
	"hexchess-svc/util"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

type ServiceView struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

var SuccessJSON []byte
var FatalErrorJSON []byte
var NotFoundErrorJSON []byte
var EmptyRefreshJSON []byte

func init() {
	var err error
	if SuccessJSON, err = json.Marshal(ServiceView{Status: 200, Message: "SUCCESS"}); err != nil {
		util.LogFatalErr("failed to marshal success service debug", err)
	}
	if FatalErrorJSON, err = json.Marshal(ServiceView{Status: 500, Message: ErrHttpFatal.Error()}); err != nil {
		util.LogFatalErr("failed to marshal fatal error service debug", err)
	}
	if NotFoundErrorJSON, err = json.Marshal(ServiceView{Status: 404, Message: "route not found"}); err != nil {
		util.LogFatalErr("failed to marshal not found service debug", err)
	}
	if EmptyRefreshJSON, err = json.Marshal(RefreshResp{Session: nil}); err != nil {
		util.LogFatalErr("failed to marshal refresh resp", err)
	}
}

func writeSuccessJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(SuccessJSON); err != nil {
		slog.Error("failed to write json response", "err", err)
	}
}

func writeEmptyRefreshJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(EmptyRefreshJSON); err != nil {
		slog.Error("failed to write json response", "err", err)
	}
}

func writeJSON[V any](w http.ResponseWriter, status int, data V) {
	v, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(v); err != nil {
		slog.Error("failed to write json response", "err", err)
	}
}

func writeBytes(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		slog.Error("failed to write binary response", "err", err)
	}
}

func getPageQuery(query url.Values) (int, error) {
	strPage := query.Get("page")
	if strPage == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(strPage)
	if err != nil {
		return 0, err
	}
	return page, nil
}

func getCountQuery(query url.Values) (int, error) {
	strCount := query.Get("count")
	if strCount == "" {
		return PerPage, nil
	}
	count, err := strconv.Atoi(strCount)
	if err != nil {
		return 0, err
	}
	return count, nil
}
