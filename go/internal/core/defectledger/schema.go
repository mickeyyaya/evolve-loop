package defectledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// The artifacts the unit owns, in the cycle workspace.
const (
	LedgerFile       = "defect-ledger.json"
	DispositionsFile = "defect-dispositions.json"
)

// The status vocabulary. Evidence is mandatory on FIXED and Reason on
// DEFERRED — an unevidenced closure claim is the laundering primitive.
const (
	StatusOpen     = "OPEN"
	StatusFixed    = "FIXED"
	StatusDeferred = "DEFERRED"
)

// PrescriptionPrefix tags the rows a WARN's prescriptions carry, so an
// operator reading the ledger can tell a named fix for a foreseen risk from a
// defect without a second ledger or a schema-breaking Kind field. carryover
// projects it for the prescription carry-in.
const PrescriptionPrefix = "PRESCRIPTION: "

// MaxEntries and TextMaxRunes bound the ledger against an agent-authored
// verdict sentinel carrying thousands of defects or a megabyte-long defect
// line (cycle-1282 DEF-6). The ledger is re-read and re-written on every
// Classify, so unbounded growth is quadratic work on the audit hot path as
// well as an unreadable artifact. Overflow is RECORDED as a synthetic entry,
// never silently dropped — a cap that erases defects would be the laundering
// primitive wearing a resource-limit costume.
const (
	MaxEntries   = 64
	TextMaxRunes = 2000
)

// The two NAMED markers the disposition pre-flight emits. Deliberately
// distinct from the per-id "(no disposition)" text: an operator reading a
// blocked continuation must see that the ARTIFACT as a whole is absent or
// short. MISSING and INCOMPLETE stay separate because the operator action
// differs — author the file from scratch vs finish the one that exists.
const (
	PreflightMissingMarker    = "disposition-preflight: MISSING"
	PreflightIncompleteMarker = "disposition-preflight: INCOMPLETE"
)

// DispositionsSchemaExample is the ONE canonical defect-dispositions.json
// example, surfaced inline on rejection (cycle-1403 Task 3): the agent
// re-authoring the file on the next dispatch does not read Go. It is
// byte-for-byte the same document (as JSON) as the examples in
// agents/evolve-auditor.md and docs/architecture/continuation-defect-ledger.md
// — the audit package's defect_ledger_doc_example_test.go holds the three in
// sync, so there is one schema with three projections.
const DispositionsSchemaExample = `{"dispositions": [
  {"id": "d0f3a7c1e59b246d8a0c4e6f13579bde2", "status": "FIXED",
   "evidence": "go/internal/phases/audit/defect_ledger.go:267-356"},
  {"id": "d9c8b7a6958473625140f3e2d1c0b9a87", "status": "DEFERRED",
   "reason": "out of this lane's scope; queued as disposition-evidence-tolerant-unmarshal"}
]}`

// evidenceSeparator joins a multi-citation `evidence` value into the single
// string carried by Entry.Evidence and written back into defect-ledger.json.
// The audit resolver (the injected Strategy) splits that string on the bare
// ';' and trims each fragment — deliberately looser, so a hand-written
// "a.go;b.go" still separates; the join only has to produce what that split
// accepts. Nothing ties the two tokens: the resolver is another package's
// until it becomes its own leaf (follow-up F6), and test 24 pins the joined
// bytes.
const evidenceSeparator = "; "

// Entry is one tracked defect — the on-disk row.
type Entry struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Status   string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Doc is the on-disk <workspace>/defect-ledger.json wire shape. OriginCycle
// names the cycle that RAISED the defects, so a continuation can trace lineage
// back past its immediate ancestor.
type Doc struct {
	OriginCycle int     `json:"origin_cycle"`
	Entries     []Entry `json:"entries"`
}

// OpenEntries returns the rows whose status is exactly OPEN — the ONE
// spelling of "still owed" the adoption seeder and the prompt block read.
func (d Doc) OpenEntries() []Entry {
	var open []Entry
	for _, e := range d.Entries {
		if e.Status == StatusOpen {
			open = append(open, e)
		}
	}
	return open
}

