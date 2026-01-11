package svc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"testing"
)

func TestDeleteOldProfilePics(t *testing.T) {
	// given
	state := SetupStateTest(t, itest.WithAws)
	defer state.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	key1 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())
	key2 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())
	key3 := fmt.Sprintf("users/profile-pics/2/%s", uuid.NewString())
	ext.PutS3Object(t, state.S3Client, state.S3ProfileBucket, key1, []byte("testfiledata1"))
	ext.PutS3Object(t, state.S3Client, state.S3ProfileBucket, key2, []byte("testfiledata2"))
	ext.PutS3Object(t, state.S3Client, state.S3ProfileBucket, key3, []byte("testfiledata3"))

	// when
	require.NoError(t, state.DeleteOldProfilePics(ctx, 1))

	// then
	assert.False(t, ext.S3ObjectExists(t, state.S3Client, state.S3ProfileBucket, key1))
	assert.Equal(t, "testfiledata2", ext.GetS3Object(t, state.S3Client, state.S3ProfileBucket, key2))
}
