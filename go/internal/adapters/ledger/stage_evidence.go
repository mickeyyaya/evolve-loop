package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	StageEvidenceSealKind = "stage-evidence-seal"
	StageEvidenceRole     = "stage-evidence"
)

type stageSeal struct {
	scope  stageevidence.ScopeKey
	digest string
	seal   stageevidence.Seal
}

func (l *FileLedger) SealStageEvidence(ctx context.Context, scope stageevidence.ScopeKey, in stageevidence.SealInput) (stageevidence.Seal, error) {
	evolveDir := filepath.Dir(l.ledgerPath)
	working, err := os.ReadFile(stageevidence.WorkingPath(evolveDir, scope))
	if errors.Is(err, os.ErrNotExist) {
		return stageevidence.Seal{}, stageevidence.ErrEmptyChain
	}
	if err != nil {
		return stageevidence.Seal{}, fmt.Errorf("ledger: read stage evidence of %s: %w", scope, err)
	}
	lines, err := l.gatherAllLines()
	if err != nil {
		return stageevidence.Seal{}, err
	}
	seals, err := sealsInOrder(lines)
	if err != nil {
		return stageevidence.Seal{}, fmt.Errorf("%w: %v", stageevidence.ErrLedgerBroken, err)
	}
	if prior, ok := latestOf(seals)[scope]; ok && in.Prior == nil {
		in.Prior = &prior.seal
	}
	seal, _, err := stageevidence.BuildSeal(scope, working, in)
	if err != nil {
		return stageevidence.Seal{}, err
	}
	digest, stored, err := l.evidence().PutAt(working)
	if err != nil {
		return stageevidence.Seal{}, fmt.Errorf("ledger: store stage evidence of %s: %w", scope, err)
	}
	rel := strings.TrimPrefix(stored, evolveDir+string(filepath.Separator))
	entry := core.LedgerEntry{
		TS: time.Now().UTC().Format(time.RFC3339), Role: StageEvidenceRole, Kind: StageEvidenceSealKind,
		CycleLabel: scope.String(), ArtifactPath: filepath.ToSlash(rel), ArtifactSHA256: digest, Message: seal.Encode(),
	}
	if scope.Kind == stageevidence.ScopeCycle {
		entry.Cycle = scope.ID
	}
	if err := l.Append(ctx, entry); err != nil {
		return stageevidence.Seal{}, fmt.Errorf("ledger: append stage evidence seal of %s: %w", scope, err)
	}
	return seal, nil
}

func latestOf(seals []stageSeal) map[stageevidence.ScopeKey]stageSeal {
	latest := map[stageevidence.ScopeKey]stageSeal{}
	for _, s := range seals {
		latest[s.scope] = s
	}
	return latest
}

func sealsInOrder(lines [][]byte) (all []stageSeal, parseErr error) {
	for _, line := range lines {
		if !bytes.Contains(line, []byte(`"`+StageEvidenceSealKind+`"`)) {
			continue
		}
		var e core.LedgerEntry
		if err := json.Unmarshal(line, &e); err != nil {
			parseErr = errors.Join(parseErr, fmt.Errorf("stage evidence seal line is not an entry: %w", err))
			continue
		}
		if e.Kind != StageEvidenceSealKind {
			continue
		}
		scope, ok := stageevidence.ParseScope(e.CycleLabel)
		seal, err := stageevidence.ParseSeal(e.Message)
		if !ok || err != nil {
			parseErr = errors.Join(parseErr, fmt.Errorf("stage evidence seal at entry %d is malformed", e.EntrySeq))
			continue
		}
		all = append(all, stageSeal{scope: scope, digest: e.ArtifactSHA256, seal: seal})
	}
	return all, parseErr
}

type StageChainReader struct {
	l     *FileLedger
	once  sync.Once
	chain *stageIndex
	err   error
}

func (l *FileLedger) StageChain() *StageChainReader {
	return &StageChainReader{l: l}
}

func (r *StageChainReader) ReadChain(_ context.Context, key string) (stageevidence.Chain, error) {
	r.once.Do(func() { r.chain, r.err = r.l.buildStageIndex() })
	if r.err != nil {
		return stageevidence.Chain{}, r.err
	}
	return r.chain.forKey(key)
}

type stageIndex struct {
	records     map[string][]stageevidence.Record
	outcomes    map[stageevidence.ScopeKey]stageevidence.Outcome
	keysByScope map[stageevidence.ScopeKey]map[string]bool
	order       []stageevidence.ScopeKey
	unsealed    []stageevidence.Outcome
	digest      string
}

