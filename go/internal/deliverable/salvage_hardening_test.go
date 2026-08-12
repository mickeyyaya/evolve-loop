package deliverable

// salvage_hardening_test.go — the acceptance criteria four consecutive lane
// audits demanded of the salvage stage and no attempt was ever scoped to close
// (cycles 1432/1434/1441/1442). Each test here IS one finding, written RED
// before its fix:
//
//	H3 (1442) stage dial not honored ...... TestReviewerReview_ShadowStage_NoSideEffects
//	H1 (1442) fail-closed arm untested .... TestReviewerReview_PersistFailure_FailsClosed
//	M2 (1441) breaker cleared by salvage .. TestReviewerReview_SalvageIsBreakerNeutral
//	F1 (1442 adversarial) TOCTOU .......... TestPersistSalvagedArtifact_RefusesWhenFileChangedUnderGate
//	M2 (1442) torn line bricks the report . TestCountSalvageApplied_TolerantOfTornLines
//	H2 (1441) residue: json.Valid guard ... TestSalvageVerdict_RefusesUnrepairablePayload
//
// The unifying invariant the auditors kept restating: a salvage may only ever
// COST a recovery, never buy one — every refusal path must be executed by a
// test, and no effect may escape the enforce dial.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// soleFencedPass is the one shape salvage is allowed to act on: sole
// bad_verdict, fenced-json, exactly one candidate, repairs to bytes that
// re-verify clean.
const soleFencedPass = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

// salvageFixture builds a workspace + project root holding one salvageable
// deliverable, and returns (workspace, projectRoot, artifactPath).
func salvageFixture(t *testing.T) (string, string, string) {
	t.Helper()
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", soleFencedPass)
	pr := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pr, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	return ws, pr, filepath.Join(ws, "audit-report.md")
}

// TestReviewerReview_ShadowStage_NoSideEffects — cycle-1442 audit H3.
//
// The salvage block was placed above the stage dial (it inherited the position
// of the observability record, which is unconditional BY DESIGN). At
// shadow/advisory the gate therefore rewrote the judged artifact on disk,
// appended the telemetry sidecar and touched the breaker — while reporting
// itself "disabled". A shadow soak run to decide whether salvage is safe then
// measures a system the disabled gate has already mutated.
//
// Effects, not the decision, are what the dial governs: computing "would this
// have salvaged" in shadow is the whole point of shadow, so this test pins the
// three EFFECTS byte-identical, not the absence of the computation.
func TestReviewerReview_ShadowStage_NoSideEffects(t *testing.T) {
	for _, stage := range []config.Stage{config.StageShadow, config.StageAdvisory} {
		t.Run(stage.String(), func(t *testing.T) {
			ws, pr, artifact := salvageFixture(t)
			sidecar := filepath.Join(pr, ".evolve", SalvageAppliedFile)
			breaker := filepath.Join(t.TempDir(), "b.json")
			// Seed a real count: an ABSENT breaker file is not evidence —
			// resetBreaker writes nothing when the count is already zero, so
			// stat-ing for absence passed even with the fix reverted
			// (diff-review LOW: the leg was vacuous).
			if n := incrBreaker(breaker); n != 1 {
				t.Fatalf("fixture: incr = %d, want 1", n)
			}

			before, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatal(err)
			}

			r := newTestReviewerPhaseIO(stage, config.StageEnforce, breaker, 3)
			r.logf = func(string, ...any) {}
			got := r.Review(context.Background(), reviewInput("audit", ws, pr))

			// Shadow approves either way; the verdict is not what is under test.
			if !got.Approve {
				t.Fatalf("precondition: stage=%s always approves; got block (%s)", stage, got.Reason)
			}
			after, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Errorf("stage=%s REWROTE the judged artifact — the disabled gate mutated the thing a soak run is measuring\n before: %q\n after:  %q",
					stage, before, after)
			}
			if _, err := os.Stat(sidecar); err == nil {
				t.Errorf("stage=%s appended the salvage telemetry sidecar %s — an effect above the dial", stage, SalvageAppliedFile)
			}
			if n := incrBreaker(breaker); n != 2 {
				t.Errorf("stage=%s changed the breaker count (next incr = %d, want 2) — an effect above the dial", stage, n)
			}
		})
	}
}

