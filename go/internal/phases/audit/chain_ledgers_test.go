package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/site"
)

const (
	alphaPredicate  = "TestC7_001_Alpha"
	helperPredicate = "TestC7_900_Helper"
	acceptanceAsk   = "item#1"
	predicatesFile  = "go/acs/cycle7/predicates_test.go"
)

type ledgerFixture struct {
	t    *testing.T
	root string
	ws   string
}

func newLedgerFixture(t *testing.T, stage string) ledgerFixture {
	t.Helper()
	f := ledgerFixture{t: t, root: t.TempDir()}
	f.ws = filepath.Join(f.root, ".evolve", "runs", "cycle-7")
	f.write(".evolve/runs/cycle-7/triage-decision.json", `{"top_n":[{"id":"item"}],"deferred":[]}`)
	f.write(".evolve/inbox/2026-10-09-item.json", `{"id":"item","title":"item","acceptance":["alpha holds"]}`)
	f.write(predicatesFile, "package cycle7\n\nimport \"testing\"\n\nfunc "+alphaPredicate+"(t *testing.T) {}\n\nfunc "+helperPredicate+"(t *testing.T, n int) {}\n")
	if stage != "" {
		f.write(".evolve/policy.json", `{"handoff":{"build-audit":{"stage":"`+stage+`"}}}`)
	}
	writeACSVerdict(t, f.ws, 0)
	return f
}

func (f ledgerFixture) write(rel, body string) {
	f.t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = os.WriteFile(path, []byte(body), 0o644)
	}
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f ledgerFixture) request() core.PhaseRequest {
	return core.PhaseRequest{Cycle: 7, ProjectRoot: f.root, Workspace: f.ws}
}

func predicateEntry(ask string, status handoff.Status, ref string) handoff.Entry {
	e := handoff.Entry{Ask: ask, Status: status}
	if status == handoff.StatusMet {
		e.Evidence = []handoff.Evidence{{Kind: "predicate", Ref: ref}}
	} else {
		e.Note = "not done"
	}
	return e
}

func (f ledgerFixture) ledger(pair string, entries ...handoff.Entry) {
	f.t.Helper()
	req, err := site.BuildRequest(site.ForPhase(f.ws, f.root, ""), 7, pair)
	if err != nil {
		f.t.Fatalf("%s request: %v", pair, err)
	}
	body, err := json.Marshal(handoff.Ledger{SchemaVersion: handoff.SchemaVersion, Pair: pair, RequestDigest: req.Digest(), Round: 1, Entries: entries})
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(".evolve/runs/cycle-7/handoff-"+pair+".json", string(body))
}

func (f ledgerFixture) cleanLedgers() {
	f.ledger("triage-tdd", predicateEntry(acceptanceAsk, handoff.StatusMet, predicatesFile+":"+alphaPredicate))
	f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusMet, predicatesFile+":"+alphaPredicate))
}

func (f ledgerFixture) classify(h hooks) (string, []core.Diagnostic) {
	f.t.Helper()
	verdict, diags, _ := h.Classify(reportWithChain("PASS", fullCoherentChain()), f.request(), core.BridgeResponse{})
	return verdict, diags
}

func errorDiagnostics(diags []core.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		if d.Severity == "error" {
			b.WriteString(d.Message + "\n")
		}
	}
	return b.String()
}

func readLedgerShadow(t *testing.T, f ledgerFixture) auditchain.ShadowRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.ws, auditchain.ShadowRecordFile))
	if err != nil {
		t.Fatal(err)
	}
	var rec auditchain.ShadowRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestChainLedgers_TheFixtureBuildsBothRequests(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.cleanLedgers()
	for _, pair := range []string{"triage-tdd", "tdd-build"} {
		ev := site.Evaluate(site.ForPhase(f.ws, f.root, ""), 7, pair)
		if ev.RequestAbsent || len(ev.Errors) != 0 || ev.Outcome != "pass" {
			t.Fatalf("%s: %+v, want a built request and a pass", pair, ev)
		}
	}
}

