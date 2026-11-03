package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func personaOrchestrator(dir string) *Orchestrator {
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(_, id string) string { return filepath.Join(dir, id+".json") }))
}

func personaTDDWorkspace(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	writeItem(t, dir, "alpha", `{"id":"alpha","title":"Alpha","acceptance":["alpha criterion one","alpha criterion two"]}`)
	ws := t.TempDir()
	writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":["alpha"]}`)
	return dir, ws
}

func personaBuildWorktree(t *testing.T, cycle int, body string) string {
	t.Helper()
	wt := t.TempDir()
	pkg := filepath.Join(wt, "go", "acs", fmt.Sprintf("cycle%d", cycle))
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(pkg, "a_test.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return wt
}

func TestSeedTaskContract_TDDPromptCarriesTheTriageTDDRequestBlockWithAskIDs(t *testing.T) {
	t.Parallel()

	dir, ws := personaTDDWorkspace(t)
	want, err := request.TriageTDD(ws, func(id string) string { return filepath.Join(dir, id+".json") })
	if err != nil {
		t.Fatal(err)
	}
	o := personaOrchestrator(dir)
	got := o.seedTaskContract(context.Background(), map[string]string{}, PhaseTDD, CycleState{WorkspacePath: ws}, dir)[CtxKeyTaskContract]
	for _, need := range []string{
		"### Handoff request (ADR-0141)",
		"pair: triage-tdd",
		"request_digest: " + want.Digest(),
		"ledger: " + filepath.Join(ws, "handoff-triage-tdd.json"),
		"- alpha#1 [required] alpha criterion one (evidence: predicate)",
		"- alpha#2 [required] alpha criterion two (evidence: predicate)",
		"Never write `author`",
	} {
		if !strings.Contains(got, need) {
			t.Errorf("the tdd prompt lacks %q:\n%s", need, got)
		}
	}
}

func TestSeedTaskContract_BuildPromptCarriesTheTDDBuildRequestBlock(t *testing.T) {
	t.Parallel()

	dir, ws := personaTDDWorkspace(t)
	wt := personaBuildWorktree(t, 7, "package cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_Thing(t *testing.T) {}\n")
	o := personaOrchestrator(dir)
	o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{} }
	got := o.seedTaskContract(context.Background(), map[string]string{}, PhaseBuild, CycleState{WorkspacePath: ws, ActiveWorktree: wt, CycleID: 7}, dir)[CtxKeyTaskContract]
	for _, need := range []string{"pair: tdd-build", "ledger: " + filepath.Join(ws, "handoff-tdd-build.json"), "- TestC7_001_Thing [required] TestC7_001_Thing (evidence: green_run)"} {
		if !strings.Contains(got, need) {
			t.Errorf("the build prompt lacks %q:\n%s", need, got)
		}
	}
}

func TestSeedTaskContract_BuildWithoutPredicatesNamesWhyThereIsNoRequest(t *testing.T) {
	t.Parallel()

	dir, ws := personaTDDWorkspace(t)
	wt := personaBuildWorktree(t, 8, "")
	o := personaOrchestrator(dir)
	o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{} }
	got := o.seedTaskContract(context.Background(), map[string]string{}, PhaseBuild, CycleState{WorkspacePath: ws, ActiveWorktree: wt, CycleID: 8}, dir)[CtxKeyTaskContract]
	if !strings.Contains(got, "Handoff request: none for pair tdd-build (") || strings.Contains(got, "request_digest:") {
		t.Errorf("a build without predicates must say why it has no request:\n%s", got)
	}
}

func TestSeedTaskContract_AuditPromptCarriesNoHandoffBlock(t *testing.T) {
	t.Parallel()

	dir, ws := personaTDDWorkspace(t)
	o := personaOrchestrator(dir)
	o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{} }
	got := o.seedTaskContract(context.Background(), map[string]string{}, PhaseAudit, CycleState{WorkspacePath: ws}, dir)[CtxKeyTaskContract]
	if strings.Contains(got, "Handoff request") {
		t.Errorf("audit writes no persona ledger:\n%s", got)
	}
}

