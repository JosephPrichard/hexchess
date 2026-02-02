package ext

const S3ReplayBucket = "hexchess-replays"
const S3ProfileBucket = "hexchess-profiles"

type RemoteAPIs struct {
	GoogleAPI
}

func MakeRemoteAPIs() RemoteAPIs {
	return RemoteAPIs{
		GoogleAPI: &RemoteGoogleAPI{},
	}
}
