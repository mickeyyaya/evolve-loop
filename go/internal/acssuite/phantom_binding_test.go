package acssuite

import (
	"encoding/json"
	"strings"
	"testing"
)

const cycle1546PhantomOutput = `=== RUN   TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase
    predicates_test.go:73: binding test TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor did NOT pass in internal/core (missing, failing, or hidden behind a build tag). exit=0
combined go-test output:
testing: warning: no tests to run
PASS
ok  	github.com/mickeyyaya/evolve-loop/go/internal/core	0.451s [no tests to run]
--- FAIL: TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase (0.72s)
`

func TestPhantomBindings_RealCycle1546ShapeIsClassified(t *testing.T) {
	got := phantomBindings(cycle1546PhantomOutput, extractFailingTests(cycle1546PhantomOutput))
	if len(got) != 1 || got[0] != "TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor" {
		t.Fatalf("the renamed-away binding must be named as a phantom; got %v", got)
	}
}

func TestPhantomBindings_FailingBoundTestIsNotAPhantom(t *testing.T) {
	out := `    predicates_test.go:73: binding test TestRealThing_Works did NOT pass in internal/core (missing, failing, or hidden behind a build tag). exit=1
--- FAIL: TestRealThing_Works (0.01s)
    real_test.go:12: assertion failed
--- FAIL: TestC1544_006_Whatever (0.5s)
`
	got := phantomBindings(out, extractFailingTests(out))
	if len(got) != 0 {
		t.Fatalf("a failing (existing) bound test must not classify as phantom; got %v", got)
	}
}

func TestPhantomBindings_PartialPhantomIsCaught(t *testing.T) {
	out := `    predicates_test.go:73: binding test TestGone_Renamed did NOT pass in internal/core (missing, failing, or hidden behind a build tag). exit=0
--- PASS: TestStillHere_Works (0.01s)
--- FAIL: TestC1544_007_Pair (0.5s)
`
	got := phantomBindings(out, extractFailingTests(out))
	if len(got) != 1 || got[0] != "TestGone_Renamed" {
		t.Fatalf("the never-ran half of a pair must classify as phantom; got %v", got)
	}
}

func TestPhantomBindings_OrdinaryRedIsUntouched(t *testing.T) {
	out := "--- FAIL: TestSomething (0.1s)\n    x_test.go:9: boom\n"
	if got := phantomBindings(out, extractFailingTests(out)); len(got) != 0 {
		t.Fatalf("no binding vocabulary means no phantoms; got %v", got)
	}
}

func TestPhantomBindings_DedupedAndOrdered(t *testing.T) {
	out := `binding test TestB_Two did NOT pass in internal/core (missing
binding test TestA_One did NOT pass in internal/core (missing
binding test TestB_Two did NOT pass in internal/core (missing
no tests to run
`
	got := phantomBindings(out, nil)
	if len(got) != 2 || got[0] != "TestB_Two" || got[1] != "TestA_One" {
		t.Fatalf("phantoms must dedupe preserving first-seen order; got %v", got)
	}
}

func TestParseGoTestJSON_RedCarriesPhantomBindings(t *testing.T) {
	ev := func(action, out string) string {
		e := map[string]string{"Action": action, "Package": "github.com/x/go/acs/cycle1544",
			"Test": "TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase"}
		if out != "" {
			e["Output"] = out
		}
		b, _ := json.Marshal(e)
		return string(b)
	}
	stream := strings.Join([]string{
		ev("run", ""),
		ev("output", "binding test TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor did NOT pass in internal/core (missing, failing, or hidden behind a build tag). exit=0\n"),
		ev("output", "testing: warning: no tests to run\n"),
		ev("fail", ""),
	}, "\n")

	rs := parseGoTestJSON(strings.NewReader(stream), 1546)

	if len(rs) != 1 || rs[0].ResultStr != "red" {
		t.Fatalf("want one red result; got %+v", rs)
	}
	if len(rs[0].PhantomBindings) != 1 ||
		rs[0].PhantomBindings[0] != "TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor" {
		t.Fatalf("the recorded red must carry the phantom classification; got %+v", rs[0].PhantomBindings)
	}
}

func TestParseGoTestJSON_GreenCarriesNoPhantoms(t *testing.T) {
	stream := `{"Action":"run","Package":"p","Test":"TestOK"}
{"Action":"pass","Package":"p","Test":"TestOK"}`
	rs := parseGoTestJSON(strings.NewReader(stream), 1)
	if len(rs) != 1 || rs[0].PhantomBindings != nil {
		t.Fatalf("green results carry no phantom classification; got %+v", rs)
	}
}

func TestParseGoTestJSON_RecordUsesFullOutputAndFailingSet(t *testing.T) {
	ev := func(test, action, out string) string {
		e := map[string]string{"Action": action, "Package": "p", "Test": test}
		if out != "" {
			e["Output"] = out
		}
		b, _ := json.Marshal(e)
		return string(b)
	}
	filler := strings.Repeat("x", 4000) + "\n"
	stream := strings.Join([]string{
		ev("TestPred", "run", ""),
		ev("TestPred", "output", filler),
		ev("TestPred", "output", "binding test TestFailsForReal did NOT pass in internal/core (missing, failing, or hidden behind a build tag). exit=1\n"),
		ev("TestPred", "output", "binding test TestGone_Renamed did NOT pass in internal/core (missing, failing, or hidden behind a build tag). exit=1\n"),
		ev("TestPred", "output", "--- FAIL: TestFailsForReal (0.01s)\n"),
		ev("TestPred", "output", filler),
		ev("TestPred", "fail", ""),
	}, "\n")

	rs := parseGoTestJSON(strings.NewReader(stream), 1)
	if len(rs) != 1 {
		t.Fatalf("want one result; got %d", len(rs))
	}
	r := rs[0]
	if !strings.Contains(r.EvidenceExcerpt, "…") {
		t.Fatalf("scenario invalid: output must exceed the excerpt cap so the middle is elided")
	}
	if len(r.PhantomBindings) != 1 || r.PhantomBindings[0] != "TestGone_Renamed" {
		t.Fatalf("full-output classification: only the never-ran binding is a phantom; got %v", r.PhantomBindings)
	}
}