// TestReviewerReview_PersistFailure_FailsClosed — cycle-1442 audit H1.
//
// The build report called this "the one place the gate must not fail open" and
// the branch shipped with zero executions (profile `reviewer.go:154.85,156.4
// 1 0`). If the approved bytes cannot be persisted, approving on them
// reinstates the very defect persistence closed: an approval over bytes no
// downstream reader will ever see.
func TestReviewerReview_PersistFailure_FailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the write-permission bit this fixture relies on")
	}
	ws, pr, _ := salvageFixture(t)

	// Make the workspace unwritable so atomicwrite's CreateTemp+rename fails
	// while the artifact itself stays readable (the verify half must succeed,
	// or this would be testing a different branch).
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })

	var logged []string
	r := newTestReviewerPhaseIO(config.StageEnforce, config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	r.logf = func(f string, a ...any) { logged = append(logged, f) }

	got := r.Review(context.Background(), reviewInput("audit", ws, pr))
	if got.Approve {
		t.Fatalf("salvage approved a repair it could NOT persist — the gate failed open on the one path it must not (cycle-1442 H1). logs: %v", logged)
	}
	var named bool
	for _, l := range logged {
		if strings.Contains(l, "refusing the salvage") {
			named = true
		}
	}
	if !named {
		t.Errorf("the refusal must say so in the operator log; got %v", logged)
	}
}

// TestReviewerReview_SalvageIsBreakerNeutral — cycle-1441 audit M2(b).
//
// The repo rule is that salvage rungs are breaker-NEUTRAL. The salvage approve
// path called resetBreaker, so a phase emitting recoverable-malformed reports
// held the consecutive-block counter at zero forever: neither the second-block
// escalation ladder nor the third-block breaker could ever fire, and a
// persistently malformed producer became invisible to both.
func TestReviewerReview_SalvageIsBreakerNeutral(t *testing.T) {
	ws, pr, _ := salvageFixture(t)
	breaker := filepath.Join(t.TempDir(), "b.json")

	// Two prior blocks on the record.
	if n := incrBreaker(breaker); n != 1 {
		t.Fatalf("fixture: first incr = %d, want 1", n)
	}
	if n := incrBreaker(breaker); n != 2 {
		t.Fatalf("fixture: second incr = %d, want 2", n)
	}

	r := newTestReviewerPhaseIO(config.StageEnforce, config.StageEnforce, breaker, 3)
	r.logf = func(string, ...any) {}
	if got := r.Review(context.Background(), reviewInput("audit", ws, pr)); !got.Approve {
		t.Fatalf("precondition: the fixture must salvage; got block (%s)", got.Reason)
	}

	// Neutral: the count is neither cleared nor advanced by a salvage.
	if n := incrBreaker(breaker); n != 3 {
		t.Errorf("salvage was not breaker-NEUTRAL: after two blocks + one salvage the next block counted %d, want 3 (a reset here hides a persistently malformed producer from both the escalation ladder and the breaker)", n)
	}
}

