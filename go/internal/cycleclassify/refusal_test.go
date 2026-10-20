package cycleclassify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func writePhaseTiming(t *testing.T, ws string, entries []phasetiming.Entry) {
	t.Helper()
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phasetiming.Path(ws), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func refusalEntries(code string) []phasetiming.Entry {
	return []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS"},
		{Phase: "triage", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "top_n card names protected surface", Code: code}}},
	}
}

func TestClassify_CodedRefusalOnTheLastOutcome_IsPhaseRefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	r := Classify(ws)
	if r.Class != ClassPhaseRefusal || r.Marker != cyclestate.DiagCodeTriageProtectedSurface || r.Source != phasetiming.FileName {
		t.Errorf("Classify = %+v", r)
	}
}

func TestClassify_UncodedFailOnTheRecord_IsNotARefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(""))
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("an uncoded FAIL is prose, not a refusal: %+v", r)
	}
}

func TestClassify_RefusalMustBeTheLastOutcome(t *testing.T) {
	ws := t.TempDir()
	entries := append(refusalEntries(cyclestate.DiagCodeTriageTopNEmpty), phasetiming.Entry{Phase: "build", Verdict: "PASS"})
	writePhaseTiming(t, ws, entries)
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("a refusal the cycle recovered from did not stop it: %+v", r)
	}
}

func TestClassify_CodedRefusalOutranksTheRegexPasses(t *testing.T) {
	ws := writeReport(t, "## Verdict\nAUDIT_FAIL: the auditor said no\n")
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	if r := Classify(ws); r.Class != ClassPhaseRefusal {
		t.Errorf("the structured record outranks prose: %+v", r)
	}
}

func TestClassify_SentinelClassStillOutranksTheRefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	sentinel := "# triage\n<!-- evolve-verdict: {\"phase\":\"triage\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"infrastructure\"}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("pass 0 (the ending phase's own sentinel class) keeps winning: %+v", r)
	}
}

func TestClassify_StaleEarlierSentinelDoesNotOutrankTheRefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	sentinel := "# scout\n<!-- evolve-verdict: {\"phase\":\"scout\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"infrastructure\"}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := Classify(ws); r.Class != ClassPhaseRefusal {
		t.Errorf("the record's last outcome outranks an earlier phase's sentinel: %+v", r)
	}
}

func TestClassify_UnreadableRecord_FallsThrough(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(phasetiming.Path(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("a corrupt record is not a refusal: %+v", r)
	}
}

func TestClassify_AHostProvenBlockerOutranksTheLanesOwnFailSentinel(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, []phasetiming.Entry{
		{Phase: "tdd", Verdict: "PASS"},
		{Phase: "build", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "blocked: needs console", Code: cyclestate.DiagCodeBlockerRoute, Subject: "item-1"}}},
	})
	sentinel := "# build\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,\"failure\":{\"class\":\"code-build-fail\",\"defects\":[\"BLOCKER protected-path go/x.go why\"]}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Classify(ws)
	if r.Class != ClassPhaseRefusal || r.Marker != cyclestate.DiagCodeBlockerRoute || r.Subject != "item-1" || r.Phase != "build" {
		t.Errorf("the host's proof of the lane's claim supersedes the lane's FAIL: %+v", r)
	}
}

func rejectedBuildBeforeTheRetro(code, retroVerdict string) []phasetiming.Entry {
	return []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS"},
		{Phase: "tdd", Verdict: "PASS"},
		{Phase: "build", Verdict: "FAIL", AbortReason: `review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor`, Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "rejected", Code: code, Subject: "item-1"}}},
		{Phase: string(cyclestate.PhaseRetro), Verdict: retroVerdict},
	}
}

func TestClassify_ARejectedBuildClassifiesFromItsOwnEntryAndNotTheRetroAfterIt(t *testing.T) {
	for _, code := range []string{cyclestate.DiagCodeBuildRejected, cyclestate.DiagCodeBlockerRoute} {
		for _, retro := range []string{"PASS", "FAIL"} {
			ws := t.TempDir()
			writePhaseTiming(t, ws, rejectedBuildBeforeTheRetro(code, retro))

			r := Classify(ws)

			if r.Class != ClassPhaseRefusal || r.Marker != code || r.Phase != "build" || r.Subject != "item-1" {
				t.Errorf("code %s, retro %s: Classify = %+v; the retro runs after the failing phase and does not decide its class", code, retro, r)
			}
		}
	}
}

func TestClassify_ARetroAloneIsNotARefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, []phasetiming.Entry{{Phase: string(cyclestate.PhaseRetro), Verdict: "PASS"}})
	if r := Classify(ws); r.Class != ClassIntegrityBreach {
		t.Errorf("a record of only the retro holds no failing phase, so the record pass falls through: %+v", r)
	}
}

func TestClassify_ACodeWithNoDispositionBeforeTheRetroFallsThroughToTheReport(t *testing.T) {
	ws := writeReport(t, "## Verdict\nAUDIT_FAIL: Verdict FAIL\n")
	writePhaseTiming(t, ws, []phasetiming.Entry{
		{Phase: "build", Verdict: "PASS"},
		{Phase: "audit", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "x", Code: "AUDIT_SOMETHING_NEW"}}},
		{Phase: string(cyclestate.PhaseRetro), Verdict: "PASS"},
	})

	if r := Classify(ws); r.Class != ClassAuditFail {
		t.Errorf("Classify = %+v; past the retro only a code with a disposition is a refusal, so the audit FAIL keeps its class", r)
	}
}

