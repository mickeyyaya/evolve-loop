package site

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
)

const (
	coverAsk    = "TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage"
	guardAsk    = "TestC1950_002_BatchAddsNoProductionEdits"
	breakerPath = "go/internal/core/blocker_breaker.go"
	conflict    = "BLOCKER predicate-conflict " + coverAsk + " " + guardAsk + " go/internal/core/blocker_breaker.go:76-77 001 demands the dead branch covered and 002 forbids the production edit that removes it"
	floorFail   = "./acs/cycle1950 (-tags acs): unit tests FAIL\n" +
		"--- FAIL: " + coverAsk + " (1.19s)\n" +
		"    predicates_test.go:74: blocker_breaker.go:AppendResolvedFingerprint is at 95.2% under the ^TestCoreCoverageBatch_ batch tests; want 100.0% (error branches included)\n" +
		"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1950\t1.471s\nFAIL\n"
)

type project struct {
	t    *testing.T
	root string
}

func newProject(t *testing.T) project {
	t.Helper()
	return project{t: t, root: t.TempDir()}
}

func (p project) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.root, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = os.WriteFile(path, []byte(body), 0o644)
	}
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p project) runDir(cycle int) string {
	return filepath.Join(p.root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
}

func (p project) site() Site {
	return Site{RunDir: p.runDir(1950), InboxRoot: p.root, Tree: p.root}
}

func (p project) cycle1950() handoff.Request {
	p.t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "request", "testdata", "cycle1950", "predicates.go.fixture"))
	if err != nil {
		p.t.Fatal(err)
	}
	p.write("go/acs/cycle1950/predicates_test.go", string(body))
	p.write(breakerPath, "package core\n")
	req, err := request.TDDBuild(filepath.Join(p.root, "go", "acs", "cycle1950"))
	if err != nil {
		p.t.Fatal(err)
	}
	return req
}

func (p project) ledger(req handoff.Request, digest string, entries ...handoff.Entry) {
	p.t.Helper()
	body, err := json.Marshal(handoff.Ledger{SchemaVersion: handoff.SchemaVersion, Pair: req.Pair, RequestDigest: digest, OutputDigest: "sha256:output", Round: 1, Entries: entries})
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1950/handoff-"+req.Pair+".json", string(body))
}

func (p project) failures(pair string, failures ...string) {
	p.t.Helper()
	body, err := json.Marshal(failures)
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1950/handoff-"+pair+"-failures.json", string(body))
}

func metPredicate(ask string) handoff.Entry {
	return handoff.Entry{Ask: ask, Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "predicate", Ref: "go/acs/cycle1950/predicates_test.go:" + ask}}}
}

func TestEvaluate_AFullyMetLedgerPasses(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, req.Digest(), metPredicate(coverAsk), metPredicate(guardAsk))

	got := Evaluate(p.site(), 1950, "tdd-build")

	if got.Outcome != "pass" || got.RequestAbsent || got.LedgerAbsent || len(got.Errors) != 0 {
		t.Fatalf("evaluation = %+v, want a clean pass", got)
	}
	if got.Request.Pair != "tdd-build" || len(got.Ledger.Entries) != 2 {
		t.Errorf("request %q and %d ledger entries; want the request and the ledger kept", got.Request.Pair, len(got.Ledger.Entries))
	}
}

func TestEvaluate_AnUnmetAskAndAnEvidenceGapAreRetry(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, req.Digest(), handoff.Entry{Ask: coverAsk, Status: handoff.StatusMet}, metPredicate(guardAsk))

	got := Evaluate(p.site(), 1950, "tdd-build")

	if got.Outcome != "retry" || strings.Join(got.Unmet, ",") != coverAsk {
		t.Fatalf("evaluation = %+v, want retry on %s", got, coverAsk)
	}
	if len(got.FailedEvidence) != 1 || !strings.Contains(got.FailedEvidence[0], "a met entry cites no evidence") {
		t.Errorf("failed evidence = %v", got.FailedEvidence)
	}
}