func TestPersonaLedgerBuiltFromTheTDDBlockPassesValidate(t *testing.T) {
	t.Parallel()

	dir, ws := personaTDDWorkspace(t)
	req, err := request.TriageTDD(ws, func(id string) string { return filepath.Join(dir, id+".json") })
	if err != nil {
		t.Fatal(err)
	}
	block := handoffRequestBlock(PhaseTDD, CycleState{WorkspacePath: ws}, func(id string) string { return filepath.Join(dir, id+".json") })
	var digest string
	for _, line := range strings.Split(block, "\n") {
		if v, ok := strings.CutPrefix(line, "request_digest: "); ok {
			digest = v
		}
	}
	ledger := handoff.Ledger{SchemaVersion: 1, Pair: request.PairTriageTDD, RequestDigest: digest, Round: 1}
	for _, ask := range req.Asks {
		ledger.Entries = append(ledger.Entries, handoff.Entry{Ask: ask.ID, Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "predicate", Ref: "TestC1_001"}}})
	}
	raw, _ := json.Marshal(ledger)
	parsed, err := handoff.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := handoff.Validate(parsed, req); err != nil {
		t.Fatalf("a ledger copied from the block must validate: %v", err)
	}
}

func personaReviewInput(t *testing.T, phase string, ledger string, records ...llmcalls.Record) (ReviewInput, string, *bytes.Buffer) {
	t.Helper()
	ws := t.TempDir()
	pair := handoffLedgerPairOf[phase]
	if ledger != "" {
		writeWSFile(t, ws, "handoff-"+pair+".json", ledger)
	}
	for _, rec := range records {
		if err := llmcalls.AppendWorkspace(ws, rec); err != nil {
			t.Fatal(err)
		}
	}
	var errBuf bytes.Buffer
	return ReviewInput{Cycle: 4, Phase: phase, Workspace: ws, Stderr: &errBuf}, filepath.Join(ws, "handoff-"+pair+".json"), &errBuf
}

func personaReview(in ReviewInput) {
	HandoffReviewer{config: policy.HandoffConfig{}, check: func(ReviewInput, string) handoff.Check { return handoff.Check{} }}.Review(context.Background(), in)
}

func TestHandoffReviewer_ReplacesAnAgentWrittenAuthorWithTheDispatchRecord(t *testing.T) {
	t.Parallel()

	for _, phase := range []string{"triage", "tdd", "build"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			older := llmcalls.Record{TS: "t1", Phase: phase, Cycle: 4, CLI: "old", Model: "old-model"}
			dispatched := llmcalls.Record{TS: "t2", Phase: phase, Cycle: 4, CLI: "claude", Model: llmcalls.UnknownModel, DispatchedModel: "", RequestedModel: "claude-opus-5-5"}
			other := llmcalls.Record{TS: "t3", Phase: "scout", Cycle: 4, CLI: "other"}
			wrongCycle := llmcalls.Record{TS: "t4", Phase: phase, Cycle: 3, CLI: "stale"}
			in, path, _ := personaReviewInput(t, phase, `{"schema_version":1,"pair":"x","round":2,"author":{"cli":"liar","model":"liar","tier":"liar"},"entries":[]}`, older, dispatched, other, wrongCycle)
			personaReview(in)
			var got struct {
				Author handoff.Author `json:"author"`
				Round  int            `json:"round"`
				Pair   string         `json:"pair"`
			}
			raw, _ := os.ReadFile(path)
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if got.Author != (handoff.Author{CLI: "claude", Model: "claude-opus-5-5", Tier: "claude-opus-5-5"}) {
				t.Errorf("author = %+v", got.Author)
			}
			if got.Round != 2 || got.Pair != "x" {
				t.Errorf("the other keys must survive: %s", raw)
			}
		})
	}
}

func TestHandoffReviewer_AMissingLedgerStaysAbsentAndGetsNoAuthor(t *testing.T) {
	t.Parallel()

	in, path, errBuf := personaReviewInput(t, "tdd", "", llmcalls.Record{TS: "t", Phase: "tdd", Cycle: 4, CLI: "claude", Model: "m"})
	stampHandoffAuthor(in, "")
	if _, err := os.Stat(path); !os.IsNotExist(err) || errBuf.Len() != 0 {
		t.Errorf("no stamp may create a ledger: stat=%v stderr=%q", err, errBuf)
	}
	personaReview(in)
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "author") || !handoff.IsCheckOnly(raw) {
		t.Errorf("the absent path must keep only the check record: %s", raw)
	}
}

func TestHandoffReviewer_NoDispatchRecordLeavesTheLedgerAsWritten(t *testing.T) {
	t.Parallel()

	const ledger = `{"schema_version":1,"author":{"cli":"agent"}}`
	for name, records := range map[string][]llmcalls.Record{
		"no_ledger_file":   nil,
		"no_matching_call": {{TS: "t", Phase: "scout", Cycle: 4, CLI: "x"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in, path, errBuf := personaReviewInput(t, "build", ledger, records...)
			stampHandoffAuthor(in, "")
			raw, _ := os.ReadFile(path)
			if string(raw) != ledger || errBuf.Len() != 0 {
				t.Errorf("ledger %s stderr %q", raw, errBuf)
			}
		})
	}
}

