package testkey

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strconv"
)

type encoder struct {
	h hash.Hash
}

func newEncoder() *encoder {
	return &encoder{h: sha256.New()}
}

func (e *encoder) str(s string) {
	e.h.Write([]byte(strconv.Itoa(len(s)) + ":" + s))
}

func (e *encoder) field(name, value string) {
	e.str(name)
	e.str(value)
}

func (e *encoder) list(name string, xs []string) {
	e.str(name)
	e.str(strconv.Itoa(len(xs)))
	for _, x := range xs {
		e.str(x)
	}
}

func (e *encoder) sum() string {
	return hex.EncodeToString(e.h.Sum(nil))
}
