package file

import s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

func ptr[T any](v T) *T {
	return &v
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
