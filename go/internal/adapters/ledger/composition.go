package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// CompositionVerdictKind is the kind of an audit carry-forward entry that ledger verify kernel-recomputes.
const CompositionVerdictKind = "composition-verdict"

// TrivialRebaseMethod is the RUNG 0 composition method; the writer and the ship-side reader share it.
const TrivialRebaseMethod = "trivial-rebase"

// ScopedReviewMethod is the RUNG 2 method, recorded when a scoped merge review resolved an overlapping change.
const ScopedReviewMethod = "scoped-review"

// IdenticalRebaseMethod is the ADR-0105 B3 method: a byte-identical rebase carrying the verdict of the audited tree it names.
const IdenticalRebaseMethod = "identical-rebase"

// compositionFields is the kernel-recomputable subset of a composition-verdict line.
type compositionFields struct {
	PatchID            string `json:"patch_id"`
	AuditedDiffPath    string `json:"audited_diff_path"`
	ComposedDiffPath   string `json:"composed_diff_path"`
	AuditedDiffSHA256  string `json:"audited_diff_sha256"`
	ComposedDiffSHA256 string `json:"composed_diff_sha256"`
}

func (r compositionRecord) fields() compositionFields {
	return compositionFields{PatchID: r.PatchID, AuditedDiffPath: r.AuditedDiffPath, ComposedDiffPath: r.ComposedDiffPath,
		AuditedDiffSHA256: r.AuditedDiffSHA256, ComposedDiffSHA256: r.ComposedDiffSHA256}
}

type compositionEvidencePair struct {
	audited, composed compositionEvidence
}

func (f compositionFields) evidence() compositionEvidencePair {
	return compositionEvidencePair{
		audited:  compositionEvidence{label: "audited_diff_path", digest: f.AuditedDiffSHA256, path: f.AuditedDiffPath},
		composed: compositionEvidence{label: "composed_diff_path", digest: f.ComposedDiffSHA256, path: f.ComposedDiffPath},
	}
}

func (p compositionEvidencePair) both() []compositionEvidence {
	return []compositionEvidence{p.audited, p.composed}
}

type compositionEvidence struct {
	label, digest, path string
}

func (e compositionEvidence) read(store ledgerartifacts.Store) ([]byte, error) {
	if e.digest != "" {
		return store.Get(e.digest)
	}
	return os.ReadFile(e.path)
}

const compositionEvidenceKind = "composition-evidence"

type compositionEvidenceRecord struct {
	TS                 string `json:"ts"`
	Kind               string `json:"kind"`
	Role               string `json:"role"`
	Cycle              int    `json:"cycle"`
	LineSHA256         string `json:"composition_line_sha256"`
	AuditedDiffSHA256  string `json:"audited_diff_sha256"`
	ComposedDiffSHA256 string `json:"composed_diff_sha256"`
	EntrySeq           int    `json:"entry_seq"`
	PrevHash           string `json:"prev_hash"`
}

type compositionEvidenceIndex struct {
	store  ledgerartifacts.Store
	byLine map[string]compositionEvidenceRecord
}

func indexCompositionEvidence(store ledgerartifacts.Store, lines [][]byte) compositionEvidenceIndex {
	idx := compositionEvidenceIndex{store: store, byLine: map[string]compositionEvidenceRecord{}}
	marker := []byte(`"` + compositionEvidenceKind + `"`)
	for _, line := range lines {
		if !bytes.Contains(line, marker) {
			continue
		}
		var rec compositionEvidenceRecord
		if json.Unmarshal(line, &rec) == nil && rec.Kind == compositionEvidenceKind {
			idx.byLine[rec.LineSHA256] = rec
		}
	}
	return idx
}

func (idx compositionEvidenceIndex) resolve(f compositionFields, line []byte) compositionFields {
	if f.AuditedDiffSHA256 != "" || f.ComposedDiffSHA256 != "" {
		return f
	}
	if alias, ok := idx.byLine[sha256Hex(line)]; ok {
		f.AuditedDiffSHA256, f.ComposedDiffSHA256 = alias.AuditedDiffSHA256, alias.ComposedDiffSHA256
	}
	return f
}

// PatchID returns the `git patch-id --stable` content identity of diff; it needs no repository.
func PatchID(diff []byte) (string, error) {
	cmd := sysexec.Command(context.Background(), "git", "patch-id", "--stable")
	cmd.Stdin = bytes.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git patch-id --stable: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("git patch-id --stable: empty output (empty diff?)")
	}
	return fields[0], nil
}

