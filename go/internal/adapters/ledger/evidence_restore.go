package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

type EvidenceAction string

const (
	EvidenceDurable      EvidenceAction = "durable"
	EvidenceStored       EvidenceAction = "stored"
	EvidenceRebuilt      EvidenceAction = "rebuilt"
	EvidenceUnrestorable EvidenceAction = "unrestorable"
)

var errNoGitTree = errors.New("the line names no git tree to regenerate it from")

var evidenceActionRank = map[EvidenceAction]int{EvidenceDurable: 0, EvidenceStored: 1, EvidenceRebuilt: 2, EvidenceUnrestorable: 3}

type EvidenceRestoration struct {
	Line   int
	Cycle  int
	Method string
	Action EvidenceAction
	Reason string
}

func (l *FileLedger) PreviewCompositionEvidence(ctx context.Context, repoDir string) ([]EvidenceRestoration, error) {
	return l.walkCompositionEvidence(ctx, repoDir, previewSink{})
}

func (l *FileLedger) RestoreCompositionEvidence(ctx context.Context, repoDir string) ([]EvidenceRestoration, error) {
	return l.walkCompositionEvidence(ctx, repoDir, ledgerSink{store: l.evidence(), ledger: l})
}

type evidenceSink interface {
	keep(diff []byte) (digest string, err error)
	record(rec compositionEvidenceRecord) error
}

type previewSink struct{}

func (previewSink) keep(diff []byte) (string, error)       { return ledgerartifacts.Digest(diff), nil }
func (previewSink) record(compositionEvidenceRecord) error { return nil }

type ledgerSink struct {
	store  ledgerartifacts.Store
	ledger *FileLedger
}

func (s ledgerSink) keep(diff []byte) (string, error) { return s.store.Put(diff) }

func (s ledgerSink) record(rec compositionEvidenceRecord) error {
	return s.ledger.appendChained(func(seq int, prevHash string) any {
		rec.EntrySeq, rec.PrevHash = seq, prevHash
		return rec
	})
}

func (l *FileLedger) walkCompositionEvidence(ctx context.Context, repoDir string, sink evidenceSink) ([]EvidenceRestoration, error) {
	lines, err := l.gatherAllLines()
	if err != nil {
		return nil, fmt.Errorf("ledger evidence restore: %w", err)
	}
	r := evidenceRestorer{index: indexCompositionEvidence(l.evidence(), lines), sink: sink, regenerate: gitDelta(ctx, repoDir)}
	anchorSHA, _ := effectiveAnchorSHA(lines, l.loadAnchorSHA())
	start, found := epochStart(lines, anchorSHA)
	if !found {
		return nil, fmt.Errorf("ledger evidence restore: epoch anchor line not found (sha %s)", anchorSHA)
	}
	var out []EvidenceRestoration
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if !bytes.Contains(line, []byte(`"`+CompositionVerdictKind+`"`)) {
			continue
		}
		var rec compositionRecord
		if json.Unmarshal(line, &rec) != nil || rec.Kind != CompositionVerdictKind {
			continue
		}
		out = append(out, r.restore(i, line, rec))
	}
	return out, nil
}

func gitDelta(ctx context.Context, repoDir string) func(base, tree string) ([]byte, error) {
	git := func(ctx context.Context, dir string, args ...string) (string, int, error) {
		out, _, code, err := gitexec.Default(dir).Capture(ctx, args...)
		return out, code, err
	}
	return func(base, tree string) ([]byte, error) { return treedelta.Delta(ctx, git, repoDir, base, tree) }
}

type evidenceRestorer struct {
	index      compositionEvidenceIndex
	sink       evidenceSink
	regenerate func(base, tree string) ([]byte, error)
}

type evidenceSlot struct {
	compositionEvidence
	base, tree string
}

type evidenceSource struct {
	action EvidenceAction
	read   func() ([]byte, error)
}