func TestChainLedgers_AtEnforceAnUnmetRequiredAskForcesFAIL(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.ledger("triage-tdd", predicateEntry(acceptanceAsk, handoff.StatusUnmet, ""))
	f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusUnmet, ""))

	verdict, diags := f.classify(hooks{})

	errs := errorDiagnostics(diags)
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s, want FAIL\n%s", verdict, errs)
	}
	if !strings.Contains(errs, alphaPredicate) || !strings.Contains(errs, acceptanceAsk) {
		t.Errorf("error diagnostics lack the unmet ask ids:\n%s", errs)
	}
	for _, request := range []string{"no request for pair", "no committed set", "no predicate found"} {
		if strings.Contains(errs, request) {
			t.Errorf("a request error reached the diagnostics (%q):\n%s", request, errs)
		}
	}
	if !strings.Contains(errs, "1 deterministic gate(s) forced FAIL ["+ledgerGate+"]") {
		t.Errorf("two findings must record the gate once:\n%s", errs)
	}
}

func TestChainLedgers_AtEnforceAnAcceptanceWithNoMetBuildEntryForcesFAIL(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.ledger("triage-tdd", predicateEntry(acceptanceAsk, handoff.StatusMet, predicatesFile+":"+helperPredicate))
	f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusMet, predicatesFile+":"+alphaPredicate))

	verdict, diags := f.classify(hooks{})

	errs := errorDiagnostics(diags)
	if verdict != core.VerdictFAIL || !strings.Contains(errs, "inbox acceptance "+acceptanceAsk) {
		t.Fatalf("verdict = %s, want FAIL naming the acceptance\n%s", verdict, errs)
	}
	for _, other := range []string{"no request for pair", "no committed set", "no predicate found", "unmet asks: " + alphaPredicate} {
		if strings.Contains(errs, other) {
			t.Errorf("the diagnostics name a different cause (%q):\n%s", other, errs)
		}
	}
}

func TestChainLedgers_AtShadowTheVerdictDoesNotChange(t *testing.T) {
	t.Parallel()
	for name, stage := range map[string]string{"shadow": "shadow", "no policy": ""} {
		f := newLedgerFixture(t, stage)
		f.ledger("triage-tdd", predicateEntry(acceptanceAsk, handoff.StatusUnmet, ""))
		f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusUnmet, ""))

		verdict, _ := f.classify(hooks{})

		if verdict != core.VerdictPASS {
			t.Errorf("%s: verdict = %s, want PASS — shadow records, it does not decide", name, verdict)
		}
		rec := readLedgerShadow(t, f)
		if len(rec.LedgerFindings) != 2 || len(rec.OverrodeBy) != 0 {
			t.Errorf("%s: record findings=%d overrode=%v, want 2 findings and no override", name, len(rec.LedgerFindings), rec.OverrodeBy)
		}
	}
}

func TestChainLedgers_AtEnforceACleanChainKeepsPASS(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.cleanLedgers()
	verdict, diags := f.classify(hooks{})
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS\n%s", verdict, errorDiagnostics(diags))
	}
	if rec := readLedgerShadow(t, f); len(rec.LedgerFindings) != 0 || rec.LedgerNote != "" {
		t.Errorf("record = %+v, want no findings and no note", rec)
	}
}

func TestChainLedgers_AtEnforceACycleThatSkippedTDDPasses(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	if err := os.RemoveAll(filepath.Join(f.root, "go")); err != nil {
		t.Fatal(err)
	}
	verdict, diags := f.classify(hooks{})
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS\n%s", verdict, errorDiagnostics(diags))
	}
	if rec := readLedgerShadow(t, f); rec.LedgerNote != "tdd did not run" || len(rec.LedgerFindings) != 0 {
		t.Errorf("record = %+v, want the note %q", rec, "tdd did not run")
	}
}

func (f ledgerFixture) withoutCommittedSet() {
	f.t.Helper()
	if err := os.Remove(filepath.Join(f.ws, "triage-decision.json")); err != nil {
		f.t.Fatal(err)
	}
}

