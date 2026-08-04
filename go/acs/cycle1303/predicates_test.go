//go:build acs

// Package cycle1303 materialises the cycle-1303 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	T1 centralize-releasepreflight-verdict-parsing
//	(todo-id sentinel-parse-tail-anchor)
//
// What the cycle must deliver. `go/internal/releasepreflight/releasepreflight.go`
// hand-rolls a SECOND verdict-sentinel scanner (`machineVerdictRE` +
// `markerVerdict`, lines 582-608) instead of calling the project's single source
// of truth, `phasecontract.ParseVerdictSentinelFull` (`sentinel.go:90-98`). The
// duplicate happens to be last-match-wins, so it does not reproduce the
// cycle-1298 first-match bug — but it is missing the SSOT's placeholder-echo
// guard (`isPlaceholderEcho`, `sentinel.go:62-74`, cycle-603 class). A
// Deliverable-Contract example sentinel captured from scrollback into an
// audit artifact is therefore still authoritative to release-preflight, in
// exactly the one call site that never adopted the centralised fix. This cycle
// deletes the duplicate and delegates to the SSOT.
//
// Predicate strategy — every predicate exercises real behaviour through the
// REAL production caller, never a source grep (the cycle-85 degenerate-predicate
// ban). `markerVerdict`/`extractVerdict` are unexported, so this external
// package can only reach them the way the release pipeline does: through
// `releasepreflight.Run`, whose step 4 reads the ledger's newest auditor
// artifact and gates the release on the verdict it parses out of it. Every
// predicate below drives `Run` over a temp repo whose audit-report body is the
// variable under test, and asserts on the release decision that comes back.
// A predicate that called a parser directly would pass on dead code; these stay
// RED until releasepreflight.go's own step-4 path actually reads through
// phasecontract.
//
//   - 001 is the crux and the NEGATIVE case: a real FAIL sentinel followed by a
//     contract-example placeholder echo declaring PASS must NOT release. Today
//     the duplicate scanner takes the placeholder (last valid marker wins, no
//     placeholder guard) and the release is allowed — RED.
//   - 002 is the opposite polarity, and is what makes 001 impossible to satisfy
//     by bolting a reject onto the duplicate: a SOLE placeholder-echo marker is
//     not a marker at all, so the prose verdict below it must govern and the
//     release must proceed. Only true delegation to ParseVerdictSentinelFull
//     produces both 001 and 002.
//   - 003 is the differential ORACLE for the delegation itself: over a corpus of
//     artifact bodies, the verdict release-preflight acts on must equal what
//     phasecontract.ParseVerdictSentinelFull returns for the same bytes. This is
//     AC1 stated behaviourally — two parsers that agree on every input the
//     oracle can distinguish are one parser.
//   - 004 is the AC2 regression guard: the PASS/WARN/FAIL mapping, strict-mode
//     WARN rejection, marker-is-authoritative precedence and prose fallback all
//     keep their current outcomes. Expected pre-existing GREEN — it must stay
//     green through the swap.
//   - 005 is the AC4 suite gate: the two owning packages, each shelled as ONE
//     named package (never a /... sweep), per the flaky-predicate-shape rules.
//
// Fixture note: the repo fixture is hand-rolled rather than borrowed from
// go/test/fixtures so that the ledger line, the artifact path and the artifact
// body are all visible at the point of use — the artifact body IS the variable
// under test in every predicate here.
package cycle1303

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
)

// sentinel renders a v1 verdict sentinel line (no failure block).
func sentinel(verdict string) string {
	return fmt.Sprintf(`<!-- evolve-verdict: {"phase":"audit","verdict":%q,"schema_version":1} -->`, verdict)
}

// placeholderSentinel renders the shape that only ever comes from a Deliverable
// Contract's own printed example echoed into captured scrollback: a v2 sentinel
// whose failure block still holds literal angle-bracket placeholder tokens
// (cycle-603). phasecontract rejects it; the duplicate scanner does not.
func placeholderSentinel(verdict string) string {
	return fmt.Sprintf(`<!-- evolve-verdict: {"phase":"audit","verdict":%q,"schema_version":2,`+
		`"failure":{"class":"<failure class>","defects":["<one line per defect>"],`+
		`"evidence_paths":["<artifact path>"]}} -->`, verdict)
}