// CompositionVerdictInput is what a caller supplies to persist a composition verdict; the writer trusts none of it.
type CompositionVerdictInput struct {
	Cycle          int
	Method         string            // composition method; blank defaults to TrivialRebaseMethod (RUNG 0)
	LaneAuditRef   string            // artifact_sha256 of the bound auditor entry
	PatchID        string            // caller-claimed patch-id of the change
	AuditedBase    string            // git HEAD the audit originally bound
	GitHead        string            // composed HEAD the verdict carries forward to
	TreeStateSHA   string            // composed tree state the gates ran on
	AuditedTreeSHA string            // the audited changes tree the carry binds (identical-rebase only)
	GateResults    map[string]string // must record "pass" for every required composed gate
	AuditedDiff    []byte            // unified diff the audit reviewed
	ComposedDiff   []byte            // unified diff of the composed (rebased) tree
}

// compositionRecord is the on-disk line: the union of ship's compositionEntry and compositionFields,
// plus the chain fields appendChained fills in.
type compositionRecord struct {
	TS                 string            `json:"ts"`
	Cycle              int               `json:"cycle"`
	Kind               string            `json:"kind"`
	Method             string            `json:"method"`
	LaneAuditRef       string            `json:"lane_audit_ref"`
	PatchID            string            `json:"patch_id"`
	AuditedBase        string            `json:"audited_base"`
	GitHead            string            `json:"git_head"`
	TreeStateSHA       string            `json:"tree_state_sha"`
	AuditedTreeSHA     string            `json:"audited_tree_sha,omitempty"`
	GateResults        map[string]string `json:"gate_results"`
	AuditedDiffPath    string            `json:"audited_diff_path"`
	ComposedDiffPath   string            `json:"composed_diff_path"`
	AuditedDiffSHA256  string            `json:"audited_diff_sha256,omitempty"`
	ComposedDiffSHA256 string            `json:"composed_diff_sha256,omitempty"`
	EntrySeq           int               `json:"entry_seq"`
	PrevHash           string            `json:"prev_hash"`
}

// WriteCompositionVerdict validates in, persists both diffs and appends one chained line; validation writes nothing.
func WriteCompositionVerdict(ledgerPath string, in CompositionVerdictInput) error {
	if err := checkCompositionLedgerPath(ledgerPath); err != nil {
		return err
	}
	if err := validateCompositionInput(in); err != nil {
		return err
	}

	audited, composed, err := storeCompositionDiffs(ledgerartifacts.Open(filepath.Dir(ledgerPath)), in)
	if err != nil {
		return err
	}

	method := in.Method
	if method == "" {
		method = TrivialRebaseMethod
	}
	rec := compositionRecord{
		TS:                 time.Now().UTC().Format(time.RFC3339),
		Cycle:              in.Cycle,
		Kind:               CompositionVerdictKind,
		Method:             method,
		LaneAuditRef:       in.LaneAuditRef,
		PatchID:            in.PatchID,
		AuditedBase:        in.AuditedBase,
		GitHead:            in.GitHead,
		TreeStateSHA:       in.TreeStateSHA,
		AuditedTreeSHA:     in.AuditedTreeSHA,
		GateResults:        in.GateResults,
		AuditedDiffPath:    audited.path,
		ComposedDiffPath:   composed.path,
		AuditedDiffSHA256:  audited.digest,
		ComposedDiffSHA256: composed.digest,
	}
	return New(filepath.Dir(ledgerPath)).appendChained(func(seq int, prevHash string) any {
		rec.EntrySeq = seq
		rec.PrevHash = prevHash
		return rec
	})
}

func checkCompositionLedgerPath(ledgerPath string) error {
	if filepath.Base(ledgerPath) != "ledger.jsonl" {
		return fmt.Errorf("composition-verdict: ledger path must be a ledger.jsonl (chained append), got %q", ledgerPath)
	}
	if info, err := os.Lstat(ledgerPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("composition-verdict: ledger path %s is a link: its tip, lock and evidence store would be its own directory's, forking the chain it points at — write through the ledger it links to", ledgerPath)
	}
	return nil
}