func TestEvaluate_AProvenBlockerIsBlockedWithItsPath(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, req.Digest(), handoff.Entry{Ask: coverAsk, Status: handoff.StatusBlocked, Note: conflict}, metPredicate(guardAsk))
	p.failures("tdd-build", floorFail)

	got := Evaluate(p.site(), 1950, "tdd-build")

	if got.Outcome != "blocked" || len(got.Blocked) == 0 || !strings.Contains(strings.Join(got.Paths, " "), breakerPath) {
		t.Fatalf("evaluation = %+v, want blocked with %s", got, breakerPath)
	}
}

func TestEvaluate_AStaleRequestDigestIsInvalid(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, "sha256:other", metPredicate(coverAsk), metPredicate(guardAsk))

	got := Evaluate(p.site(), 1950, "tdd-build")

	if got.Outcome != "invalid" || len(got.Errors) == 0 {
		t.Fatalf("evaluation = %+v, want invalid with errors", got)
	}
}

func TestEvaluate_AStaleOutputForcesInvalidWhateverTheOutcome(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, req.Digest(), metPredicate(coverAsk), metPredicate(guardAsk))
	s := p.site()
	s.Stale = "sha256:old"

	got := Evaluate(s, 1950, "tdd-build")

	if got.Outcome != "invalid" || !strings.Contains(strings.Join(got.Errors, " "), "sha256:old is stale") {
		t.Fatalf("evaluation = %+v, want invalid naming the stale digest", got)
	}
}

func TestEvaluate_ALedgerProblemIsAbsentOnlyWhenTheFileIsMissingOrCheckOnly(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.cycle1950()
	got := Evaluate(p.site(), 1950, "tdd-build")
	if got.Outcome != "error" || !got.LedgerAbsent || got.RequestAbsent {
		t.Fatalf("missing ledger: %+v", got)
	}
	p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", `{"schema_version":1,"pair":"tdd-build","check":{"stage":"shadow","outcome":"error"}}`)
	got = Evaluate(p.site(), 1950, "tdd-build")
	if !got.LedgerAbsent || !strings.Contains(strings.Join(got.Errors, " "), "only the check record") {
		t.Fatalf("check-only ledger: %+v", got)
	}
	p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", `{not json`)
	got = Evaluate(p.site(), 1950, "tdd-build")
	if got.Outcome != "invalid" || got.LedgerAbsent {
		t.Fatalf("corrupt ledger: %+v", got)
	}
}

func TestEvaluate_AFailuresFileProblemIsAnError(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, req.Digest(), metPredicate(coverAsk), metPredicate(guardAsk))
	p.write(".evolve/runs/cycle-1950/handoff-tdd-build-failures.json", "{not json")

	got := Evaluate(p.site(), 1950, "tdd-build")

	if got.Outcome != "error" || len(got.Errors) == 0 || got.LedgerAbsent || got.RequestAbsent {
		t.Fatalf("evaluation = %+v, want a plain error", got)
	}
}

func TestEvaluate_ARequestThatCannotExistIsAbsentAndAnyOtherRequestErrorIsNot(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	if got := Evaluate(p.site(), 1950, "tdd-build"); !got.RequestAbsent || got.Outcome != "error" {
		t.Errorf("no ACS directory: %+v, want RequestAbsent", got)
	}
	if got := Evaluate(p.site(), 1950, "triage-tdd"); !got.RequestAbsent {
		t.Errorf("no committed set: %+v, want RequestAbsent", got)
	}
	if got := Evaluate(p.site(), 1950, "bogus"); got.RequestAbsent || got.Outcome != "error" || !strings.Contains(strings.Join(got.Errors, " "), "no request for pair bogus") {
		t.Errorf("bogus pair: %+v, want a plain request error", got)
	}
}

