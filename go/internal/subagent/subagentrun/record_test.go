package subagentrun

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 40 — every integrity rung is ONE ARTIFACT_INTEGRITY_FAIL naming it,
// with the ladder's diagnostic as the reason and the evidence carried on the
// Outcome; a PASS emits nothing.
func TestRecord_IntegrityRungSignals(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		rung    IntegrityReason
		adapter func(path, token string)
		read    func(string) ([]byte, error)
		reason  string
	}{
		{IntegrityMissing, func(string, string) {}, os.ReadFile, "artifact missing: "},
		{IntegrityStale, func(p, tok string) { writeArtifact(t, p, tok, fixedNow.Add(-6*time.Minute)) }, os.ReadFile, "artifact stale (6m0s old): "},
		{IntegrityUnreadable, func(p, tok string) { writeArtifact(t, p, tok, fixedNow) }, func(string) ([]byte, error) { return nil, errors.New("sealed") }, "artifact unreadable: sealed"},
		{IntegrityEmpty, func(p, _ string) {
			writeArtifact(t, p, "", fixedNow)
			_ = os.WriteFile(p, nil, 0o644)
			_ = os.Chtimes(p, fixedNow, fixedNow)
		}, os.ReadFile, "artifact empty: "},
		{IntegrityTokenMissing, func(p, _ string) { writeArtifact(t, p, "other", fixedNow) }, os.ReadFile, `challenge token "` + aaToken + `" missing from artifact`},
	}
	for _, c := range cases {
		deps := happyDeps(t)
		deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
			c.adapter(e.ArtifactPath, e.ChallengeToken)
			return 0, nil
		})
		d, r := observed(t, deps, WithFS(StatMTime, c.read, HashFile))
		req := f.request()
		req.ProjectRoot = t.TempDir()
		out, err := d.Dispatch(context.Background(), req)
		if err != nil || out.Verdict != VerdictIntegrityFail || out.Integrity != c.rung || len(out.Diagnostics) != 1 || !strings.HasPrefix(out.Diagnostics[0].Message, c.reason) {
			t.Fatalf("%s: %v %+v", c.rung, err, out)
		}
		e := r.only(t, CodeArtifactIntegrityFail)
		if e.Fields["rung"] != string(c.rung) || e.Fields["step"] != "verify" || e.Fields["exit_code"] != "0" || e.Fields["artifact"] != out.ArtifactPath || e.Reason != out.Diagnostics[0].Message {
			t.Fatalf("%s: %+v", c.rung, e)
		}
	}
	d, r := observed(t, happyDeps(t))
	if out, err := d.Dispatch(context.Background(), f.request()); err != nil || out.Verdict != VerdictPASS || out.Integrity != "" || len(out.Diagnostics) != 0 || len(r.events) != 0 {
		t.Fatalf("PASS: %v %+v %v", err, out, r.codes())
	}
}

// Test 41 — a sound artifact with a non-zero exit is FAIL and ONE
// VERDICT_FAIL carrying the exit code; no integrity code.
func TestRecord_VerdictFailCarriesTheExitCode(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
		writeArtifact(t, e.ArtifactPath, e.ChallengeToken, fixedNow)
		return 81, nil
	})
	d, r := observed(t, deps)
	out, err := d.Dispatch(context.Background(), f.request())
	if err != nil || out.Verdict != VerdictFAIL || out.ExitCode != 81 || out.Integrity != "" {
		t.Fatalf("%v %+v", err, out)
	}
	e := r.only(t, CodeVerdictFail)
	if e.Fields["exit_code"] != "81" || e.Fields["step"] != "verify" || e.Fields["cli"] != "claude" || e.Fields["artifact"] != out.ArtifactPath || e.Reason != "adapter exited 81 over a sound artifact" {
		t.Fatalf("%+v", e)
	}
}

// Test 42 — an adapter error: the ledger line exists (exit_code -1) before
// the error is returned, the outcome is populated, exactly ONE
// ADAPTER_EXEC_FAILED carries verdict + integrity, no VERDICT_FAIL /
// INTEGRITY_FAIL doubles it, and the signal precedes the ledger append.
func TestRecord_ExecErrorWritesTheLedgerThenReturnsWithOneOutcomeSignal(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.Adapter = AdapterFunc(func(context.Context, AdapterEnv) (int, error) { return -1, errors.New("adapter crashed") })
	var seenAtOpen []string
	c, r := recording()
	open := func(p string) (io.WriteCloser, error) {
		for _, e := range r.events {
			seenAtOpen = append(seenAtOpen, string(e.Code))
		}
		return openAppend(p)
	}
	d := New(deps, WithClock(clock), WithRand(aaRand), WithSignals(func() *signalcenter.Center { return c }), WithLedgerOpener(open))
	req := f.request()
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	out, err := d.Dispatch(context.Background(), req)
	if err == nil || err.Error() != "subagent/run: adapter exec: adapter crashed" || errors.Unwrap(err) == nil || errors.Unwrap(err).Error() != "adapter crashed" {
		t.Fatalf("%v", err)
	}
	if out.ExitCode != -1 || out.Verdict != VerdictIntegrityFail || out.Integrity != IntegrityMissing || len(out.Diagnostics) != 2 {
		t.Fatalf("the outcome is populated: %+v", out)
	}
	line, readErr := os.ReadFile(req.LedgerPath)
	if readErr != nil || !strings.Contains(string(line), `"exit_code":-1,`) {
		t.Fatalf("the ledger is written before the return: %v %s", readErr, line)
	}
	e := r.only(t, CodeAdapterExecFailed)
	if e.Fields["step"] != "exec" || e.Fields["exit_code"] != "-1" || e.Fields["verdict"] != VerdictIntegrityFail || e.Fields["integrity"] != "missing" || e.Reason != "adapter exec failed (exit=-1): adapter crashed" {
		t.Fatalf("%+v", e)
	}
	if len(seenAtOpen) != 1 || seenAtOpen[0] != string(CodeAdapterExecFailed) {
		t.Fatalf("the outcome signal precedes the ledger append: %v", seenAtOpen)
	}
}

