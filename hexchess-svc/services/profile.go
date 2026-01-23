package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"log/slog"
	"strconv"
	"time"
)

func MakeProfilePicPrefix(userID string) string {
	return fmt.Sprintf("%s/%s", ProfilePicPrefix, userID)
}

func MakeProfileNewPicKey(userID int64) string {
	return fmt.Sprintf("%s/%d/%s", ProfilePicPrefix, userID, uuid.NewString())
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

func (s *State) DeleteOldProfilePics(ctx context.Context, playerID int) error {
	prefix := MakeProfilePicPrefix(strconv.Itoa(playerID))

	// remove all but the newest keys. there should never be more 1000 keys, but if there are, this will never delete the newest key
	listOutput, err := s.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.S3ProfileBucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return fmt.Errorf("list profile pics by prefix=%s: from s3 bucket: %s: %w", prefix, s.S3ProfileBucket, err)
	}
	slog.InfoContext(ctx, "listed profile pics for deletion", "listOutput", listOutput.Contents)

	if len(listOutput.Contents) == 0 {
		return nil
	}
	keys := filterLeastRecentKeys(listOutput.Contents)
	slog.InfoContext(ctx, "deleting profile pics", "keys", keys)

	if _, err := s.S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(s.S3ProfileBucket),
		Delete: &s3Types.Delete{Objects: keys},
	}); err != nil {
		return fmt.Errorf("delete profile pics by keys %v: from s3 bucket: %s: %w", keys, s.S3ProfileBucket, err)
	}
	return nil
}

var ErrNoProfilePic = errors.New("no profile pic found for user")

func (s *State) GetProfilePicKey(ctx context.Context, userID string) (string, error) {
	// retrieves all profile pictures for any user and retrieves the most recent one. this runs on the assumption that we may not be deleting old profile pics.
	prefix := MakeProfilePicPrefix(userID)

	listOutput, err := s.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.S3ProfileBucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return "", fmt.Errorf("list profile pics by prefix=%s: from s3 bucket: %s: %w", prefix, s.S3ProfileBucket, err)
	}

	mostRecentKey := findMostRecentKey(listOutput.Contents)
	if mostRecentKey == "" {
		return "", ErrNoProfilePic
	}
	slog.InfoContext(ctx, "constructed profile pic", "key", mostRecentKey)
	return mostRecentKey, nil
}
