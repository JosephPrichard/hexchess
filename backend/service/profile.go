package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/egress"
	"hexchess-svc/model"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func ParseProfilePicKey(key string) (int64, error) {
	tokens := strings.Split(key, "/")
	if len(tokens) != 4 {
		return 0, fmt.Errorf("invalid profile Key, incorrect number of tokens: %s", key)
	}
	userID, err := strconv.ParseInt(tokens[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("profile Key userID is not a valid integer: %s: %w", key, err)
	}
	return userID, nil
}

func findMostRecentKey(objects []s3Types.Object) string {
	var mostRecentKey string
	mostRecentCreTime := time.Time{}
	for _, obj := range objects {
		if obj.Key == nil || obj.LastModified == nil {
			continue
		}
		if obj.LastModified.After(mostRecentCreTime) {
			mostRecentKey = *obj.Key
			mostRecentCreTime = obj.LastModified.UTC()
		}
	}
	return mostRecentKey
}

func filterLeastRecentKeys(objects []s3Types.Object) []s3Types.ObjectIdentifier {
	mostRecentKey := findMostRecentKey(objects)

	keys := make([]s3Types.ObjectIdentifier, 0)
	for _, obj := range objects {
		if obj.Key == nil || *obj.Key == mostRecentKey {
			continue
		}
		keys = append(keys, s3Types.ObjectIdentifier{Key: obj.Key})
	}

	return keys
}

const ProfilePicPrefix = "users/profile-pics"

func (services *HexchessServices) DeleteOldProfilePics(ctx context.Context, playerID int) error {
	prefix := makeProfilePicPrefix(strconv.Itoa(playerID))

	// remove all but the newest keys. there should never be more 1000 keys, but if there are, this will never delete the newest Key
	listOutput, err := services.aws.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(egress.S3ProfileBucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return fmt.Errorf("list profile pics by prefix=%s: from s3 bucket: %s: %w", prefix, egress.S3ProfileBucket, err)
	}
	slog.InfoContext(ctx, "listed profile pics for deletion", "listOutput", listOutput.Contents)

	if len(listOutput.Contents) == 0 {
		return nil
	}
	keys := filterLeastRecentKeys(listOutput.Contents)
	slog.InfoContext(ctx, "deleting profile pics", "keys", keys)

	if _, err := services.aws.S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(egress.S3ProfileBucket),
		Delete: &s3Types.Delete{Objects: keys},
	}); err != nil {
		return fmt.Errorf("delete profile pics by keys %v: from s3 Bucket: %s: %w", keys, egress.S3ProfileBucket, err)
	}
	return nil
}

func (services *HexchessServices) UploadProfilePic(ctx context.Context, uploader model.PlayerState, file io.Reader, contentType string) (string, error) {
	// uploading profile picture based off a computed Key
	key := makeProfileNewPicKey(uploader.ID, services.entropy.MakeUUID())

	slog.InfoContext(ctx, "uploading profile pic to s3", "key", key, "player", uploader)
	start := time.Now()

	putOutput, err := services.aws.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(egress.S3ProfileBucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
		// with max cache control. profile pics are immutable, since we issue a new unique Key on upload.
		CacheControl: aws.String("public, max-age=31536000"),
	})
	if err != nil {
		return "", fmt.Errorf("put profile pic %s: to s3 bucket: %s: %w", key, egress.S3ProfileBucket, err)
	}

	slog.InfoContext(ctx, "finished uploading profile pic to s3", "key", key, "took", time.Since(start), "player", uploader, "output", putOutput)
	return key, nil
}

var ErrNoProfilePic = errors.New("no profile pic found for user")

func (services *HexchessServices) GetProfilePicKey(ctx context.Context, userID string) (string, error) {
	// retrieves all profile pictures for any user and retrieves the most recent one. this runs on the assumption that we may not be deleting old profile pics.
	prefix := makeProfilePicPrefix(userID)

	listOutput, err := services.aws.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(egress.S3ProfileBucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return "", fmt.Errorf("list profile pics by prefix=%s: from s3 bucket: %s: %w", prefix, egress.S3ProfileBucket, err)
	}

	mostRecentKey := findMostRecentKey(listOutput.Contents)
	if mostRecentKey == "" {
		return "", ErrNoProfilePic
	}
	slog.InfoContext(ctx, "constructed profile pic", "key", mostRecentKey)
	return mostRecentKey, nil
}

func (services *HexchessServices) MakeProfileURL(key string) string {
	return services.aws.MakeS3Url(egress.S3ProfileBucket, key)
}
