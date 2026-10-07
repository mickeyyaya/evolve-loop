// Package ledgerartifacts is the ledger's write-once, content-addressed evidence store.
// See docs/architecture/packages/internal-ledgerartifacts.md.
package ledgerartifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const DirName = "ledger-artifacts"

const (
	objectsDirName   = "sha256"
	digestLen        = sha256.Size * 2
	fanOutLen        = 2
	objectMode       = 0o444
	legacyTempPrefix = ".put-"
)

var (
	errCorrupt   = errors.New("ledgerartifacts: stored bytes no longer hash to their name")
	errBadDigest = errors.New("ledgerartifacts: not a lowercase sha256 hex digest")
)

type Store struct {
	root string
}

func Open(evolveDir string) Store {
	return Store{root: filepath.Join(evolveDir, DirName)}
}

func (s Store) Path(digest string) (string, error) {
	if !isDigest(digest) {
		return "", fmt.Errorf("%w: %q", errBadDigest, digest)
	}
	return s.fannedOut(digest), nil
}

func (s Store) Put(body []byte) (string, error) {
	digest := Digest(body)
	path := s.fannedOut(digest)
	if existing, err := os.ReadFile(path); err == nil && Digest(existing) == digest {
		return digest, nil
	}
	if err := reapTempsOfGoneWriters(filepath.Dir(path)); err != nil {
		return "", err
	}
	if err := writeAtomically(path, body); err != nil {
		return "", err
	}
	return digest, nil
}

func (s Store) Get(digest string) ([]byte, error) {
	path, err := s.Path(digest)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ledgerartifacts: %w", err)
	}
	if Digest(body) != digest {
		return nil, fmt.Errorf("%w: %s", errCorrupt, path)
	}
	return body, nil
}

func (s Store) fannedOut(digest string) string {
	return filepath.Join(s.root, objectsDirName, digest[:fanOutLen], digest[fanOutLen:])
}

func writeAtomically(path string, body []byte) error {
	if err := atomicwrite.Durable(path, body, objectMode); err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	return nil
}

func reapTempsOfGoneWriters(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	for _, e := range entries {
		if pid, ok := tempWriter(e.Name()); ok && writerGone(pid) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

func tempWriter(name string) (pid int, ok bool) {
	rest, ok := strings.CutPrefix(name, legacyTempPrefix)
	if !ok {
		return atomicwrite.TempWriter(name)
	}
	pidText, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(pidText)
	return pid, err == nil
}

func writerGone(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

func Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func isDigest(s string) bool {
	if len(s) != digestLen {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
