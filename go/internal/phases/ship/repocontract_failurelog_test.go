package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyPackEvents_KeepsEachFailuresOwnOutput(t *testing.T) {
	t.Parallel()
	feed := strings.Join([]string{
		`{"Action":"output","Package":"p/ok","Test":"TestFine","Output":"=== RUN   TestFine\n"}`,
		`{"Action":"output","Package":"p/ok","Test":"TestFine","Output":"    fine_test.go:3: passing chatter\n"}`,
		`{"Action":"pass","Package":"p/ok","Test":"TestFine"}`,
		`{"Action":"output","Package":"p/ok","Output":"ok  \tp/ok\t0.1s\n"}`,
		`{"Action":"pass","Package":"p/ok"}`,
		`{"Action":"output","Package":"p/ratchet","Test":"TestRatchet","Output":"=== RUN   TestRatchet\n"}`,
		`{"Action":"output","Package":"p/ratchet","Test":"TestRatchet","Output":"    x/characterization_test.go builds a raw git repo: use gittest.Fixture(t)\n"}`,
		`{"Action":"output","Package":"p/ratchet","Test":"TestRatchet","Output":"--- FAIL: TestRatchet (0.01s)\n"}`,
		`{"Action":"fail","Package":"p/ratchet","Test":"TestRatchet"}`,
		`{"Action":"output","Package":"p/ratchet","Output":"FAIL\tp/ratchet\t0.1s\n"}`,
		`{"Action":"fail","Package":"p/ratchet"}`,
		`{"ImportPath":"p/broken [p/broken.test]","Action":"build-output","Output":"broken.go:3:23: cannot use \"s\" as int value\n"}`,
		`{"ImportPath":"p/broken [p/broken.test]","Action":"build-fail"}`,
		`{"Action":"output","Package":"p/broken","Output":"FAIL\tp/broken [build failed]\n"}`,
		`{"Action":"fail","Package":"p/broken","FailedBuild":"p/broken [p/broken.test]"}`,
		`{"ImportPath":"./internal/gone/...","Action":"build-output","Output":"pattern ./internal/gone/...: lstat ./internal/gone/: no such file or directory\n"}`,
		`{"ImportPath":"./internal/gone/...","Action":"build-fail"}`,
		`{"Action":"output","Package":"./internal/gone/...","Output":"FAIL\t./internal/gone/... [setup failed]\n"}`,
		`{"Action":"fail","Package":"./internal/gone/...","FailedBuild":"./internal/gone/..."}`,
	}, "\n")
	failed, log := classifyPackEvents(strings.NewReader(feed), io.Discard)
	if len(failed) != 3 {
		t.Fatalf("one named test and two build failures; got %v", failed)
	}
	for _, want := range []string{"characterization_test.go builds a raw git repo", "--- FAIL: TestRatchet", "cannot use \"s\" as int value", "FAIL\tp/broken [build failed]", "lstat ./internal/gone/: no such file or directory"} {
		if !strings.Contains(log, want) {
			t.Errorf("the failure log carries each failure's own text; %q missing from %q", want, log)
		}
	}
	for _, unwanted := range []string{"passing chatter", "ok  \tp/ok", "FAIL\tp/ratchet\t0.1s"} {
		if strings.Contains(log, unwanted) {
			t.Errorf("the failure log leaves out passing output and a named failure's package roll-up; %q in %q", unwanted, log)
		}
	}
}

func TestRunRepoContractPackages_CarriesTheFailingTestsOwnOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module failurelog\n\ngo 1.23\n")
	mustWrite(t, filepath.Join(dir, "failurelog_test.go"), `package failurelog

import "testing"

func TestChatty(t *testing.T) { t.Log("passing chatter") }

func TestRatchet(t *testing.T) { t.Error("x/characterization_test.go builds a raw git repo") }
`)
	o := runRepoContractPackages(context.Background(), dir, packLog{notes: io.Discard, raw: io.Discard}, []string{"./..."})
	if !o.realRed() || !strings.Contains(o.failureLog, "characterization_test.go builds a raw git repo") {
		t.Fatalf("a real run keeps the failing test's own message; red=%v log=%q", o.realRed(), o.failureLog)
	}
	if strings.Contains(o.failureLog, "passing chatter") {
		t.Fatalf("a passing test's output is not failure text; log=%q", o.failureLog)
	}
}

