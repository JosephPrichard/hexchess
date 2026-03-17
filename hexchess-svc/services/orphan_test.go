package svc

import (
	"context"
	"fmt"
	"testing"

	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRemoveOrphanedBucketObjects(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.ROPostgres, itest.Aws)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	inputProfileKeys := []string{
		fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString()),
		fmt.Sprintf("users/profile-pics/2/%s", uuid.NewString()),
		fmt.Sprintf("users/profile-pics/3/%s", uuid.NewString()),
		fmt.Sprintf("users/profile-pics/8000/%s", uuid.NewString()),
		fmt.Sprintf("users/profile-pics/9000/%s", uuid.NewString()),
	}

	for _, id := range inputProfileKeys {
		egress.PutTestS3Object(t, services.AWS.S3Client, services.AWS.S3ProfileBucket, id, []byte("test"))
	}

	// when
	services.ClearBucketOrphans(ctx, 2)

	// then
	profileKeys := egress.ListTestS3Objects(t, services.AWS.S3Client, services.AWS.S3ProfileBucket)

	assertKeyContainment := func(actual []string, shouldContain []string, shouldNotContain []string) {
		t.Helper()
		for _, key := range shouldContain {
			assert.Contains(t, actual, key)
		}
		for _, key := range shouldNotContain {
			assert.NotContains(t, actual, key)
		}
	}
	assertKeyContainment(profileKeys, inputProfileKeys[:3], inputProfileKeys[3:])
}
