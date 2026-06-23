package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// fixedNow is the comparison clock all pure Verify cases judge against.
var fixedNow = time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

// hasDiag reports whether any diagnostic message contains want.
func hasDiag(diags []core.Diagnostic, want string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, want) {
			return true
		}
	}
	return false
}

// TestVerify is the contract table for the pure verdict ladder. It needs no
// filesystem and no real clock, so every case is independent and parallel.
// It covers the four integrity branches, the exec-status branch, the happy
// PASS, and the precedence rule (integrity beats exec status).
func TestVerify(t *testing.T) {
	t.Parallel()
	fresh := fixedNow.Add(-1 * time.Minute) // 1 min old → fresh

	cases := []struct {
		name        string
		in          VerifyInput
		wantVerdict string
		wantDiag    string // substring expected in diagnostics ("" = no check)
	}{
		{
			name:        "stat error is integrity fail",
			in:          VerifyInput{StatErr: errors.New("missing"), Now: fixedNow, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/nope"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact missing: /nope",
		},
		{
			name:        "stale artifact is integrity fail",
			in:          VerifyInput{MTime: fixedNow.Add(-10 * time.Minute), Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact stale",
		},
		{
			name:        "exactly max age is still fresh",
			in:          VerifyInput{MTime: fixedNow.Add(-ArtifactMaxAge), Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok"},
			wantVerdict: VerdictPASS,
		},
		{
			name:        "read error is integrity fail",
			in:          VerifyInput{MTime: fresh, Now: fixedNow, MaxAge: ArtifactMaxAge, ReadErr: errors.New("permission denied"), Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact unreadable",
		},
		{
			name:        "empty body is integrity fail",
			in:          VerifyInput{MTime: fresh, Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte{}, Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "artifact empty: /a",
		},
		{
			name:        "missing token is integrity fail",
			in:          VerifyInput{MTime: fresh, Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte("plain body, no match"), Token: "tok", ArtifactPath: "/a"},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    `challenge token "tok" missing`,
		},
		{
			name:        "valid artifact nonzero exit is FAIL",
			in:          VerifyInput{MTime: fresh, Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok", ExitCode: 1},
			wantVerdict: VerdictFAIL,
		},
		{
			name:        "valid artifact exec error is FAIL with bridge diagnostic",
			in:          VerifyInput{MTime: fresh, Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte("body tok"), Token: "tok", ExecErr: errors.New("boom"), ExitCode: 2},
			wantVerdict: VerdictFAIL,
			wantDiag:    "bridge launch failed (exit=2)",
		},
		{
			name:        "integrity beats exec status: missing artifact with clean exit is INTEGRITY_FAIL",
			in:          VerifyInput{StatErr: errors.New("missing"), Now: fixedNow, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/a", ExitCode: 0},
			wantVerdict: VerdictIntegrityFail,
		},
		{
			name:        "exec error does not short-circuit integrity: missing artifact still INTEGRITY_FAIL with leading bridge diag",
			in:          VerifyInput{StatErr: errors.New("missing"), Now: fixedNow, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/a", ExecErr: errors.New("boom"), ExitCode: 9},
			wantVerdict: VerdictIntegrityFail,
			wantDiag:    "bridge launch failed (exit=9)",
		},
		{
			name:        "happy path is PASS with no diagnostics",
			in:          VerifyInput{MTime: fresh, Now: fixedNow, MaxAge: ArtifactMaxAge, Body: []byte("header tok body"), Token: "tok", ExitCode: 0},
			wantVerdict: VerdictPASS,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Verify(c.in)
			if got.Verdict != c.wantVerdict {
				t.Errorf("verdict = %s, want %s", got.Verdict, c.wantVerdict)
			}
			if c.wantDiag != "" && !hasDiag(got.Diagnostics, c.wantDiag) {
				t.Errorf("diagnostics %v missing %q", got.Diagnostics, c.wantDiag)
			}
			if c.wantVerdict == VerdictPASS && len(got.Diagnostics) != 0 {
				t.Errorf("PASS should carry no diagnostics, got %v", got.Diagnostics)
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
		Now:          fixedNow,
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
// These cases retarget the four legacy classifyArtifact FS tests onto the
// SSOT entry point (they used to assert classifyArtifact directly).

func TestVerifyArtifact_StatErrIsIntegrityFail(t *testing.T) {
	t.Parallel()
	stat := func(string) (time.Time, error) { return time.Time{}, errors.New("missing") }
	got := VerifyArtifact(stat, os.ReadFile, time.Now, "/nope", "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail {
		t.Errorf("got %s, want INTEGRITY_FAIL", got.Verdict)
	}
}

func TestVerifyArtifact_StaleArtifactIsIntegrityFail(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "stale.md")
	_ = os.WriteFile(path, []byte("body with tok"), 0o644)
	old := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(path, old, old)
	got := VerifyArtifact(defaultStatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail {
		t.Errorf("got %s, want INTEGRITY_FAIL", got.Verdict)
	}
}

func TestVerifyArtifact_EmptyBodyIsIntegrityFail(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.md")
	_ = os.WriteFile(path, []byte{}, 0o644)
	got := VerifyArtifact(defaultStatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail {
		t.Errorf("got %s, want INTEGRITY_FAIL", got.Verdict)
	}
}

func TestVerifyArtifact_ReadErrIntegrityFail(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root cannot mask read")
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sealed.md")
	_ = os.WriteFile(path, []byte("body with tok"), 0o600)
	_ = os.Chmod(path, 0o000)
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	got := VerifyArtifact(defaultStatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictIntegrityFail {
		t.Errorf("got %s, want INTEGRITY_FAIL", got.Verdict)
	}
}

// TestVerifyArtifact_HappyPathIsPass proves the adapter gathers a real fresh
// token-bearing artifact and judges it PASS — the case the legacy FS tests
// never covered.
func TestVerifyArtifact_HappyPathIsPass(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ok.md")
	_ = os.WriteFile(path, []byte("first line tok\nbody\n"), 0o644)
	got := VerifyArtifact(defaultStatMTime, os.ReadFile, time.Now, path, "tok", 0, nil)
	if got.Verdict != VerdictPASS {
		t.Errorf("got %s, want PASS", got.Verdict)
	}
}
