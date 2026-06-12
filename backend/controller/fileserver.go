package controller

import (
	"errors"
	"hexchess-svc/assets"
	"hexchess-svc/lib/serrors"
	svc "hexchess-svc/service"
	"log/slog"
	"net/http"
)

func (api *API) HandleUploadProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return serrors.New("get session player", err)
	}

	contentChecksum := r.Header.Get("Content-Digest")
	contentType := r.Header.Get("Content-Type")

	uploadResp, err := api.services.UploadProfilePic(ctx, player, r.Body, contentType, contentChecksum)
	if err != nil {
		return serrors.New("upload profile pic", err)
	}
	slog.InfoContext(ctx, "uploaded profile pic", "uploadResp", uploadResp)

	writeJSON(w, http.StatusOK, uploadResp)
	return nil
}

func (api *API) HandleGetProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := r.URL.Query().Get("userId")

	key, err := api.services.GetProfilePicKey(ctx, userID)
	if err != nil {
		level := slog.LevelError
		if errors.Is(err, svc.ErrNoProfilePic) {
			level = slog.LevelWarn
		}
		slog.Log(ctx, level, "failed to get profile pic key for user", "userID", userID, "error", err)

		_, err := w.Write(assets.DefaultProfilePic)
		return err
	}

	s3URL := api.services.MakeProfileURL(key)
	slog.InfoContext(ctx, "resolved user key to S3 profile pic URL", "url", s3URL, "userID", userID)

	// cache control is for what URL is being redirected to, this only changes if the user uploads a new profile pic
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	http.Redirect(w, r, s3URL, http.StatusTemporaryRedirect)
	return nil
}
