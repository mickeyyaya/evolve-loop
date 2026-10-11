package envelope

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	crockford    = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	idChars      = 26
	entropyBytes = 10
	maxFirstChar = '7'
)

type IDSource struct {
	Now     func() time.Time
	Entropy io.Reader
}

func (s IDSource) New() (string, error) {
	var raw [16]byte
	ms := uint64(s.Now().UnixMilli())
	for i := 0; i < 6; i++ {
		raw[i] = byte(ms >> (40 - 8*i))
	}
	if _, err := io.ReadFull(s.Entropy, raw[6:6+entropyBytes]); err != nil {
		return "", fmt.Errorf("%w: entropy: %v", ErrID, err)
	}
	return encodeID(raw), nil
}

func encodeID(raw [16]byte) string {
	var hi, lo uint64
	for i := 0; i < 8; i++ {
		hi = hi<<8 | uint64(raw[i])
		lo = lo<<8 | uint64(raw[8+i])
	}
	out := make([]byte, idChars)
	for i := idChars - 1; i >= 0; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out)
}

func ValidID(id string) bool {
	if len(id) != idChars || id[0] > maxFirstChar {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune(crockford, c) {
			return false
		}
	}
	return true
}
