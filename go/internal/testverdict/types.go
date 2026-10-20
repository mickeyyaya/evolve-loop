package testverdict

import (
	"errors"
	"path/filepath"
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
	ErrNoGate   = errors.New("testverdict: a record needs a gate id of lowercase letters, digits and dashes")

	ErrInconsistent = errors.New("testverdict: the log of the run does not match the reads that the key holds")
)

type Record struct {
	Package  string
	Outcome  Outcome
	Key      testkey.Key
	Log      testkey.Log
	Gate     string
	StoredAt time.Time
}

type Stats struct {
	Files   int
	Entries int
	Bytes   int64
	ByGate  map[string]int
}

type Store struct {
	root string
	now  func() time.Time
}

func DirFor(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "landing", "testcache")
}

func NewStore(root string, now func() time.Time) *Store {
	return &Store{root: root, now: now}
}
