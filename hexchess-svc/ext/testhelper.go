package ext

import (
	"bytes"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func PutS3Object(t *testing.T, s3Client *s3.Client, bucket string, key string, b []byte) {
	t.Helper()
	_, err := s3Client.PutObject(t.Context(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(b),
	})
	require.NoError(t, err)
}

func GetS3Object(t *testing.T, s3Client *s3.Client, bucket string, key string) string {
	t.Helper()
	t.Logf("getting s3 object by key '%s'", key)

	object, err := s3Client.GetObject(t.Context(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	require.NoError(t, err)
	defer object.Body.Close()

	b, err := io.ReadAll(object.Body)
	require.NoError(t, err)
	return string(b)
}

func S3ObjectExists(t *testing.T, s3Client *s3.Client, bucket string, key string) bool {
	t.Helper()
	t.Logf("checking s3 object by key '%s'", key)

	object, err := s3Client.GetObject(t.Context(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false
	}
	defer object.Body.Close()
	return true
}
