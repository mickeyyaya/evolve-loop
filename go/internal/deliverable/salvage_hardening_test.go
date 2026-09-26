package deliverable

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// soleFencedPass is the one shape salvage may act on: a sole, fenced, single-candidate bad_verdict that re-verifies clean.
const soleFencedPass = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

// salvageFixture returns (workspace, projectRoot, artifactPath) holding one salvageable deliverable.
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

func TestReviewerReview_ShadowStage_NoSideEffects(t *testing.T) {
	for _, stage := range []config.Stage{config.StageShadow, config.StageAdvisory} {
		t.Run(stage.String(), func(t *testing.T) {
			ws, pr, artifact := salvageFixture(t)
			sidecar := filepath.Join(pr, ".evolve", SalvageAppliedFile)
			breaker := filepath.Join(t.TempDir(), "b.json")
			// Seed a real count: resetBreaker writes nothing at zero, so an absent file would prove nothing.
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

func TestReviewerReview_PersistFailure_FailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the write-permission bit this fixture relies on")
	}
	ws, pr, _ := salvageFixture(t)

	// An unwritable workspace fails atomicwrite's rename while the artifact stays readable for the verify half.
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

func TestReviewerReview_SalvageIsBreakerNeutral(t *testing.T) {
	ws, pr, _ := salvageFixture(t)
	breaker := filepath.Join(t.TempDir(), "b.json")

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

	if n := incrBreaker(breaker); n != 3 {
		t.Errorf("salvage was not breaker-NEUTRAL: after two blocks + one salvage the next block counted %d, want 3 (a reset here hides a persistently malformed producer from both the escalation ladder and the breaker)", n)
	}
}

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

func TestSalvageVerdict_RefusesUnrepairablePayload(t *testing.T) {
	// Recoverable-looking, but broken beyond a trailing comma, so no repair yields valid JSON.
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

func TestPersistSalvagedArtifact_NoArtifactPathIsNoOp(t *testing.T) {
	if err := persistSalvagedArtifact("", "judged", "repaired"); err != nil {
		t.Errorf("empty ArtifactPath must be a no-op success, got %v", err)
	}
	// This test also covers a path that vanished under the gate: it must refuse, never blind-write.
	gone := filepath.Join(t.TempDir(), "vanished.md")
	if err := persistSalvagedArtifact(gone, "judged", "repaired"); err == nil {
		t.Error("an unreadable artifact must refuse the write-back, not blind-write over it")
	}
	if _, err := os.Stat(gone); err == nil {
		t.Error("the refusal created the file it refused to reconcile")
	}
}
