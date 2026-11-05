package integration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const (
	epochFileName = "integration-epoch"
	epochLockName = "integration-epoch.lock"
)

type EpochReader interface {
	Current() (uint64, error)
}

type EpochHolder struct {
	dir   string
	write func(string, []byte) error
	once  sync.Once
	value uint64
	err   error
}

func NewEpochHolder(evolveDir string) *EpochHolder {
	return &EpochHolder{dir: evolveDir, write: atomicwrite.Bytes}
}

func EpochPath(evolveDir string) string { return filepath.Join(evolveDir, epochFileName) }

func (h *EpochHolder) Claim() (uint64, error) {
	h.once.Do(func() { h.value, h.err = h.increment() })
	return h.value, h.err
}

func (h *EpochHolder) Current() (uint64, error) { return readEpoch(EpochPath(h.dir)) }

func (h *EpochHolder) increment() (uint64, error) {
	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		return 0, fmt.Errorf("integration: create the epoch directory: %w", err)
	}
	release, err := flock.Lock(filepath.Join(h.dir, epochLockName))
	if err != nil {
		return 0, fmt.Errorf("integration: lock the epoch file: %w", err)
	}
	defer release()
	cur, err := readEpoch(EpochPath(h.dir))
	if err != nil {
		return 0, err
	}
	next := cur + 1
	if err := h.write(EpochPath(h.dir), []byte(strconv.FormatUint(next, 10)+"\n")); err != nil {
		return 0, fmt.Errorf("integration: write the epoch file: %w", err)
	}
	return next, nil
}

func readEpoch(path string) (uint64, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("integration: read the epoch file: %w", err)
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("integration: epoch file %s holds %q: %w", path, strings.TrimSpace(string(raw)), err)
	}
	return n, nil
}
