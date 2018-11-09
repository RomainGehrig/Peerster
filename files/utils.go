package files

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

func toHash(hashValue []byte) (ret SHA256_HASH, err error) {
	if len(hashValue) != sha256.Size {
		return ret, errors.New(fmt.Sprint("Cannot convert byte array of length ", len(hashValue), " to a hash of length ", sha256.Size))
	}
	copy(ret[:], hashValue)
	return ret, nil
}
