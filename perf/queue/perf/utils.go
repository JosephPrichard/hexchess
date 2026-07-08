package perf

import (
	crand "crypto/rand"
	"math/big"
	mrand "math/rand"
)

func randChar(str string) byte {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(len(str))))
	if err != nil {
		panic("failed to generate random number: " + err.Error())
	}
	return str[n.Int64()]
}

func randRange(low int64, high int64) int64{
	return int64(mrand.Intn(int(high))) + low
}
