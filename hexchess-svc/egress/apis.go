package egress

type RemoteAPIs struct {
	GoogleAPI
}

func MakeRemoteAPIs() RemoteAPIs {
	return RemoteAPIs{
		GoogleAPI: MakeGoogleAPI("", nil),
	}
}