func TestHandoffReviewer_AServedModelIsKeptOverTheRequestedOne(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "tdd", `{"schema_version":1}`, llmcalls.Record{TS: "t", Phase: "tdd", Cycle: 4, CLI: "claude", Model: "served", RequestedModel: "asked"})
	stampHandoffAuthor(in, "")
	var got struct {
		Author handoff.Author `json:"author"`
	}
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &got); err != nil || got.Author != (handoff.Author{CLI: "claude", Model: "served", Tier: "asked"}) {
		t.Errorf("author = %s (%v)", raw, err)
	}
}

func TestHandoffReviewer_ANonObjectLedgerKeepsItsBytesAndWarns(t *testing.T) {
	t.Parallel()

	in, path, errBuf := personaReviewInput(t, "tdd", `["not","an","object"]`, llmcalls.Record{TS: "t", Phase: "tdd", Cycle: 4, CLI: "c", Model: "m"})
	stampHandoffAuthor(in, "")
	raw, _ := os.ReadFile(path)
	if string(raw) != `["not","an","object"]` || !strings.Contains(errBuf.String(), "the author was not stamped") {
		t.Errorf("ledger %s stderr %q", raw, errBuf)
	}
}

func TestHandoffReviewer_OtherPhasesAreNotStamped(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	writeWSFile(t, ws, "handoff-tdd-build.json", `{"schema_version":1}`)
	if err := llmcalls.AppendWorkspace(ws, llmcalls.Record{TS: "t", Phase: "audit", Cycle: 4, CLI: "c", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	stampHandoffAuthor(ReviewInput{Cycle: 4, Phase: "audit", Workspace: ws}, "")
	raw, _ := os.ReadFile(filepath.Join(ws, "handoff-tdd-build.json"))
	if string(raw) != `{"schema_version":1}` {
		t.Errorf("audit must not stamp: %s", raw)
	}
}

func TestHandoffReviewer_UnreadableLedgerOrDispatchRecordWarnsAndWritesNoStamp(t *testing.T) {
	t.Parallel()

	t.Run("ledger_is_a_directory", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		if err := os.Mkdir(filepath.Join(ws, "handoff-triage-tdd.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		var errBuf bytes.Buffer
		stampHandoffAuthor(ReviewInput{Cycle: 4, Phase: "tdd", Workspace: ws, Stderr: &errBuf}, "")
		if !strings.Contains(errBuf.String(), "the author was not stamped") {
			t.Errorf("stderr %q", errBuf.String())
		}
	})
	t.Run("dispatch_record_is_a_directory", func(t *testing.T) {
		t.Parallel()
		in, path, errBuf := personaReviewInput(t, "tdd", `{"schema_version":1}`)
		if err := os.Mkdir(llmcalls.Path(in.Workspace), 0o755); err != nil {
			t.Fatal(err)
		}
		stampHandoffAuthor(in, "")
		raw, _ := os.ReadFile(path)
		if string(raw) != `{"schema_version":1}` || !strings.Contains(errBuf.String(), "the author was not stamped") {
			t.Errorf("ledger %s stderr %q", raw, errBuf)
		}
	})
}

func authorOf(t *testing.T, path string) handoff.Author {
	t.Helper()
	var got struct {
		Author handoff.Author `json:"author"`
	}
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got.Author
}

func ledgerField(t *testing.T, path, key string) string {
	t.Helper()
	var fields map[string]any
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	v, _ := fields[key].(string)
	return v
}

func TestHandoffReviewer_ADispatchedModelWinsWhenServedModelIsUnknown(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "tdd", `{"schema_version":1}`, llmcalls.Record{
		TS: "t", Phase: "tdd", Cycle: 4, CLI: "claude",
		Model: llmcalls.UnknownModel, DispatchedModel: "claude-sonnet-4-6", RequestedModel: "sonnet",
	})
	stampHandoffAuthor(in, "")
	if got := authorOf(t, path); got != (handoff.Author{CLI: "claude", Model: "claude-sonnet-4-6", Tier: "sonnet"}) {
		t.Errorf("author = %+v, want DispatchedModel claude-sonnet-4-6", got)
	}
}

func TestHandoffReviewer_StampsTheOutputDigestOnTheFirstReview(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "build", `{"schema_version":1}`, llmcalls.Record{TS: "t", Phase: "build", Cycle: 4, CLI: "c", Model: "m"})
	if stale := stampHandoffAuthor(in, "tree-1"); stale != "" {
		t.Errorf("stale = %q", stale)
	}
	if got := ledgerField(t, path, "output_digest"); got != "tree-1" {
		t.Errorf("output_digest = %q", got)
	}
}

func TestHandoffReviewer_StampsTheDigestEvenWithoutADispatchRecord(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "build", `{"schema_version":1}`)
	stampHandoffAuthor(in, "tree-1")
	if got := ledgerField(t, path, "output_digest"); got != "tree-1" {
		t.Errorf("output_digest = %q, the digest stands alone when no record exists", got)
	}
}

