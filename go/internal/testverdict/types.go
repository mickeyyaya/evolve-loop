package testverdict

import (
	"errors"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
)

type Outcome string

const (
	Pass    Outcome = "pass"
	Fail    Outcome = "fail"
	Timeout Outcome = "timeout"
)

var (
	ErrNotGreen = errors.New("testverdict: only a pass is stored")
	ErrBadKey   = errors.New("testverdict: key is not a 64 digit hex digest of its parts")

	ErrInconsistent = errors.New("testverdict: the log of the run does not match the reads that the key holds")
)

type Record struct {
	Package  string
	Outcome  Outcome
	Key      testkey.Key
	Log      testkey.Log
	StoredAt time.Time
}

type Store struct {
	root string
	now  func() time.Time
}

func NewStore(root string, now func() time.Time) *Store {
	return &Store{root: root, now: now}
}
