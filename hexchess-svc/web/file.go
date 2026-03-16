package web

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"hexchess-svc/assets"
	svc "hexchess-svc/services"
	"log/slog"
	"net/http"
	"time"
)

// MaxProfilePicSize 5 MiB
const MaxProfilePicSize = 5 << 20

func (server *Server) HandleUploadProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	player, _, err := GetSessionPlayer(ctx, server.Services, r)
	if err != nil {
		return fmt.Errorf("get session player: %w", err)
	}

	contentType := r.Header.Get("Content-Type")

	// the maximum memory we are using is the same as the max bytes reader to prevent writing temp files to disk
	r.Body = http.MaxBytesReader(w, r.Body, MaxProfilePicSize)

	if err := r.ParseMultipartForm(MaxProfilePicSize); err != nil {
		return fmt.Errorf("parse multipart form: %w", err)
	}
	defer func() {
		// this isn't necessary if MaxBytesReader = MaxMemory, but it will become necessary if we change that
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
	}()
	file, _, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("get file from form: %w", err)
	}
	defer file.Close()

	// uploading profile picture based off a computed key
	key := svc.MakeProfileNewPicKey(player.ID)
	slog.InfoContext(ctx, "uploading profile pic to s3", "key", key, "player", player)
	start := time.Now()

	putOutput, err := server.AWS.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(server.AWS.S3ProfileBucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
		// with max cache control. profile pics are immutable, since we issue a new unique key on upload.
		CacheControl: aws.String("public, max-age=31536000"),
	})
	if err != nil {
		return fmt.Errorf("put profile pic %s: to s3 bucket: %s: %w", key, server.AWS.S3ProfileBucket, err)
	}

	slog.InfoContext(ctx, "finished uploading profile pic to s3", "key", key, "took", time.Since(start), "player", player, "output", putOutput)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(ctx, "recovered from panic while deleting old profile pics", "error", err)
			}
		}()
		// removes old profile pictures on upload of a new profile pic, since retrieval function will always get the most recent file.
		if err := server.DeleteOldProfilePics(ctx, int(player.ID)); err != nil {
			slog.ErrorContext(ctx, "failed to remove old profile pics", "error", err)
		}
	}()

	writeJSON(w, http.StatusOK, ServiceView{Status: http.StatusOK, Message: key})
	return nil
}

func (server *Server) HandleGetProfilePic(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID := r.URL.Query().Get("userId")

	key, err := server.GetProfilePicKey(ctx, userID)
	if errors.Is(svc.ErrNoProfilePic, err) {
		w.Write(assets.DefaultProfilePic)
		return nil
	} else if err != nil {
		return fmt.Errorf("get profile pic key for user %s: %w", userID, err)
	}

	s3URL := server.AWS.MakeS3Url(server.AWS.S3ProfileBucket, key)
	slog.InfoContext(ctx, "resolved user ID to S3 profile pic URL", "url", s3URL, "userID", userID)

	// cache control is for what URL is being redirected to, this only changes if the user uploads a new profile pic
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	http.Redirect(w, r, s3URL, http.StatusTemporaryRedirect)
	return nil
}
