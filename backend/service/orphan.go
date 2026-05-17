package svc

import (
	"context"
	"fmt"
	"hexchess-svc/egress"
	"log/slog"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const PageLength = 1000

func (services *HexchessServices) ClearOrphanFiles(ctx context.Context, pageLength int32) {
	var wg sync.WaitGroup

	for _, config := range []RemoveOrphansOpts{
		{
			Bucket:     egress.S3ProfileBucket,
			Prefix:     ProfilePicPrefix,
			PageLength: pageLength,
			parseID:    ParseProfilePicKey,
			selectIDs:  services.querier.SelectExistsUsersByIDs,
		},
	} {
		wg.Go(func() {
			if err := services.removeOrphanedObjects(ctx, config); err != nil {
				slog.ErrorContext(ctx, "failed to remove orphaned objects", "config", config, "error", err)
			}
		})
	}

	wg.Wait()
}

type RemoveOrphansOpts struct {
	Bucket     string
	Prefix     string
	PageLength int32
	parseID    func(string) (int64, error)
	selectIDs  func(context.Context, []int64) ([]int64, error)
}

// removeOrphanedObjects is a generic algorithm to delete any orphaned keys by paginating all keys in a bucket
// it assumes that we can parse the existingID from any given key, and that we can lookup if that key is valid or not from a database.
func (services *HexchessServices) removeOrphanedObjects(ctx context.Context, opts RemoveOrphansOpts) error {
	page := 0

	paginator := s3.NewListObjectsV2Paginator(services.aws.S3Client, &s3.ListObjectsV2Input{
		Bucket:  aws.String(opts.Bucket),
		Prefix:  aws.String(opts.Prefix),
		MaxKeys: aws.Int32(opts.PageLength),
	})

	// paginates through all keys in the Bucket page by page, collects each key, checks if they are orphaned or not, and deletes orphans.
	for paginator.HasMorePages() {
		// list keys for this page.
		listObjects, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}
		page++

		// parse and extract object IDQueue and keys from api call.
		type KeyPair struct {
			key      string
			objectID int64
		}
		var ids []int64
		var pairs []KeyPair
		for _, obj := range listObjects.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key
			objectID, err := opts.parseID(key)
			if err != nil {
				slog.ErrorContext(ctx, "failed to parse object Key", "key", key, "error", err)
				continue
			}
			ids = append(ids, objectID)
			pairs = append(pairs, KeyPair{key: key, objectID: objectID})
		}

		slog.InfoContext(ctx, "listed objects", "listOutput", pairs, "pageLength", opts.PageLength,
			"page", page, "bucket", opts.Bucket, "prefix", opts.Prefix)

		// find orphaned keys, and store them in a map.
		validIDs, err := opts.selectIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("select valid object IDQueue: %w", err)
		}
		existingObjectIDs := make(map[int64]bool, len(validIDs))
		for _, objectID := range validIDs {
			existingObjectIDs[objectID] = true
		}

		slog.InfoContext(ctx, "selected valid object IDQueue", "objectIDs", validIDs, "bucket", opts.Bucket)

		var orphanedKeyStrs []string
		var orphanedKeys []s3Types.ObjectIdentifier
		for _, pair := range pairs {
			if !existingObjectIDs[pair.objectID] {
				orphanedKeyStrs = append(orphanedKeyStrs, pair.key)
				orphanedKeys = append(orphanedKeys, s3Types.ObjectIdentifier{Key: aws.String(pair.key)})
			}
		}
		if len(orphanedKeys) == 0 {
			continue
		}

		// delete orphaned keys from bucket.
		// we could make this a background goroutine, but since latency does not matter (this is background job), we keep it sync for simplicity
		slog.InfoContext(ctx, "deleting orphaned keys", "keys", orphanedKeyStrs, "bucket", opts.Bucket)

		if _, err := services.aws.S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(opts.Bucket),
			Delete: &s3Types.Delete{Objects: orphanedKeys},
		}); err != nil {
			slog.ErrorContext(ctx, "failed to delete orphaned keys", "keys", orphanedKeys, "error", err)
		}
	}
	return nil
}