func (l *FileLedger) buildStageIndex() (*stageIndex, error) {
	var segs []sealedSegment
	var live [][]byte
	var tip tipRead
	if err := l.readSnapshot(func() (err error) {
		segs, live, err = l.readChain()
		tip = l.readTipFile()
		return err
	}); err != nil {
		return nil, fmt.Errorf("%w: %v", stageevidence.ErrLedgerBroken, err)
	}
	if _, err := l.verifyChain(segs, live, tip.check); err != nil {
		return nil, fmt.Errorf("%w: %v", stageevidence.ErrLedgerBroken, err)
	}
	all, err := sealsInOrder(chainOrder(segs, live))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", stageevidence.ErrLedgerBroken, err)
	}
	idx := &stageIndex{
		records:     map[string][]stageevidence.Record{},
		outcomes:    map[stageevidence.ScopeKey]stageevidence.Outcome{},
		keysByScope: map[stageevidence.ScopeKey]map[string]bool{},
	}
	if err := idx.absorb(l.evidence(), all); err != nil {
		return nil, err
	}
	if err := idx.findUnsealed(filepath.Dir(l.ledgerPath), all); err != nil {
		return nil, err
	}
	return idx, nil
}

func (idx *stageIndex) absorb(store ledgerartifacts.Store, all []stageSeal) error {
	byScope := map[stageevidence.ScopeKey][]stageSeal{}
	for _, s := range all {
		byScope[s.scope] = append(byScope[s.scope], s)
		idx.order = removeScope(idx.order, s.scope)
		idx.order = append(idx.order, s.scope)
	}
	digest := sha256.New()
	for _, scope := range idx.order {
		history := byScope[scope]
		last := history[len(history)-1]
		stored, err := store.Get(last.digest)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", stageevidence.ErrCorruptStore, scope, err)
		}
		records, err := stageevidence.VerifyStored(scope, stored, last.seal)
		if err != nil {
			return err
		}
		if err := checkPriorSeals(scope, history[:len(history)-1], records); err != nil {
			return err
		}
		idx.addScope(scope, last, records)
		digest.Write([]byte(last.digest))
	}
	idx.digest = hex.EncodeToString(digest.Sum(nil))
	return nil
}

func checkPriorSeals(scope stageevidence.ScopeKey, prior []stageSeal, records []stageevidence.Record) error {
	for _, p := range prior {
		if !p.seal.IsPrefixOf(records) {
			return &stageevidence.BrokenChainError{Scope: scope, Seq: p.seal.Count}
		}
	}
	return nil
}

func (idx *stageIndex) addScope(scope stageevidence.ScopeKey, last stageSeal, records []stageevidence.Record) {
	idx.outcomes[scope] = last.seal.Outcome(scope)
	idx.keysByScope[scope] = map[string]bool{}
	for _, r := range records {
		if r.Key == "" {
			continue
		}
		idx.records[r.Key] = append(idx.records[r.Key], r)
		idx.keysByScope[scope][r.Key] = true
	}
}

func removeScope(list []stageevidence.ScopeKey, scope stageevidence.ScopeKey) []stageevidence.ScopeKey {
	out := list[:0]
	for _, s := range list {
		if s != scope {
			out = append(out, s)
		}
	}
	return out
}

func (idx *stageIndex) findUnsealed(evolveDir string, all []stageSeal) error {
	latest := latestOf(all)
	entries, err := os.ReadDir(filepath.Join(evolveDir, stageevidence.WorkingDirName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ledger: list stage evidence working files: %w", err)
	}
	for _, e := range entries {
		scope, ok := stageevidence.ParseWorkingName(e.Name())
		if !ok {
			continue
		}
		body, err := os.ReadFile(filepath.Join(evolveDir, stageevidence.WorkingDirName, e.Name()))
		if err != nil {
			return fmt.Errorf("ledger: read stage evidence working file %s: %w", e.Name(), err)
		}
		if sealed, ok := latest[scope]; ok && sealed.digest == ledgerartifacts.Digest(body) {
			continue
		}
		out := stageevidence.Outcome{Scope: scope.Kind, ID: scope.ID, Incomplete: true}
		idx.outcomes[scope] = out
		idx.unsealed = append(idx.unsealed, out)
	}
	return nil
}

func (idx *stageIndex) forKey(key string) (stageevidence.Chain, error) {
	records := idx.records[key]
	if len(records) == 0 {
		return stageevidence.Chain{}, stageevidence.ErrEmptyChain
	}
	var outcomes []stageevidence.Outcome
	for _, scope := range idx.order {
		if idx.keysByScope[scope][key] {
			outcomes = append(outcomes, idx.outcomes[scope])
		}
	}
	for _, o := range idx.unsealed {
		if !idx.keysByScope[o.ScopeKey()][key] {
			outcomes = append(outcomes, o)
		}
	}
	return stageevidence.PreverifiedChain(records, outcomes, idx.digest), nil
}