const nestedFixtureEvents = `{"Action":"run","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause"}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause","Output":"=== RUN   TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause\n","OutputType":"frame"}
{"Action":"run","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestInnerRunCycleErrorF4"}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestInnerRunCycleErrorF4","Output":"=== RUN   TestInnerRunCycleErrorF4\n","OutputType":"frame"}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestInnerRunCycleErrorF4","Output":"    engine.go:162: RunCycle: lease held\n","OutputType":"error"}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestInnerRunCycleErrorF4","Output":"--- FAIL: TestInnerRunCycleErrorF4 (0.08s)\n","OutputType":"frame"}
{"Action":"fail","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestInnerRunCycleErrorF4","Elapsed":0.08}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause","Output":"--- PASS: TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause (0.08s)\n","OutputType":"frame"}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Test":"TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause","Elapsed":0.08}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Output":"PASS\n","OutputType":"frame"}
{"Action":"output","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Output":"ok  \tgithub.com/mickeyyaya/evolve-loop/go/internal/routingtest\t0.355s\n"}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/routingtest","Elapsed":0.355}`

const realRedEvents = `{"Action":"output","Package":"p/ratchet","Test":"TestRatchet","Output":"=== RUN   TestRatchet\n"}
{"Action":"output","Package":"p/ratchet","Test":"TestRatchet","Output":"    ratchet_test.go:9: a real red\n"}
{"Action":"output","Package":"p/ratchet","Test":"TestRatchet","Output":"--- FAIL: TestRatchet (0.01s)\n"}
{"Action":"fail","Package":"p/ratchet","Test":"TestRatchet"}
{"Action":"output","Package":"p/ratchet","Output":"FAIL\tp/ratchet\t0.1s\n"}
{"Action":"fail","Package":"p/ratchet"}`

func TestClassifyPackEvents_ANestedFixtureFailureInAPassingPackageIsNotARed(t *testing.T) {
	t.Parallel()

	failed, log := classifyPackEvents(strings.NewReader(nestedFixtureEvents), io.Discard)

	if len(failed) != 0 || log != "" {
		t.Fatalf("the package ends pass, so its nested fixture is no red; failed=%v log=%q", failed, log)
	}
}

func TestClassifyPackEvents_ANestedFixtureNextToARealRedNamesOnlyTheRealRed(t *testing.T) {
	t.Parallel()

	failed, log := classifyPackEvents(strings.NewReader(nestedFixtureEvents+"\n"+realRedEvents), io.Discard)

	if len(failed) != 1 || failed[0].String() != "p/ratchet.TestRatchet" {
		t.Fatalf("only the failing package's test is a red; got %v", failed)
	}
	if !strings.Contains(log, "a real red") || strings.Contains(log, "lease held") {
		t.Fatalf("the failure log carries the real red and not the fixture text; log=%q", log)
	}
}

func TestClassifyPackEvents_ANamedFailureWithNoPackageResultStaysARed(t *testing.T) {
	t.Parallel()
	cut := strings.Join(strings.Split(realRedEvents, "\n")[:4], "\n")

	failed, log := classifyPackEvents(strings.NewReader(cut), io.Discard)

	if len(failed) != 1 || failed[0].String() != "p/ratchet.TestRatchet" || !strings.Contains(log, "a real red") {
		t.Fatalf("a stream that ends before the package result keeps the named red; failed=%v log=%q", failed, log)
	}
}

func TestClassifyPackEvents_TwoNamedFailuresInOnePackageKeepBothOutputsInStreamOrder(t *testing.T) {
	t.Parallel()
	feed := strings.Join([]string{
		`{"Action":"output","Package":"p/ratchet","Test":"TestFirst","Output":"first failure line\n"}`,
		`{"Action":"fail","Package":"p/ratchet","Test":"TestFirst"}`,
		`{"Action":"output","Package":"p/ratchet","Test":"TestFirst","Output":"second run of first\n"}`,
		`{"Action":"fail","Package":"p/ratchet","Test":"TestFirst"}`,
		`{"Action":"output","Package":"p/ratchet","Test":"TestSecond","Output":"second failure line\n"}`,
		`{"Action":"fail","Package":"p/ratchet","Test":"TestSecond"}`,
		`{"Action":"fail","Package":"p/ratchet"}`,
		`{"ImportPath":"p/broken [p/broken.test]","Action":"build-output","Output":"compile error line\n"}`,
		`{"ImportPath":"p/broken [p/broken.test]","Action":"build-fail"}`,
		`{"Action":"fail","Package":"p/broken","FailedBuild":"p/broken [p/broken.test]"}`,
	}, "\n")

	_, log := classifyPackEvents(strings.NewReader(feed), io.Discard)

	want := "first failure line\nsecond run of first\nsecond failure line\ncompile error line\n"
	if log != want {
		t.Fatalf("got %q, want %q", log, want)
	}
}