// makeRepo builds a preflight-ready repo whose newest auditor artifact carries
// exactly auditBody, and returns the repo root.
func makeRepo(t *testing.T, auditBody string) string {
	t.Helper()
	root := t.TempDir()
	auditPath := filepath.Join(root, ".evolve", "runs", "cycle-99", "audit-report.md")
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"),
		[]byte(`{"name":"x","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditBody), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := fmt.Sprintf(
		`{"ts":%q,"cycle":99,"role":"auditor","kind":"agent_subprocess","model":"opus",`+
			`"exit_code":0,"artifact_path":%q,"artifact_sha256":"deadbeef",`+
			`"git_head":"none","tree_state_sha":"none"}`+"\n",
		time.Now().UTC().Format(time.RFC3339), auditPath)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "ledger.jsonl"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// preflight runs the REAL release preflight over an artifact carrying auditBody
// and returns the audit verdict it settled on plus the release decision. This is
// the production path: cmd/evolve's `release` calls releasepreflight.Run, whose
// step 4 is the only consumer of the verdict-marker parser under test.
func preflight(t *testing.T, auditBody string, strict bool) (verdict string, err error) {
	t.Helper()
	res, err := releasepreflight.Run(releasepreflight.Options{
		Target:         "1.0.1",
		RepoRoot:       makeRepo(t, auditBody),
		SkipTests:      true,
		StrictPass:     strict,
		Stderr:         &strings.Builder{},
		Now:            time.Now,
		GitClean:       func(string) (bool, error) { return true, nil },
		CurrentBranch:  func(string) (string, error) { return "main", nil },
		GateTestRunner: func(string, string) error { return nil },
		CIConclusion: func(string) (releasepreflight.CIRunStatus, error) {
			return releasepreflight.CIRunStatus{}, nil
		},
	})
	return res.AuditVerdict, err
}

// realFailThenPlaceholderEcho is the cycle-603 shape landing in the one call
// site that never adopted the guard: the auditor's genuine FAIL, then the
// Deliverable Contract's own printed PASS example captured from scrollback.
const realFailThenPlaceholderEcho = "# Audit — cycle 99\n\n" +
	"The change regresses the ship gate.\n\n" +
	sentinelFail + "\n\n" +
	"## Deliverable Contract (echoed from the prompt)\n\n" +
	placeholderPass + "\n"

// Rendered once as consts so the corpus in 003 can reference the same bytes.
const (
	sentinelFail    = `<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":1} -->`
	sentinelPass    = `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`
	sentinelWarn    = `<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":1} -->`
	placeholderPass = `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":2,` +
		`"failure":{"class":"<failure class>","defects":["<one line per defect>"],` +
		`"evidence_paths":["<artifact path>"]}} -->`
	placeholderFail = `<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":2,` +
		`"failure":{"class":"<failure class>","defects":["<one line per defect>"]}} -->`
)

// TestC1303_001_placeholder_echo_cannot_override_real_fail is the NEGATIVE case
// and the wiring proof: a genuine FAIL sentinel trailed by a contract-example
// placeholder echo must still block the release. The duplicate scanner in
// releasepreflight takes the last JSON-parsable marker with no placeholder
// guard, so today the echoed PASS wins and the release is allowed — the exact
// re-opening of the closed cycle-603 class that centralising on
// phasecontract.ParseVerdictSentinelFull prevents.
func TestC1303_001_placeholder_echo_cannot_override_real_fail(t *testing.T) {
	verdict, err := preflight(t, realFailThenPlaceholderEcho, false)
	if !errors.Is(err, releasepreflight.ErrCheckFailed) {
		t.Fatalf("a real FAIL sentinel trailed by a contract-example placeholder echo must BLOCK the "+
			"release (the placeholder is never a real agent verdict — sentinel.go:62-74); "+
			"Run returned verdict=%q err=%v, want ErrCheckFailed", verdict, err)
	}
	// Cross-check the SSOT agrees the placeholder is not the verdict: if this
	// ever flips, the fixture — not the production path — is what changed.
	if s, ok := phasecontract.ParseVerdictSentinelFull(realFailThenPlaceholderEcho); !ok || s.Verdict != "FAIL" {
		t.Fatalf("fixture drift: ParseVerdictSentinelFull(fixture) = (%+v, %v), want verdict FAIL", s, ok)
	}
}

// TestC1303_002_sole_placeholder_echo_is_not_a_marker pins the opposite polarity
// so 001 cannot be satisfied by bolting a "reject placeholders" special case
// onto the duplicate scanner. Under ParseVerdictSentinelFull a placeholder echo
// is not a valid candidate AT ALL — so an artifact whose only sentinel is an
// echoed FAIL has NO machine marker, and extractVerdict must fall through to the
// prose verdict, releasing normally. Today the duplicate scanner treats that
// echoed FAIL as authoritative and blocks.
func TestC1303_002_sole_placeholder_echo_is_not_a_marker(t *testing.T) {
	body := "# Audit — cycle 99\n\nVerdict: PASS\n\nConfidence: 1.0\n\n" +
		"## Deliverable Contract (echoed from the prompt)\n\n" + placeholderFail + "\n"
	if s, ok := phasecontract.ParseVerdictSentinelFull(body); ok {
		t.Fatalf("fixture drift: the SSOT must find NO valid sentinel in a placeholder-only body, got %+v", s)
	}
	verdict, err := preflight(t, body, false)
	if err != nil {
		t.Fatalf("a body whose ONLY sentinel is a contract-example placeholder echo has no machine "+
			"marker, so the prose 'Verdict: PASS' must govern and the release must proceed; "+
			"Run err = %v", err)
	}
	if verdict != "PASS" {
		t.Errorf("AuditVerdict = %q, want PASS from the prose fallback", verdict)
	}
}

// oracleCase is one differential row: the same bytes fed to the SSOT parser and
// to the real release-preflight path.
type oracleCase struct {
	name string
	body string
}

// TestC1303_003_release_verdict_matches_phasecontract_oracle is AC1 stated
// behaviourally. For every artifact body, the verdict release-preflight acts on
// must be exactly what phasecontract.ParseVerdictSentinelFull reports (or, when
// the SSOT finds no valid sentinel, whatever the prose fallback yields — the
// oracle then only asserts that no MARKER verdict was manufactured). Two parsers
// that agree on every input this oracle can distinguish are one parser; the
// placeholder rows are the inputs that separate them today.
func TestC1303_003_release_verdict_matches_phasecontract_oracle(t *testing.T) {
	cases := []oracleCase{
		{"lone-pass", "# Audit\n\n" + sentinelPass + "\n"},
		{"lone-warn", "# Audit\n\n" + sentinelWarn + "\n"},
		{"lone-fail", "# Audit\n\n" + sentinelFail + "\n"},
		{"quoted-pass-then-real-fail", "# Audit\n\nPrior cycle said:\n" + sentinelPass + "\n\n" + sentinelFail + "\n"},
		{"real-fail-then-placeholder-pass", realFailThenPlaceholderEcho},
		{"real-pass-then-placeholder-fail", "# Audit\n\n" + sentinelPass + "\n\nContract example:\n" + placeholderFail + "\n"},
		{"malformed-then-real-pass", "# Audit\n\n<!-- evolve-verdict: {not json} -->\n\n" + sentinelPass + "\n"},
		{"empty-verdict-then-real-pass", "# Audit\n\n" +
			`<!-- evolve-verdict: {"phase":"audit","verdict":"","schema_version":1} -->` + "\n\n" + sentinelPass + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, ok := phasecontract.ParseVerdictSentinelFull(tc.body)
			if !ok {
				t.Skipf("no valid sentinel in fixture %q — nothing for the oracle to compare", tc.name)
			}
			// Non-strict: PASS and WARN release, FAIL blocks. That mapping is
			// extractVerdict's and is unchanged by this task; what the oracle
			// pins is WHICH verdict feeds it.
			verdict, err := preflight(t, tc.body, false)
			switch want.Verdict {
			case "PASS", "WARN":
				if err != nil {
					t.Fatalf("SSOT verdict %q must release; release-preflight blocked with %v "+
						"(the two parsers disagree — releasepreflight is not delegating)", want.Verdict, err)
				}
				if verdict != want.Verdict {
					t.Errorf("release-preflight acted on verdict %q, SSOT says %q", verdict, want.Verdict)
				}
			default: // FAIL
				if !errors.Is(err, releasepreflight.ErrCheckFailed) {
					t.Fatalf("SSOT verdict %q must BLOCK; release-preflight returned verdict=%q err=%v "+
						"(the two parsers disagree — releasepreflight is not delegating)",
						want.Verdict, verdict, err)
				}
			}
		})
	}
}

