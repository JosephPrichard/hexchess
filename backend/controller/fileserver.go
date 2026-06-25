package controller

import (
	"hexchess-svc/assets"
	"hexchess-svc/lib/serrors"
	"log/slog"
	"net/http"
	"strconv"
)

func (api *API) HandleUploadProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return serrors.Wrap("get session player", err)
	}

	contentChecksum := r.Header.Get("Content-Digest")
	contentLength := r.Header.Get("Content-Length")
	contentType := r.Header.Get("Content-Type")

	contentLengthInt64, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return respError("contentLength", err)
	}

	uploadResp, err := api.services.UploadProfilePic(ctx, player, r.Body, contentType, contentLengthInt64, contentChecksum)
	if err != nil {
		return serrors.Wrap("upload profile pic", err)
	}
	slog.InfoContext(ctx, "uploaded profile pic", "uploadResp", uploadResp)

	writeJSON(w, http.StatusOK, uploadResp)
	return nil
}

func (api *API) HandleGetProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := r.URL.Query().Get("userId")

	s3URL, err := api.services.GetProfilePicURL(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "failed to get profile pic key for user", "userID", userID, "error", err)

		_, err := w.Write(assets.DefaultProfilePic)
		return err
	}
	// cache control is for what URL is being redirected to; this only changes if the user uploads a new profile pic
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	http.Redirect(w, r, s3URL, http.StatusTemporaryRedirect)
	return nil
}
