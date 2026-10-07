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
)

const DirName = "ledger-artifacts"

const (
	objectsDirName = "sha256"
	digestLen      = sha256.Size * 2
	fanOutLen      = 2
	objectMode     = 0o444
	tempPrefix     = ".put-"
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	if err := reapTempsOfGoneWriters(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, tempPattern(os.Getpid()))
	if err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	if err := placeObject(tmp, path, body); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

func placeObject(tmp *os.File, path string, body []byte) error {
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("ledgerartifacts: write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("ledgerartifacts: sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("ledgerartifacts: close %s: %w", path, err)
	}
	if err := os.Chmod(tmp.Name(), objectMode); err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("ledgerartifacts: %w", err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return fmt.Errorf("ledgerartifacts: sync %s: %w", dir, err)
	}
	return d.Close()
}

func reapTempsOfGoneWriters(dir string) error {
	entries, err := os.ReadDir(dir)
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

func tempPattern(pid int) string {
	return tempPrefix + strconv.Itoa(pid) + "-*"
}

func tempWriter(name string) (pid int, ok bool) {
	rest, ok := strings.CutPrefix(name, tempPrefix)
	if !ok {
		return 0, false
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