// TestPersistSalvagedArtifact_RefusesWhenFileChangedUnderGate — cycle-1442
// adversarial-review F1 (raised, never adjudicated).
//
// The gate reads the artifact once, decides over those bytes, then writes the
// repair. atomicwrite is an unconditional rename, not a compare-and-swap, so a
// still-live agent that rewrites its report in the window between the read and
// the write has its CORRECTED verdict silently replaced by the repaired stale
// bytes — with Approve=true. The write must be conditional on the file still
// holding the bytes the decision was computed over.
func TestPersistSalvagedArtifact_RefusesWhenFileChangedUnderGate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-report.md")
	const judged = "the bytes the gate read and decided over\n"
	const corrected = "the agent's OWN corrected report, written after the gate's read\n"
	if err := os.WriteFile(path, []byte(corrected), 0o644); err != nil {
		t.Fatal(err)
	}

	err := persistSalvagedArtifact(path, judged, "repaired bytes\n")
	if err == nil {
		t.Fatal("persist overwrote a file that changed under the gate — a live agent's corrected report is silently replaced by repaired stale bytes (adversarial F1)")
	}
	got, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(got) != corrected {
		t.Errorf("the refusal must leave the file untouched; got %q", got)
	}

	// The ordinary path still writes: the guard must not cost every salvage.
	if err := os.WriteFile(path, []byte(judged), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := persistSalvagedArtifact(path, judged, "repaired bytes\n"); err != nil {
		t.Fatalf("unchanged file must persist normally: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "repaired bytes\n" {
		t.Errorf("repaired bytes not persisted; got %q", got)
	}
}

// TestCountSalvageApplied_TolerantOfTornLines — cycle-1442 audit M2.
//
// The sidecar is append-only and unauthenticated: a torn line is an ordinary
// crash artifact, not an attack. The in-process summary tolerates one by
// design; the CLI counter hard-errored on it and `evolve salvage report` then
// exited 1 with NO output, discarding the already-computed baseline section.
// Two consumers of one file disagreeing on robustness is the defect — and the
// tolerant reading must still be HONEST about what it skipped, or a forged
// torn line becomes a way to hide records.
func TestCountSalvageApplied_TolerantOfTornLines(t *testing.T) {
	rec := func(pattern string) string {
		b, _ := json.Marshal(map[string]any{"event_type": salvageAppliedEventType, "pattern": pattern})
		return string(b)
	}
	content := strings.Join([]string{
		rec("fenced-json"),
		`{"event_type":"salvage_applied","pattern":"trunc`, // torn by a crash mid-append
		rec("displaced-line"),
		"",
		"not json at all",
	}, "\n") + "\n"

	saved, malformed, err := CountSalvageApplied(strings.NewReader(content))
	if err != nil {
		t.Fatalf("a torn line must not brick the count — the whole operator report is discarded on this error: %v", err)
	}
	if saved != 2 {
		t.Errorf("saved = %d, want 2 (the two intact records)", saved)
	}
	if malformed != 2 {
		t.Errorf("malformed = %d, want 2 — skipping silently would let a forged torn line hide records", malformed)
	}
}

// TestSalvageVerdict_RefusesUnrepairablePayload — cycle-1441 audit H2 residue.
//
// repairVerdict's last guard refuses to emit a sentinel whose payload is not
// valid JSON (json.Valid, salvage_extract.go). It shipped at zero executions:
// the classifier's own recoverable shapes normally repair to valid JSON, so no
// existing fixture reaches it. A guard nothing executes is a guard nobody knows
// still works.
func TestSalvageVerdict_RefusesUnrepairablePayload(t *testing.T) {
	// Recoverable-looking (fenced block carrying a verdict key) but the payload
	// is structurally broken beyond a trailing comma, so no repair can produce
	// valid JSON.
	const content = "## Verdict\n" +
		"```json\n" + `{"phase":"audit","verdict":"PASS" "stray":}` + "\n```\n"

	res := Result{
		Content:    content,
		Violations: []Violation{{Code: CodeBadVerdict, Message: "no parseable verdict"}},
	}
	got, applied := SalvageVerdict(res)
	if applied {
		t.Errorf("salvage claimed a repair for a payload that cannot be valid JSON — the json.Valid refusal guard did not hold")
	}
	if got.Content != res.Content {
		t.Errorf("a refusal must return the input byte-identical; got %q", got.Content)
	}
}

// TestRepairVerdict_RefusesSpanNotAddressingContent — the other refusal arm of
// the same guard (theme T2: every failure arm executed, not just the happy
// one). The offsets are qualified against the content they were computed from;
// a classification carried to DIFFERENT bytes addresses nothing, and repairing
// on it is precisely the classifier↔repairer divergence the offset threading
// exists to make impossible (cycle-1406 CRITICAL-1).
func TestRepairVerdict_RefusesSpanNotAddressingContent(t *testing.T) {
	cls := BadVerdictClassification{
		Recoverable: true,
		Pattern:     SalvagePatternFencedJSON,
		Reason:      "fixture",
		span:        verdictSpan{start: 10, end: 400},
		payload:     verdictSpan{start: 20, end: 300},
	}
	if _, ok := repairVerdict("short content", cls); ok {
		t.Error("repair accepted a span that does not address these bytes — offsets from one document must never repair another")
	}
}

// TestPersistSalvagedArtifact_NoArtifactPathIsNoOp — the NoArtifact contract
// (deliverable.go, ship): no artifact to reconcile, so persistence is a no-op
// SUCCESS. Pinned because the alternative reading (error) would fail-closed
// every salvage on an artifact-less phase.
func TestPersistSalvagedArtifact_NoArtifactPathIsNoOp(t *testing.T) {
	if err := persistSalvagedArtifact("", "judged", "repaired"); err != nil {
		t.Errorf("empty ArtifactPath must be a no-op success, got %v", err)
	}
	// The re-read's own failure arm: a path that vanished under the gate is
	// unreadable, and an unreadable artifact must refuse (fail closed) rather
	// than blind-write the repair over whatever appears there next.
	gone := filepath.Join(t.TempDir(), "vanished.md")
	if err := persistSalvagedArtifact(gone, "judged", "repaired"); err == nil {
		t.Error("an unreadable artifact must refuse the write-back, not blind-write over it")
	}
	if _, err := os.Stat(gone); err == nil {
		t.Error("the refusal created the file it refused to reconcile")
	}
}