func TestChainLedgers_AtEnforceNoCommittedSetStillFailsAMissingBuildLedger(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.withoutCommittedSet()
	verdict, diags := f.classify(hooks{})
	errs := errorDiagnostics(diags)
	if verdict != core.VerdictFAIL || strings.Count(errs, "handoff tdd-build: the ledger is missing") != 1 {
		t.Fatalf("verdict = %s, want FAIL with one missing tdd-build ledger finding\n%s", verdict, errs)
	}
	if rec := readLedgerShadow(t, f); rec.LedgerNote == "tdd did not run" {
		t.Errorf("note = %q: TDD did run, only the committed set is absent", rec.LedgerNote)
	}
}

func TestChainLedgers_AtEnforceNoCommittedSetWithACleanBuildLedgerPassesWithItsOwnNote(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.withoutCommittedSet()
	f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusMet, predicatesFile+":"+alphaPredicate))
	verdict, diags := f.classify(hooks{})
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS\n%s", verdict, errorDiagnostics(diags))
	}
	if rec := readLedgerShadow(t, f); rec.LedgerNote != "no committed set" || len(rec.LedgerFindings) != 0 {
		t.Errorf("record = %+v, want the note %q and no finding", rec, "no committed set")
	}
}

func TestChainLedgers_NoPredicatesAndNoCommittedSetRecordsTDDDidNotRun(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.withoutCommittedSet()
	if err := os.RemoveAll(filepath.Join(f.root, "go")); err != nil {
		t.Fatal(err)
	}
	f.classify(hooks{})
	if rec := readLedgerShadow(t, f); rec.LedgerNote != "tdd did not run" {
		t.Errorf("note = %q, want %q: the missing predicates take precedence over the missing committed set", rec.LedgerNote, "tdd did not run")
	}
}

func TestChainLedgers_AtEnforceMissingTriageTDDLedgerWithPredicatesFails(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusMet, predicatesFile+":"+alphaPredicate))
	verdict, diags := f.classify(hooks{})
	errs := errorDiagnostics(diags)
	if verdict != core.VerdictFAIL || strings.Count(errs, "the ledger is missing") != 1 {
		t.Fatalf("verdict = %s, want FAIL with exactly one missing-ledger finding\n%s", verdict, errs)
	}
}

func TestChainLedgers_AtEnforceAStaleBuildLedgerForcesFAIL(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.cleanLedgers()
	var asked []string
	h := hooks{buildStale: func(_ core.PhaseRequest, ledgerPath string) string {
		asked = append(asked, filepath.Base(ledgerPath))
		return "sha256:old"
	}}

	verdict, diags := f.classify(h)

	errs := errorDiagnostics(diags)
	if verdict != core.VerdictFAIL || !strings.Contains(errs, "sha256:old is stale") {
		t.Fatalf("verdict = %s, want FAIL naming the stale digest\n%s", verdict, errs)
	}
	if len(asked) != 1 || asked[0] != "handoff-tdd-build.json" {
		t.Errorf("the stale rule asked about %v, want only handoff-tdd-build.json", asked)
	}
}

func TestChainLedgers_TriageTDDStaleDigestIsNotAFinding(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "enforce")
	f.cleanLedgers()
	h := hooks{buildStale: func(_ core.PhaseRequest, ledgerPath string) string {
		if strings.HasSuffix(ledgerPath, "handoff-triage-tdd.json") {
			return "sha256:old"
		}
		return ""
	}}
	if verdict, diags := f.classify(h); verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS\n%s", verdict, errorDiagnostics(diags))
	}
}

func TestChainLedgers_AnUnreadableStageWarnsAndStaysInShadow(t *testing.T) {
	t.Parallel()
	f := newLedgerFixture(t, "")
	f.write(".evolve/policy.json", `{not json`)
	f.ledger("triage-tdd", predicateEntry(acceptanceAsk, handoff.StatusUnmet, ""))
	f.ledger("tdd-build", predicateEntry(alphaPredicate, handoff.StatusUnmet, ""))

	verdict, diags := f.classify(hooks{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s, want PASS", verdict)
	}
	var warned bool
	for _, d := range diags {
		warned = warned || (d.Severity == "warning" && strings.Contains(d.Message, "handoff stage unreadable: the build-audit gate is in shadow"))
	}
	if !warned {
		t.Errorf("no warning for the unreadable stage: %+v", diags)
	}
}