// TestC1303_004_extract_verdict_behavior_preserved is the AC2 regression guard:
// every currently-promised release-gate outcome that does NOT involve a
// placeholder echo must survive the swap byte-for-byte. Expected GREEN before
// and after — a break here means the refactor changed the contract instead of
// centralising it.
func TestC1303_004_extract_verdict_behavior_preserved(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		strict    bool
		wantErr   bool
		wantVerd  string
		rationale string
	}{
		{"marker-pass", "# Audit\n\n" + sentinelPass + "\n", false, false, "PASS",
			"a lone PASS marker releases"},
		{"marker-warn-nonstrict", "# Audit\n\n" + sentinelWarn + "\n", false, false, "WARN",
			"WARN releases under the fluent posture"},
		{"marker-warn-strict", "# Audit\n\n" + sentinelWarn + "\n", true, true, "",
			"EVOLVE_RELEASE_STRICT_PASS rejects WARN"},
		{"marker-fail", "# Audit\n\n" + sentinelFail + "\n", false, true, "",
			"a FAIL marker blocks"},
		{"marker-fail-beats-later-prose-pass", "# Audit\n\n" + sentinelFail + "\n\nVerdict: PASS\n", false, true, "",
			"a present marker is authoritative — prose must not override it"},
		{"last-marker-wins", "# Audit\n\nPrior cycle:\n" + sentinelPass + "\n\n" + sentinelFail + "\n", false, true, "",
			"a quoted earlier PASS can never silence the report's own later FAIL"},
		{"prose-inline-pass", "# Audit\n\nVerdict: PASS\n\nConfidence: 1.0\n", false, false, "PASS",
			"no marker → inline prose fallback still works"},
		{"prose-heading-pass", "# Audit\n\n## Verdict\n\n**PASS**\n", false, false, "PASS",
			"no marker → heading prose fallback still works"},
		{"no-verdict-at-all", "# Audit\n\nNothing conclusive here.\n", false, true, "",
			"an artifact declaring no verdict blocks the release"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, err := preflight(t, tc.body, tc.strict)
			if tc.wantErr {
				if !errors.Is(err, releasepreflight.ErrCheckFailed) {
					t.Fatalf("%s: want ErrCheckFailed, got verdict=%q err=%v", tc.rationale, verdict, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: want release, got err = %v", tc.rationale, err)
			}
			if verdict != tc.wantVerd {
				t.Errorf("%s: AuditVerdict = %q, want %q", tc.rationale, verdict, tc.wantVerd)
			}
		})
	}
}

// TestC1303_005_owning_package_suites_green is AC4. Each suite is shelled as ONE
// named package (never a `/...` sweep and never a multi-package invocation), per
// the flaky-predicate-shape rules; both complete in ~1.5s locally, so neither is
// a contention amplifier. `go` is invoked with an explicit Dir so the predicate
// is indifferent to the lane's process cwd.
func TestC1303_005_owning_package_suites_green(t *testing.T) {
	goDir := repoGoDir(t)
	for _, pkg := range []string{"./internal/releasepreflight", "./internal/phasecontract"} {
		cmd := exec.Command("go", "test", "-count=1", pkg)
		cmd.Dir = goDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("go test %s failed: %v\n%s", pkg, err, out)
		}
	}
}

// repoGoDir walks up from the predicate's own directory (go/acs/cycle1303) to
// the module root, so the suites run against THIS lane's worktree rather than
// whatever tree the process happens to sit in.
func repoGoDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := dir; d != string(filepath.Separator); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatalf("no go.mod found walking up from %s", dir)
	return ""
}

// Keep the rendering helpers referenced: they document the exact sentinel shapes
// the consts above spell out, and are the constructors a future cycle should
// extend the corpus with.
var _ = []func(string) string{sentinel, placeholderSentinel}
