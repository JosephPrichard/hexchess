package file

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/model"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/ioutil"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const ProfilePicPrefix = "users/profile-pics"

type ProfileService struct {
	aws        cloud.AWSClient
	dispatcher async.Dispatcher
	entropy    entropy.Generator
}

func NewProfileService(aws cloud.AWSClient, dispatcher async.Dispatcher, entropy entropy.Generator) *ProfileService {
	return &ProfileService{aws: aws, dispatcher: dispatcher, entropy: entropy}
}

func (services *ProfileService) deleteExpiredProfilePics(ctx context.Context, playerID int) error {
	prefix := fmtProfilePicPrefix(strconv.Itoa(playerID))

	// remove all but the newest keys. there should never be more 1000 keys, but if there are, this will never delete the newest Key
	listOutput, err := services.aws.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(services.aws.S3ProfileBucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return serrors.New("list profile pics by prefix", err, "prefix", prefix, "bucket", services.aws.S3ProfileBucket)
	}

	slog.InfoContext(ctx, "listed profile pics for deletion", "prefix", prefix,
		"bucket", services.aws.S3ProfileBucket, "keys", keysOfObjects(listOutput.Contents))

	if len(listOutput.Contents) == 0 {
		return nil
	}
	objectIdentifiers := filterLeastRecentKeys(listOutput.Contents)
	keys := keysOfObjectIDs(objectIdentifiers)

	slog.InfoContext(ctx, "deleting profile pics", "keys", keys)

	if len(objectIdentifiers) > 0 {
		if _, err := services.aws.S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(services.aws.S3ProfileBucket),
			Delete: &s3Types.Delete{Objects: objectIdentifiers},
		}); err != nil {
			return serrors.New("delete profile pics by keys", err, "keys", keys, "bucket", services.aws.S3ProfileBucket)
		}
	}
	return nil
}

type UploadProfileResult struct {
	Key string `json:"key"`
}

// MaxProfilePicSize 5 MiB
const MaxProfilePicSize = 5 << 20

var (
	InvalidChecksum     = errors.New("invalid or missing sha256 checksum")
	ErrProfilePicTooBig = fmt.Errorf("profile picture exceeds max size of %d bytes", MaxProfilePicSize)
)

func (services *ProfileService) UploadProfilePic(
	ctx context.Context,
	uploader model.PlayerState,
	file io.ReadCloser,
	contentType string,
	contentLength int64,
	contentChecksum string,
) (UploadProfileResult, error) {
	defer perf.WithContext(ctx).Log()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer file.Close()
	body := ioutil.NewLimitReader(cancel, file, MaxProfilePicSize)

	key := fmtProfilePicKey(uploader.ID, services.entropy.NewUUID().String())

	var checksumAlgorithm s3Types.ChecksumAlgorithm
	var checksumSHA256 *string
	var optFns []func(*s3.Options)

	if contentChecksum != "" {
		// if a checksum is provided, it will be automatically computed. (requires body to be seekeable OR an HTTPs request)
		optFns = append(optFns, s3.WithAPIOptions(
			// this middleware prevents the checksum from being computed, it is being passed from client in this case
			v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware,
		))
		checksumAlgorithm = s3Types.ChecksumAlgorithmSha256
		checksumSHA256 = aws.String(contentChecksum)
	}

	slog.InfoContext(ctx, "uploading profile pic",
		"key", key, "uploader", uploader, "contentType", contentType, "contentChecksum", contentChecksum)

	output, err := services.aws.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(services.aws.S3ProfileBucket),
		Key:               aws.String(key),
		ContentType:       aws.String(contentType),
		ContentLength:     aws.Int64(contentLength),
		Body:              body,
		ChecksumSHA256:    checksumSHA256,
		ChecksumAlgorithm: checksumAlgorithm,
		// with max cache control. profile pics are immutable, since we issue a new unique key on upload.
		CacheControl: aws.String("public, max-age=31536000"),
	}, optFns...)
	if body.HasExceededLimit() && errors.Is(err, context.Canceled) {
		return UploadProfileResult{}, ErrProfilePicTooBig
	}
	if err != nil {
		return UploadProfileResult{}, serrors.New("put profile pic", err, "key", key, "bucket", services.aws.S3ProfileBucket)
	}

	services.dispatcher.Go(func() {
		detachedCtx := context.WithoutCancel(ctx)
		if err := services.deleteExpiredProfilePics(detachedCtx, int(uploader.ID)); err != nil {
			slog.ErrorContext(detachedCtx, "failed to remove expired profile pics", "uploader", uploader, "error", err)
		}
	})

	slog.InfoContext(ctx, "finished uploading profile pic", "output", output)
	return UploadProfileResult{Key: key}, nil
}

var ErrNoProfilePic = errors.New("no profile pic found for user")

func (services *ProfileService) GetProfilePicURL(ctx context.Context, userID string) (string, error) {
	// retrieves all profile pictures for any user and retrieves the most recent one. this runs on the assumption that we may not be deleting old profile pics.
	prefix := fmtProfilePicPrefix(userID)

	listOutput, err := services.aws.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(services.aws.S3ProfileBucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return "", serrors.New("list profile pics by prefix", err, "prefix", prefix, "bucket", services.aws.S3ProfileBucket)
	}

	mostRecentKey := findMostRecentKey(listOutput.Contents)
	if mostRecentKey == "" {
		return "", ErrNoProfilePic
	}
	slog.InfoContext(ctx, "get profile pic by most recent key", "key", mostRecentKey, "userID", userID)

	presignOutput, err := services.aws.PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(services.aws.S3ProfileBucket),
		Key:    aws.String(mostRecentKey),
	})
	if err != nil {
		return "", serrors.New("presign profile pic url by key", err, "key", mostRecentKey, "bucket", services.aws.S3ProfileBucket)
	}

	s3URL := presignOutput.URL
	slog.InfoContext(ctx, "got profile pic presigned url by most recent key", "url", s3URL, "userID", userID)
	return s3URL, nil
}

func (services *ProfileService) NewProfileURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", services.aws.S3Endpoint, services.aws.S3ProfileBucket, key)
}

// s3 URL prefixes

func fmtProfilePicPrefix(userID string) string {
	return fmt.Sprintf("%s/%s", ProfilePicPrefix, userID)
}

func fmtProfilePicKey(userID int64, id string) string {
	return fmt.Sprintf("%s/%d/%s", ProfilePicPrefix, userID, id)
}

// s3 utilities

func ParseProfilePicKey(key string) (int64, error) {
	tokens := strings.Split(key, "/")
	if len(tokens) != 4 {
		return 0, fmt.Errorf("invalid profile Key, incorrect number of tokens: %s", key)
	}
	userID, err := strconv.ParseInt(tokens[2], 10, 64)
	if err != nil {
		return 0, serrors.New("profile Key userID is not a valid integer", err, "key", key)
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