func TestHandoffReviewer_ADigestOfAnotherTreeIsReportedStaleAndKept(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "build", `{"schema_version":1,"output_digest":"tree-old"}`)
	if stale := stampHandoffAuthor(in, "tree-2"); stale != "tree-old" {
		t.Errorf("stale = %q, want tree-old", stale)
	}
	if got := ledgerField(t, path, "output_digest"); got != "tree-old" {
		t.Errorf("a stale digest must not be rewritten: %q", got)
	}
}

func TestHandoffReviewer_ADigestOfTheCurrentTreeIsNotStale(t *testing.T) {
	t.Parallel()

	in, _, _ := personaReviewInput(t, "build", `{"schema_version":1,"output_digest":"tree-2"}`)
	if stale := stampHandoffAuthor(in, "tree-2"); stale != "" {
		t.Errorf("stale = %q", stale)
	}
}

func TestHandoffReviewer_AnEmptyTreeDigestLeavesTheLedgerUnchanged(t *testing.T) {
	t.Parallel()

	const ledger = `{"schema_version":1,"output_digest":"tree-old"}`
	in, path, _ := personaReviewInput(t, "build", ledger)
	if stale := stampHandoffAuthor(in, ""); stale != "" {
		t.Errorf("stale = %q", stale)
	}
	if raw, _ := os.ReadFile(path); string(raw) != ledger {
		t.Errorf("ledger %s", raw)
	}
}

func TestHandoffReviewer_ARound2RewriteWithoutADigestIsStampedFresh(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "build", `{"schema_version":1,"output_digest":"tree-1"}`)
	stampHandoffAuthor(in, "tree-1")
	writeWSFile(t, in.Workspace, "handoff-tdd-build.json", `{"schema_version":1,"round":2}`)
	if stale := stampHandoffAuthor(in, "tree-2"); stale != "" {
		t.Errorf("stale = %q", stale)
	}
	if got := ledgerField(t, path, "output_digest"); got != "tree-2" {
		t.Errorf("output_digest = %q, want tree-2", got)
	}
}

func staleReviewer(tree string, outcome string) HandoffReviewer {
	return HandoffReviewer{
		config: policy.HandoffConfig{request.PairTDDBuild: {Stage: "enforce"}},
		check:  func(ReviewInput, string) handoff.Check { return handoff.Check{Outcome: outcome} },
		tree:   func(ReviewInput) string { return tree },
	}
}

func TestHandoffReviewer_AStaleDigestMakesTheCheckInvalidAndNamesIt(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "build", `{"schema_version":1,"output_digest":"tree-old"}`)
	res := staleReviewer("tree-new", string(handoff.ExitPass)).Review(context.Background(), in)
	if res.Approve || !res.Retry || !strings.Contains(res.Remediation, "tree-old") {
		t.Errorf("a stale digest must reject in enforce and name the digest: %+v", res)
	}
	var recorded struct {
		Check handoff.Check `json:"check"`
	}
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &recorded); err != nil || recorded.Check.Outcome != "invalid" {
		t.Errorf("the recorded check must be invalid: %s (%v)", raw, err)
	}
}

func TestHandoffReviewer_AFreshDigestKeepsThePassingCheck(t *testing.T) {
	t.Parallel()

	in, path, _ := personaReviewInput(t, "build", `{"schema_version":1}`)
	res := staleReviewer("tree-new", string(handoff.ExitPass)).Review(context.Background(), in)
	if !res.Approve || ledgerField(t, path, "output_digest") != "tree-new" {
		t.Errorf("res %+v", res)
	}
}

func TestHandoffReviewer_TheDefaultTreeDigestIsEmptyWithoutAWorktree(t *testing.T) {
	t.Parallel()

	if got := (HandoffReviewer{}).outputTree(context.Background(), ReviewInput{}); got != "" {
		t.Errorf("tree = %q", got)
	}
}
