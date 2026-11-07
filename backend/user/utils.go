package user

import (
	"time"

	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/exp/constraints"
)

func average[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func findMostRecentS3Key(objects []s3Types.Object) string {
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

func filterLeastRecentS3Keys(objects []s3Types.Object) []s3Types.ObjectIdentifier {
	mostRecentKey := findMostRecentS3Key(objects)

	keys := make([]s3Types.ObjectIdentifier, 0)
	for _, obj := range objects {
		if obj.Key == nil || *obj.Key == mostRecentKey {
			continue
		}
		keys = append(keys, s3Types.ObjectIdentifier{Key: obj.Key})
	}

	return keys
}

func keysOfObjects(objects []s3Types.Object) []string {
	var s []string
	for _, o := range objects {
		if o.Key == nil {
			continue
		}
		s = append(s, *o.Key)
	}
	return s
}

func keysOfObjectIDs(objects []s3Types.ObjectIdentifier) []string {
	var s []string
	for _, o := range objects {
		if o.Key == nil {
			continue
		}
		s = append(s, *o.Key)
	}
	return s
}
