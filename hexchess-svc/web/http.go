package web

import (
	"encoding/json"
	"hexchess-svc/util"
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
	if FatalErrorJSON, err = json.Marshal(ServiceView{Status: 500, Message: ErrHttpFatal.Error()}); err != nil {
		util.LogFatal("failed to marshal fatal error service view", "err", err)
	}
	if NotFoundErrorJSON, err = json.Marshal(ServiceView{Status: 404, Message: "route not found"}); err != nil {
		util.LogFatal("failed to marshal not found service view", "err", err)
	}
	if SuccessJSON, err = json.Marshal(ServiceView{Status: 200, Message: "SUCCESS"}); err != nil {
		util.LogFatal("failed to marshal success service view", "err", err)
	}
	if EmptyRefreshJSON, err = json.Marshal(RefreshResp{Session: nil}); err != nil {
		util.LogFatal("failed to marshal refresh resp", "err", err)
	}
}

func writeSuccessJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(SuccessJSON)
}

func writeEmptyRefreshJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(EmptyRefreshJSON)
}

func writeJSON[V any](w http.ResponseWriter, status int, data V) {
	v, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(v)
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