func TestBuildRequest_APairWithoutARequestIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := BuildRequest(Site{}, 1, "bogus"); !errors.Is(err, request.ErrNoRequestForPair) {
		t.Fatalf("err = %v, want ErrNoRequestForPair", err)
	}
}

func TestForPhase_TheWorktreeFallsBackToTheProjectRoot(t *testing.T) {
	t.Parallel()
	if got := ForPhase("ws", "root", ""); got != (Site{RunDir: "ws", InboxRoot: "root", Tree: "root"}) {
		t.Errorf("fallback site = %+v", got)
	}
	if got := ForPhase("ws", "root", "tree"); got.Tree != "tree" {
		t.Errorf("site = %+v, want the worktree as the tree", got)
	}
}

func TestInboxResolver_FindsAnItemAndReturnsEmptyForAMissingOne(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.write(".evolve/inbox/2026-10-09-real-id.json", `{"id":"real-id"}`)
	p.write(".evolve/inbox/2026-10-09-surreal-id.json", `{"id":"surreal-id"}`)
	p.write(".evolve/inbox/2026-07-20-short-name.json", `{"id":"a-longer-recorded-id"}`)
	p.write(".evolve/inbox/glued-id.json", "{}")
	resolve := InboxResolver(p.root)
	tests := map[string]string{
		"real-id":              filepath.Join(p.root, ".evolve", "inbox", "2026-10-09-real-id.json"),
		"a-longer-recorded-id": filepath.Join(p.root, ".evolve", "inbox", "2026-07-20-short-name.json"),
		"missing-id":           "",
		"d":                    "",
		"glued-id":             filepath.Join(p.root, ".evolve", "inbox", "glued-id.json"),
	}
	for id, want := range tests {
		if got := resolve(id); got != want {
			t.Errorf("resolve(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestPredicateSources_AnUnreadableFileIsNamed(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.write("go/acs/cycle1950/good_test.go", "package x\n")
	if err := os.Mkdir(filepath.Join(p.root, "go", "acs", "cycle1950", "bad.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := predicateSources(p.root, 1950)
	if got["good_test.go"] != "package x\n" || !strings.HasPrefix(got["bad.go"], "unreadable: ") {
		t.Errorf("sources = %v", got)
	}
}

func TestEvaluate_AFailuresPathThatIsADirectoryIsAnError(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	p.ledger(req, req.Digest(), metPredicate(coverAsk), metPredicate(guardAsk))
	if err := os.MkdirAll(filepath.Join(p.runDir(1950), "handoff-tdd-build-failures.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(p.site(), 1950, "tdd-build"); got.Outcome != "error" || len(got.Errors) == 0 {
		t.Fatalf("evaluation = %+v, want an error", got)
	}
}

func TestEvaluate_AMetAskThatCitesAGhostPredicateIsUnresolved(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	req := p.cycle1950()
	ghost := handoff.Entry{Ask: coverAsk, Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "predicate", Ref: "go/acs/cycle1950/predicates_test.go:TestC1950_999_Ghost"}}}
	p.ledger(req, req.Digest(), ghost, metPredicate(guardAsk))
	got := Evaluate(p.site(), 1950, "tdd-build")
	if got.Outcome != "retry" || !strings.Contains(strings.Join(got.FailedEvidence, " "), "TestC1950_999_Ghost") {
		t.Fatalf("evaluation = %+v, want retry naming the ghost", got)
	}
}

func TestEvaluate_AnUnreadableLedgerIsAnErrorNotAnAbsentLedger(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.cycle1950()
	if err := os.MkdirAll(filepath.Join(p.runDir(1950), "handoff-tdd-build.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Evaluate(p.site(), 1950, "tdd-build")
	if got.LedgerAbsent || got.RequestAbsent || got.Outcome != "error" || len(got.Errors) == 0 {
		t.Errorf("evaluation = %+v, want a plain read error that is not an absent ledger", got)
	}
}