func (r evidenceRestorer) restore(i int, line []byte, rec compositionRecord) EvidenceRestoration {
	out := EvidenceRestoration{Line: i, Cycle: rec.Cycle, Method: rec.Method, Action: EvidenceDurable}
	_, aliased := r.index.byLine[sha256Hex(line)]
	slots := slotsFor(rec, r.index.resolve(rec.fields(), line))
	for _, slot := range slots.each() {
		restored, action, reason := r.restoreSlot(*slot, rec.PatchID)
		*slot = restored
		if evidenceActionRank[action] > evidenceActionRank[out.Action] {
			out.Action, out.Reason = action, reason
		}
	}
	if out.Action == EvidenceDurable || out.Action == EvidenceUnrestorable || aliased || rec.AuditedDiffSHA256 != "" {
		return out
	}
	if err := r.sink.record(evidenceRecordFor(line, rec, slots)); err != nil {
		out.Action, out.Reason = EvidenceUnrestorable, "evidence record: "+err.Error()
	}
	return out
}

func evidenceRecordFor(line []byte, rec compositionRecord, slots evidenceSlots) compositionEvidenceRecord {
	return compositionEvidenceRecord{TS: time.Now().UTC().Format(time.RFC3339), Kind: compositionEvidenceKind, Role: operatorRole,
		Cycle: rec.Cycle, LineSHA256: sha256Hex(line), AuditedDiffSHA256: slots.audited.digest, ComposedDiffSHA256: slots.composed.digest}
}

type evidenceSlots struct {
	audited, composed evidenceSlot
}

func slotsFor(rec compositionRecord, f compositionFields) evidenceSlots {
	pair := f.evidence()
	slots := evidenceSlots{audited: evidenceSlot{compositionEvidence: pair.audited}, composed: evidenceSlot{compositionEvidence: pair.composed}}
	if rec.Method == IdenticalRebaseMethod {
		slots.audited.base, slots.audited.tree = rec.AuditedBase, rec.AuditedTreeSHA
		slots.composed.base, slots.composed.tree = rec.GitHead, rec.TreeStateSHA
	}
	return slots
}

func (s *evidenceSlots) each() []*evidenceSlot {
	return []*evidenceSlot{&s.audited, &s.composed}
}

func (r evidenceRestorer) restoreSlot(s evidenceSlot, patchID string) (evidenceSlot, EvidenceAction, string) {
	if s.digest != "" {
		if _, err := r.index.store.Get(s.digest); err == nil {
			return s, EvidenceDurable, ""
		}
	}
	diff, action, err := r.recover(s, patchID)
	if err == nil {
		s.digest, err = r.sink.keep(diff)
	}
	if err != nil {
		return s, EvidenceUnrestorable, fmt.Sprintf("%s: %v", s.label, err)
	}
	return s, action, ""
}

func (r evidenceRestorer) recover(s evidenceSlot, patchID string) ([]byte, EvidenceAction, error) {
	var refusals []string
	for _, src := range r.sources(s) {
		diff, err := src.read()
		if err == nil {
			err = s.proves(diff, patchID)
		}
		if err == nil {
			return diff, src.action, nil
		}
		refusals = append(refusals, fmt.Sprintf("%s: %v", src.action, err))
	}
	return nil, EvidenceUnrestorable, errors.New(strings.Join(refusals, "; "))
}

func (r evidenceRestorer) sources(s evidenceSlot) []evidenceSource {
	rebuild := func() ([]byte, error) { return r.regenerate(s.base, s.tree) }
	if s.tree == "" {
		rebuild = func() ([]byte, error) { return nil, errNoGitTree }
	}
	return []evidenceSource{
		{action: EvidenceStored, read: func() ([]byte, error) { return os.ReadFile(s.path) }},
		{action: EvidenceRebuilt, read: rebuild},
	}
}

func (s evidenceSlot) proves(diff []byte, patchID string) error {
	got, err := PatchID(diff)
	if err != nil {
		return err
	}
	if got != patchID {
		return fmt.Errorf("re-derives patch-id %s, the line records %s", got, patchID)
	}
	if digest := ledgerartifacts.Digest(diff); s.digest != "" && digest != s.digest {
		return fmt.Errorf("hashes to %s, the line records %s", digest, s.digest)
	}
	return nil
}
