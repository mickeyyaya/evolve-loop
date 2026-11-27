package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
)

type ChainPort interface {
	Seal(ctx context.Context, pool int, digest string) error
	SealedAll(ctx context.Context) (map[int]string, error)
}

func RecordDigest(projectRoot string, pool int) (string, error) {
	raw, err := os.ReadFile(WaveRecordPath(projectRoot, pool))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func Chained(sealed map[int]string, projectRoot string, pool int) (bool, error) {
	want, found := sealed[pool]
	if !found {
		return false, nil
	}
	disk, err := RecordDigest(projectRoot, pool)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil && disk == want, err
}

type SealedWaveStore struct {
	Inner WaveStore
	Root  string
	Chain ChainPort
	Ctx   context.Context
}

func (s SealedWaveStore) sealContext() context.Context {
	if s.Ctx == nil {
		return context.Background()
	}
	return s.Ctx
}

func (s SealedWaveStore) Read(pool int) (WaveRecord, bool, error) { return s.Inner.Read(pool) }

func (s SealedWaveStore) Pools() []int { return s.Inner.Pools() }

var ErrNoChainPort = errors.New("integration: wave store has no chain port")

func (s SealedWaveStore) Write(rec WaveRecord) error {
	if s.Chain == nil {
		return ErrNoChainPort
	}
	if err := s.Inner.Write(rec); err != nil {
		return err
	}
	digest, err := RecordDigest(s.Root, rec.Pool)
	if err != nil {
		return err
	}
	return s.Chain.Seal(s.sealContext(), rec.Pool, digest)
}