func validateCompositionInput(in CompositionVerdictInput) error {
	diffs := []struct {
		label string
		diff  []byte
	}{
		{"audited diff", in.AuditedDiff},
		{"composed diff", in.ComposedDiff},
	}
	for _, d := range diffs {
		if len(bytes.TrimSpace(d.diff)) == 0 {
			return fmt.Errorf("composition-verdict: %s is empty or whitespace-only — nothing to attest", d.label)
		}
	}
	if missing := ciparity.MissingComposedGates(in.GateResults); missing != nil {
		return fmt.Errorf("composition-verdict: composed-tree gates not green: %s", strings.Join(missing, ","))
	}
	for _, d := range diffs {
		got, err := PatchID(d.diff)
		if err != nil {
			return fmt.Errorf("composition-verdict: %s: %w", d.label, err)
		}
		if got != in.PatchID {
			return fmt.Errorf("composition-verdict: %s recomputes patch-id %s but caller claims %s — refusing to write a line verify would flag as tampered", d.label, got, in.PatchID)
		}
	}
	return nil
}

func storeCompositionDiffs(store ledgerartifacts.Store, in CompositionVerdictInput) (audited, composed compositionEvidence, err error) {
	if audited, err = storeCompositionDiff(store, in.AuditedDiff); err != nil {
		return audited, composed, err
	}
	composed, err = storeCompositionDiff(store, in.ComposedDiff)
	return audited, composed, err
}

func storeCompositionDiff(store ledgerartifacts.Store, diff []byte) (compositionEvidence, error) {
	digest, err := store.Put(diff)
	if err != nil {
		return compositionEvidence{}, fmt.Errorf("composition-verdict: store diff: %w", err)
	}
	path, err := store.Path(digest)
	return compositionEvidence{digest: digest, path: path}, err
}

// verifyCompositionLine checks both persisted diffs recompute the recorded patch_id. Failures wrap
// core.ErrLedgerChainBroken so `evolve ledger verify` exits 2 as for a hash break.
func verifyCompositionLine(i int, line []byte, evidence compositionEvidenceIndex) error {
	var f compositionFields
	if err := json.Unmarshal(line, &f); err != nil {
		return fmt.Errorf("%w: line %d composition-verdict unmarshal: %v", core.ErrLedgerChainBroken, i, err)
	}
	if f.PatchID == "" {
		return fmt.Errorf("%w: line %d composition-verdict has no patch_id — not kernel-recomputable", core.ErrLedgerChainBroken, i)
	}
	for _, p := range evidence.resolve(f, line).evidence().both() {
		diff, err := p.read(evidence.store)
		if err != nil {
			return fmt.Errorf("%w: line %d composition-verdict %s unreadable: %v", core.ErrLedgerChainBroken, i, p.label, err)
		}
		got, err := PatchID(diff)
		if err != nil {
			return fmt.Errorf("%w: line %d composition-verdict %s: %v", core.ErrLedgerChainBroken, i, p.label, err)
		}
		if got != f.PatchID {
			return fmt.Errorf("%w: line %d composition-verdict tampered: %s recomputes patch-id %s, entry records %s",
				core.ErrLedgerChainBroken, i, p.label, got, f.PatchID)
		}
	}
	return nil
}

// CompositionVerdict is one carry record as ship reads it back; the diffs stay on disk at the recorded paths.
type CompositionVerdict struct {
	Cycle            int
	Method           string
	LaneAuditRef     string
	PatchID          string
	AuditedBase      string
	GitHead          string
	TreeStateSHA     string
	AuditedTreeSHA   string
	GateResults      map[string]string
	AuditedDiffPath  string
	ComposedDiffPath string
}

// LatestCompositionVerdict returns the newest carry record of method for the audit laneAuditRef names;
// an absent ledger or no such record is simply no carry.
func LatestCompositionVerdict(ledgerPath, method, laneAuditRef string) (CompositionVerdict, bool, error) {
	body, err := os.ReadFile(ledgerPath)
	if os.IsNotExist(err) {
		return CompositionVerdict{}, false, nil
	}
	if err != nil {
		return CompositionVerdict{}, false, err
	}
	var latest CompositionVerdict
	found := false
	for _, line := range bytes.Split(body, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec compositionRecord
		if json.Unmarshal(line, &rec) != nil || rec.Kind != CompositionVerdictKind || rec.Method != method || rec.LaneAuditRef != laneAuditRef {
			continue
		}
		latest = CompositionVerdict{Cycle: rec.Cycle, Method: rec.Method, LaneAuditRef: rec.LaneAuditRef, PatchID: rec.PatchID,
			AuditedBase: rec.AuditedBase, GitHead: rec.GitHead, TreeStateSHA: rec.TreeStateSHA, AuditedTreeSHA: rec.AuditedTreeSHA,
			GateResults: rec.GateResults, AuditedDiffPath: rec.AuditedDiffPath, ComposedDiffPath: rec.ComposedDiffPath}
		found = true
	}
	return latest, found, nil
}