// readFault is a present ledger that could not be loaded; op names the failing
// step (read | parse) for the triage fields. Its text is the wire the
// diagnostics carry verbatim.
type readFault struct {
	op  string
	err error
}

func (f *readFault) Error() string { return f.op + " " + LedgerFile + ": " + f.err.Error() }
func (f *readFault) Unwrap() error { return f.err }

// read loads dir's ledger. Missing file → (zero, false, nil): a cycle with no
// ledger has nothing to reconcile. Present-but-unparseable is a fault —
// schema drift on the anti-laundering record must be loud.
func read(dir string) (Doc, bool, *readFault) {
	raw, err := os.ReadFile(filepath.Join(dir, LedgerFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Doc{}, false, nil
		}
		return Doc{}, false, &readFault{op: "read", err: err}
	}
	var doc Doc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Doc{}, false, &readFault{op: "parse", err: err}
	}
	return doc, true, nil
}

// Read is the Center-free reader carryover and the adoption seeder decode
// through — a read fault here can never fire an AUDIT_* code; each caller
// keeps its own posture.
func Read(dir string) (Doc, bool, error) {
	doc, ok, fault := read(dir)
	if fault != nil {
		return doc, ok, fault
	}
	return doc, ok, nil
}

// Write persists doc atomically into dir (2-space JSON, the parent created).
func Write(dir string, doc Doc) error {
	return atomicwrite.JSON(filepath.Join(dir, LedgerFile), doc)
}

// ID derives an entry id from the defect TEXT alone. A positional id re-binds
// the same id string to different text as soon as a list is reordered, so a
// disposition keyed on it closes something other than what it claims —
// laundering by renumbering. A content hash is stable across cycles, chains
// and re-emissions. SIXTEEN bytes, not four (cycle-1282 DEF-3): the preimage
// is chosen by the agent authoring the sentinel, and a 32-bit id is a ~2^32
// brute force away from a benign defect that collides with an inherited
// CRITICAL; the merge additionally cross-checks TEXT per id.
func ID(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "d" + hex.EncodeToString(sum[:16])
}

// Truncate bounds s to at most max runes, marking any cut so a reader can tell
// a clipped defect line from a short one. The FOURTH rune-cap rule beside
// carryover's three: no TrimSpace, the suffix with no leading space.
func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…[truncated]"
}

// dispositionDoc is the on-disk <workspace>/defect-dispositions.json the
// continuation's builder/auditor writes: the claim, per inherited defect id.
type dispositionDoc struct {
	Dispositions []struct {
		ID       string              `json:"id"`
		Status   string              `json:"status"`
		Evidence dispositionEvidence `json:"evidence"`
		Reason   string              `json:"reason"`
	} `json:"dispositions"`
}

// dispositionEvidence is the wire type of a disposition's `evidence` field: a
// single citation STRING or a JSON ARRAY of citation strings (cycle-1399: the
// auditor cited `["a.go:1", "b.go:2"]`, a string-typed field refused the whole
// document and the gate blocked a correct claim). Tolerance is widened for the
// SHAPE only, never the CLAIM: an object, number or bool is still rejected
// outright rather than degraded to "" (cycle-1285 F2 — a silent degrade is the
// gate's cheapest bypass), and every citation must still resolve on its own.
type dispositionEvidence struct {
	citations []string
}

// UnmarshalJSON accepts `"a"` and `["a","b"]`; everything else is an error,
// which surfaces through ReadDispositions' blocking unparseable branch.
func (e *dispositionEvidence) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		e.citations = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		e.citations = many
		return nil
	}
	return fmt.Errorf("`evidence` must be a citation string or an array of citation strings, got %s", Truncate(strings.TrimSpace(string(raw)), 120))
}

// joined renders the citations as the one string the rest of the mechanism
// carries. An empty array joins to "" — the existing "no evidence" case, not a
// new pass: `[]` is a FIXED claim with nothing behind it.
func (e dispositionEvidence) joined() string {
	return strings.Join(e.citations, evidenceSeparator)
}
