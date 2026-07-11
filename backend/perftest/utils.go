package perftest

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

func randRange[T interface{ ~int64 | ~int }](low T, high T) T{
	return T(mrand.Intn(int(high))) + low // range (low, high)
}
