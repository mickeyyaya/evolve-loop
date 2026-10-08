package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyPackEvents_KeepsEachFailuresOwnOutput(t *testing.T) {
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
