package subagentrun

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// verify_test.go — the host's contract_test.go moved (ADR-0103 unit 16 D5):
// the same cases over cyclestate.Diagnostic (the type core.Diagnostic
// aliases), with the typed rung asserted per case; the read-error case
// provokes the read seam instead of a chmod (no root skip).

// verifyNow is the comparison clock all pure Verify cases judge against.
var verifyNow = time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

// hasDiag reports whether any diagnostic message contains want.
func hasDiag(diags []cyclestate.Diagnostic, want string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, want) {
			return true
		}
	}
	return false
}

// Test 39 — the contract table for the pure verdict ladder: the four
// integrity branches, the exec-status branch, the happy PASS, the precedence
// rule (integrity beats exec status), and the typed rung.
func TestVerify_LadderAndTypedRung(t *testing.T) {
	t.Parallel()
	fresh := verifyNow.Add(-1 * time.Minute) // 1 min old → fresh

	cases := []struct {
		name        string
		in          VerifyInput
		wantVerdict string
		wantDiag    string // substring expected in diagnostics ("" = no check)
		wantReason  IntegrityReason
	}{
		{
			name:        "stat error is integrity fail",
			in:          VerifyInput{StatErr: errors.New("missing"), Now: verifyNow, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/nope"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact missing: /nope",
			wantReason:  IntegrityMissing,
		},
		{
			name:        "stale artifact is integrity fail",
			in:          VerifyInput{MTime: verifyNow.Add(-10 * time.Minute), Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact stale (10m0s old): /a",
			wantReason:  IntegrityStale,
		},
		{
			name:        "exactly max age is still fresh",
			in:          VerifyInput{MTime: verifyNow.Add(-ArtifactMaxAge), Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok"},
			wantVerdict: VerdictPASS,
		},
		{
			name:        "read error is integrity fail",
			in:          VerifyInput{MTime: fresh, Now: verifyNow, MaxAge: ArtifactMaxAge, ReadErr: errors.New("permission denied"), Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact unreadable: permission denied",
			wantReason:  IntegrityUnreadable,
		},
		{
			name:        "empty body is integrity fail",
			in:          VerifyInput{MTime: fresh, Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte{}, Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact empty: /a",
			wantReason:  IntegrityEmpty,
		},
		{
			name:        "missing token is integrity fail",
			in:          VerifyInput{MTime: fresh, Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte("plain body, no match"), Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    `challenge token "tok" missing from artifact`,
			wantReason:  IntegrityTokenMissing,
		},
		{
			name:        "valid artifact nonzero exit is FAIL",
			in:          VerifyInput{MTime: fresh, Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok", ExitCode: 1},
			wantVerdict: VerdictFAIL,
		},
		{
			name:        "valid artifact exec error is FAIL with bridge diagnostic",
			in:          VerifyInput{MTime: fresh, Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok", ExecErr: errors.New("boom"), ExitCode: 2},
			wantVerdict: VerdictFAIL,
			wantDiag:    "bridge launch failed (exit=2)",
		},
		{
			name:        "integrity beats exec status: missing artifact with clean exit is INTEGRITY_FAIL",
			in:          VerifyInput{StatErr: errors.New("missing"), Now: verifyNow, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/a", ExitCode: 0},
			wantVerdict: VerdictIntegrityFail,
			wantReason:  IntegrityMissing,
		},
		{
			name:        "exec error does not short-circuit integrity: missing artifact still INTEGRITY_FAIL with leading bridge diag",
			in:          VerifyInput{StatErr: errors.New("missing"), Now: verifyNow, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/a", ExecErr: errors.New("boom"), ExitCode: 9},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "bridge launch failed (exit=9)",
			wantReason:  IntegrityMissing,
		},
		{
			name:        "happy path is PASS with no diagnostics",
			in:          VerifyInput{MTime: fresh, Now: verifyNow, MaxAge: ArtifactMaxAge, Body: []byte("header tok body"), Token: "tok", ExitCode: 0},
			wantVerdict: VerdictPASS,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var got VerifyResult
			got = Verify(c.in)
			if got.Verdict != c.wantVerdict {
				t.Errorf("verdict = %s, want %s", got.Verdict, c.wantVerdict)
			}
			if c.wantDiag != "" && !hasDiag(got.Diagnostics, c.wantDiag) {
				t.Errorf("diagnostics %v missing %q", got.Diagnostics, c.wantDiag)
			}
			if c.wantVerdict == VerdictPASS && len(got.Diagnostics) != 0 {
				t.Errorf("PASS should carry no diagnostics, got %v", got.Diagnostics)
			}
			if got.Reason != c.wantReason {
				t.Errorf("rung = %q, want %q", got.Reason, c.wantReason)
			}
		})
	}
}

// TestVerify_DiagnosticOrdering pins that a non-nil ExecErr emits its
// bridge-launch diagnostic FIRST (before any integrity diagnostic), matching
// the legacy Runner.classify ordering it replaces.
func TestVerify_DiagnosticOrdering(t *testing.T) {
	t.Parallel()
	got := Verify(VerifyInput{
		StatErr:      errors.New("missing"),
		Now:          verifyNow,
		MaxAge:       ArtifactMaxAge,
		Token:        "tok",
		ArtifactPath: "/a",
		ExecErr:      errors.New("boom"),
		ExitCode:     7,
	})
	if got.Verdict != VerdictIntegrityFail {
		t.Fatalf("verdict = %s, want INTEGRITY_FAIL", got.Verdict)
	}
	if len(got.Diagnostics) != 2 {
		t.Fatalf("want 2 diagnostics (bridge + missing), got %d: %v", len(got.Diagnostics), got.Diagnostics)
	}
	if !strings.Contains(got.Diagnostics[0].Message, "bridge launch failed") {
		t.Errorf("first diagnostic should be the bridge error, got %q", got.Diagnostics[0].Message)
	}
	if !strings.Contains(got.Diagnostics[1].Message, "artifact missing") {
		t.Errorf("second diagnostic should be the integrity failure, got %q", got.Diagnostics[1].Message)
	}
}

// --- VerifyArtifact: the one I/O adapter every dispatch path shares. ---

func TestVerifyArtifact_StatErrIsIntegrityFail(t *testing.T) {
	t.Parallel()
	stat := func(string) (time.Time, error) { return time.Time{}, errors.New("missing") }
	got := VerifyArtifact(stat, os.ReadFile, time.Now, "/nope", "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail || got.Reason != IntegrityMissing {
		t.Errorf("got %s/%s, want INTEGRITY_FAIL/missing", got.Verdict, got.Reason)
	}
}

func TestVerifyArtifact_StaleArtifactIsIntegrityFail(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "stale.md")
	_ = os.WriteFile(path, []byte("body with tok"), 0o644)
	old := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(path, old, old)
	got := VerifyArtifact(StatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail || got.Reason != IntegrityStale {
		t.Errorf("got %s/%s, want INTEGRITY_FAIL/stale", got.Verdict, got.Reason)
	}
}

func TestVerifyArtifact_EmptyBodyIsIntegrityFail(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.md")
	_ = os.WriteFile(path, []byte{}, 0o644)
	got := VerifyArtifact(StatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail || got.Reason != IntegrityEmpty {
		t.Errorf("got %s/%s, want INTEGRITY_FAIL/empty", got.Verdict, got.Reason)
	}
}

// A failing read seam (the host's chmod-0 fixture, without the root skip).
func TestVerifyArtifact_ReadErrIntegrityFail(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sealed.md")
	_ = os.WriteFile(path, []byte("body with tok"), 0o600)
	read := func(string) ([]byte, error) { return nil, errors.New("permission denied") }
	got := VerifyArtifact(StatMTime, read, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail || got.Reason != IntegrityUnreadable {
		t.Errorf("got %s/%s, want INTEGRITY_FAIL/unreadable", got.Verdict, got.Reason)
	}
}

// TestVerifyArtifact_HappyPathIsPass proves the adapter gathers a real fresh
// token-bearing artifact and judges it PASS.
func TestVerifyArtifact_HappyPathIsPass(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ok.md")
	_ = os.WriteFile(path, []byte("first line tok\nbody\n"), 0o644)
	got := VerifyArtifact(StatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictPASS || got.Reason != "" {
		t.Errorf("got %s/%q, want PASS", got.Verdict, got.Reason)
	}
}

// Test 48 (part) — the production stat and hash over real files.
func TestStatMTimeAndHashFile_RealFiles(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.md")
	_ = os.WriteFile(path, []byte("abc"), 0o644)
	at := time.Date(2026, 5, 23, 17, 0, 0, 0, time.UTC)
	_ = os.Chtimes(path, at, at)
	if mt, err := StatMTime(path); err != nil || !mt.Equal(at) {
		t.Fatalf("%v %v", mt, err)
	}
	if _, err := StatMTime(filepath.Join(tmp, "nope")); err == nil {
		t.Fatal("a missing file errs")
	}
	if sha, err := HashFile(path); err != nil || sha != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("%q %v", sha, err)
	}
	if _, err := HashFile(filepath.Join(tmp, "nope")); err == nil {
		t.Fatal("a missing file errs")
	}
	if _, err := HashFile(tmp); err == nil {
		t.Fatal("a directory cannot be read")
	}
}
