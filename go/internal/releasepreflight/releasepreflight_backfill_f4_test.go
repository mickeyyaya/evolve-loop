package releasepreflight

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestRun_AFailingCIProbeIsACheckFailureNamingTheCause(t *testing.T) {
	opts := stubOpts(makeRepo(t, "1.0.0"), "1.0.1")
	var buf bytes.Buffer
	opts.Stderr = &buf
	opts.CIConclusion = func(string) (CIRunStatus, error) { return CIRunStatus{}, errors.New("gh crashed") }

	res, err := Run(opts)

	if !errors.Is(err, ErrCheckFailed) || !strings.Contains(err.Error(), "release-commit CI check error: gh crashed") {
		t.Fatalf("err = %v\nlog=%s", err, buf.String())
	}
	if res.CIConclusion != "" || res.CIOverridden {
		t.Errorf("a probe error must not record a conclusion: %+v", res)
	}
}

func goShimF4(t *testing.T, script string) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(t.TempDir(), "fake-go")
	fakeclitest.Install(t, shim, script)
	old := defaultGoBinFn
	t.Cleanup(func() { defaultGoBinFn = old })
	defaultGoBinFn = func() string { return shim }
	return repo
}

func TestDefaultGateTestRunner_PassesOnAGreenSuiteAndPassesTheCountFlagAndSuite(t *testing.T) {
	repo := goShimF4(t, "#!/bin/sh\necho \"ARGS: $*\" > \"$PWD/args.txt\"\nexit 0\n")

	if err := defaultGateTestRunner(repo, "./internal/guards/..."); err != nil {
		t.Fatalf("a green go test must pass the gate: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(repo, "go", "args.txt"))
	if err != nil || strings.TrimSpace(string(got)) != "ARGS: test -count=1 ./internal/guards/..." {
		t.Fatalf("the gate must run `go test -count=1 <suite>` in <repo>/go: %q %v", got, err)
	}
}

func TestDefaultGateTestRunner_ARedSuiteCarriesItsOutput(t *testing.T) {
	repo := goShimF4(t, "#!/bin/sh\necho SUITE_RED_OUTPUT\nexit 1\n")

	err := defaultGateTestRunner(repo, "./x")

	if err == nil || !strings.Contains(err.Error(), "go test ./x") || !strings.Contains(err.Error(), "SUITE_RED_OUTPUT") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckRecentAudit_AnUnreadableLedgerIsAnErrorNotAMissingAudit(t *testing.T) {
	dir := t.TempDir()

	res, err := checkRecentAudit(dir, "headsha", false, time.Now())

	if err == nil || errors.Is(err, os.ErrNotExist) || res.verdict == auditVerdictNone {
		t.Fatalf("res=%+v err=%v: a ledger that exists but cannot be read must not look like no audit", res, err)
	}
	res, err = checkRecentAudit(filepath.Join(dir, "absent.jsonl"), "headsha", false, time.Now())
	if err != nil || res.verdict != auditVerdictNone {
		t.Fatalf("an absent ledger is the NONE verdict: res=%+v err=%v", res, err)
	}
}