func TestClassify_TheRejectedBuildsOwnFailSentinelStillWins(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, rejectedBuildBeforeTheRetro(cyclestate.DiagCodeBuildRejected, "PASS"))
	sentinel := "# build\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"code-build-fail\"}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}

	if r := Classify(ws); r.Class != Classification("code-build-fail") || r.Source != "build-report.md" {
		t.Errorf("Classify = %+v; the build's own FAIL sentinel names the class, which also charges the item", r)
	}
}

func infraEnvelope(phase, marker, excerpt string) string {
	return fmt.Sprintf(`{"schema_version":"2.0","source":{"producer":"normalizer","cli":"claude-tmux","phase":%q,"agent":%q},"kind":"infra_failure","severity":"INCIDENT","data":{"excerpt":%q,"marker":%q,"source":"stdout"}}`+"\n", phase, phase, excerpt, marker)
}

const zshCompinitExcerpt = "⎿  compinit:527: operation not permitted: /opt/homebrew/share/zsh/site-functions/_docker-compose"

func TestClassify_ABuildRejectedBecauseOfInfrastructureStaysInfrastructure(t *testing.T) {
	for name, tc := range map[string]struct {
		file, body, source string
	}{
		"a rate limit":                   {"build-events.ndjson", infraEnvelope("build", "api_429", "HTTP 429 Too Many Requests"), "build-events.ndjson"},
		"a 529":                          {"build-events.ndjson", infraEnvelope("build", "api_529", "API Error 529 Overloaded"), "build-events.ndjson"},
		"sandbox EPERM":                  {"build-events.ndjson", infraEnvelope("build", "eperm", "sandbox-exec: sandbox_apply: Operation not permitted"), "build-events.ndjson"},
		"a 529 beside the compinit line": {"build-events.ndjson", infraEnvelope("build", "eperm", zshCompinitExcerpt+"\nAPI Error 529 Overloaded"), "build-events.ndjson"},
		"empty output":                   {"build-stdout.log", "", "build-stdout.log"},
	} {
		t.Run(name, func(t *testing.T) {
			ws := t.TempDir()
			writePhaseTiming(t, ws, rejectedBuildBeforeTheRetro(cyclestate.DiagCodeBuildRejected, "PASS"))
			writeFile(t, ws, tc.file, tc.body)

			if r := Classify(ws); r.Class != ClassInfrastructure || r.Source != tc.source {
				t.Fatalf("Classify = %+v, want infrastructure from %s; the build's own dispatch hit infrastructure, so the item is not charged", r, tc.source)
			}
		})
	}
}

func TestClassify_ARejectedBuildIsChargedDespiteTheZshCompinitLineAndOtherPhasesInfra(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, rejectedBuildBeforeTheRetro(cyclestate.DiagCodeBuildRejected, "PASS"))
	writeFile(t, ws, "build-events.ndjson", infraEnvelope("build", "eperm", zshCompinitExcerpt)+infraEnvelope("build", "eperm", "[acs] WARN: verification single-flight unavailable (verifylock: open /x/.repo.git/evolve-verify.lock: operation not permitted)"))
	writeFile(t, ws, "build-stdout.log", "the builder worked\n")
	writeFile(t, ws, "tdd-events.ndjson", infraEnvelope("tdd", "eperm", zshCompinitExcerpt)+infraEnvelope("tdd", "api_529", "API Error 529 Overloaded"))
	writeFile(t, ws, "scout-stdout.log", "")
	writeFile(t, ws, "orchestrator-report.md", "scout: HTTP 429 Too Many Requests, retried\n")

	if r := Classify(ws); r.Class != ClassPhaseRefusal || r.Marker != cyclestate.DiagCodeBuildRejected {
		t.Fatalf("Classify = %+v; every pane prints the compinit line, and infra in another phase did not reject this build, so the item is charged", r)
	}
}

func TestClassify_TheRejectedBuildsOwnSentinelOutranksInfrastructureAsOnMain(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, rejectedBuildBeforeTheRetro(cyclestate.DiagCodeBuildRejected, "PASS"))
	writeFile(t, ws, "build-report.md", "# build\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"code-build-fail\"}} -->\n")
	writeFile(t, ws, "orchestrator-report.md", "scout: HTTP 429 Too Many Requests, retried\n")

	if r := Classify(ws); r.Class != Classification("code-build-fail") {
		t.Fatalf("Classify = %+v; the build's own FAIL sentinel names the class before any infrastructure marker, as on main", r)
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFirstInfraMarker_SkipsAMalformedLineAndANestedKind(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-events.ndjson", `{"kind":"infra_failure",`+"\n"+
		`{"kind":"progress","data":{"inner":{"kind":"infra_failure"}}}`+"\n"+
		infraEnvelope("build", "api_529", "API Error 529 Overloaded"))

	if marker, ok := firstInfraMarker(ws, filepath.Join(ws, "build-events.ndjson")); !ok || marker != "api_529" {
		t.Fatalf("firstInfraMarker = %q, %v; a broken line and a nested kind are not events, the 529 envelope is", marker, ok)
	}
}
