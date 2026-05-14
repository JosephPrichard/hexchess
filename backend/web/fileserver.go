package web

import (
	"errors"
	"fmt"
	"hexchess-svc/assets"
	svc "hexchess-svc/service"
	"io"
	"log/slog"
	"net/http"
)

// MaxProfilePicSize 5 MiB
const MaxProfilePicSize = 5 << 20

func (api *API) HandleUploadProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	player, err := api.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	contentType := r.Header.Get("Content-Type")

	bodyFile := io.LimitReader(r.Body, MaxProfilePicSize)
	defer r.Body.Close()

	key, err := api.services.UploadProfilePic(ctx, player, bodyFile, contentType)
	if err != nil {
		return fmt.Errorf("upload profile pic: %w", err)
	}
	if err := api.services.DeleteOldProfilePics(ctx, int(player.ID)); err != nil {
		slog.ErrorContext(ctx, "failed to remove old profile pics", "error", err)
	}

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: key})
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

		_, Err := w.Write(assets.DefaultProfilePic)
		return Err
	}

	s3URL := api.services.MakeProfileURL(key)
	slog.InfoContext(ctx, "resolved user key to S3 profile pic URL", "url", s3URL, "userID", userID)

	// cache control is for what URL is being redirected to, this only changes if the user uploads a new profile pic
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	http.Redirect(w, r, s3URL, http.StatusTemporaryRedirect)
	return nil
}