// Test 43 — the hash signal fires only when the artifact stood the ladder.
func TestRecord_HashFailedOnlyWhenTheArtifactStood(t *testing.T) {
	f := newFixture(t)
	hashErr := func(string) (string, error) { return "", errors.New("hash boom") }
	d, r := observed(t, happyDeps(t), WithFS(StatMTime, os.ReadFile, hashErr))
	req := f.request()
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	out, err := d.Dispatch(context.Background(), req)
	if err != nil || out.Verdict != VerdictPASS || out.ArtifactSHA256 != "" {
		t.Fatalf("%v %+v", err, out)
	}
	if e := r.only(t, CodeArtifactHashFailed); e.Fields["step"] != "hash" || e.Fields["artifact"] != out.ArtifactPath || e.Reason != `artifact hash failed; ledger stamps artifact_sha256="": hash boom` {
		t.Fatalf("%+v", e)
	}
	line, _ := os.ReadFile(req.LedgerPath)
	if !strings.Contains(string(line), `"artifact_sha256":""`) {
		t.Fatalf("%s", line)
	}
	deps := happyDeps(t)
	deps.Adapter = AdapterFunc(func(context.Context, AdapterEnv) (int, error) { return 0, nil })
	d, r = observed(t, deps, WithFS(StatMTime, os.ReadFile, hashErr))
	req.ProjectRoot, req.LedgerPath = t.TempDir(), ""
	if out, err := d.Dispatch(context.Background(), req); err != nil || out.Verdict != VerdictIntegrityFail {
		t.Fatalf("%v %+v", err, out)
	}
	if r.only(t, CodeArtifactIntegrityFail).Code != CodeArtifactIntegrityFail {
		t.Fatal("on a missing artifact only the integrity code")
	}
}

// Test 44 — a ledger write error masks the exec error with the bare text,
// names the op in the fields, and follows the outcome signal; no ledger path
// ⇒ no ledger, no code.
func TestRecord_LedgerWriteErrorMasksExecErrorAndNamesTheOp(t *testing.T) {
	f := newFixture(t)
	blocker := filepath.Join(f.root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := happyDeps(t)
	deps.Adapter = AdapterFunc(func(_ context.Context, e AdapterEnv) (int, error) {
		writeArtifact(t, e.ArtifactPath, e.ChallengeToken, fixedNow)
		return -1, errors.New("adapter crashed")
	})
	d, r := observed(t, deps)
	req := f.request()
	req.LedgerPath = filepath.Join(blocker, "sub", "ledger.jsonl")
	out, err := d.Dispatch(context.Background(), req)
	if err == nil || !strings.HasPrefix(err.Error(), "subagent/run: ledger write: ") || strings.Contains(err.Error(), "adapter exec") || strings.Contains(err.Error(), "mkdir:") {
		t.Fatalf("%v", err)
	}
	if out.Verdict != VerdictFAIL || out.ExitCode != -1 {
		t.Fatalf("the outcome is populated: %+v", out)
	}
	if codes := r.codes(); len(codes) != 2 || codes[0] != CodeAdapterExecFailed || codes[1] != CodeLedgerWriteFailed {
		t.Fatalf("the outcome signal then the ledger failure: %v", codes)
	}
	if e := r.events[1]; e.Fields["step"] != "ledger" || e.Fields["op"] != "mkdir" || e.Fields["path"] != req.LedgerPath || !strings.HasPrefix(e.Reason, "ledger write failed at mkdir: ") {
		t.Fatalf("%+v", e)
	}
	d, r = observed(t, deps)
	req.LedgerPath = ""
	if _, err := d.Dispatch(context.Background(), req); err == nil || !strings.HasPrefix(err.Error(), "subagent/run: adapter exec: ") || len(r.events) != 1 {
		t.Fatalf("no ledger path ⇒ the exec error, one signal: %v %v", err, r.codes())
	}
}
