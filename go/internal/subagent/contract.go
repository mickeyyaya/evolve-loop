package subagent

import (
	"bytes"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// contract.go is the single source of truth for "is this dispatched-agent
// artifact valid?". Before B3 the same verdict ladder lived in three Go
// copies — classifyArtifact (run.go, single dispatch), (*Runner).classify
// (subagent.go), and the bash verify_artifact() it was ported from — that
// could drift apart. Verify holds the one ladder; every dispatch path reaches
// it through VerifyArtifact. (The lighter `evolve subagent check-token` CLI
// probe in checktoken.go is intentionally a separate exists+token contract,
// not a dispatch verdict, and is left as-is.)

// VerifyInput is the gathered evidence a dispatched agent's artifact is judged
// against. No field touches the filesystem or the wall clock — VerifyArtifact
// gathers the stat/body once and the clock is passed in — so Verify is a pure
// function and its verdict ladder is table-testable with literals.
type VerifyInput struct {
	// ExecErr is the adapter/bridge launch-or-exec error (nil on a clean
	// launch). A non-nil ExecErr emits a leading bridge-error diagnostic but
	// does NOT short-circuit the integrity ladder: a missing/stale/empty/
	// tokenless artifact is still INTEGRITY_FAIL even when the launch failed.
	ExecErr  error
	ExitCode int

	// StatErr is non-nil when the artifact is missing/unstattable. MTime is
	// the artifact modtime, consulted only when StatErr == nil.
	StatErr error
	MTime   time.Time
	// Now is the comparison clock and MaxAge the freshness window: the
	// artifact must be newer than Now-MaxAge to be considered fresh.
	Now    time.Time
	MaxAge time.Duration

	// ReadErr is non-nil when the artifact is unreadable; Body holds the
	// artifact bytes when ReadErr == nil.
	ReadErr error
	Body    []byte

	// Token is the expected challenge token; the artifact body must contain it.
	Token string
	// ArtifactPath is used only to render diagnostics, never for I/O here.
	ArtifactPath string
}

// VerifyResult is the verdict plus the integrity diagnostics that explain it.
// Callers that only need the verdict read Verdict; callers that surface the
// reasons (the subagent Result) read Diagnostics.
type VerifyResult struct {
	Verdict     string
	Diagnostics []core.Diagnostic
}

// Verify applies the one ordered verdict ladder — integrity first, exec last:
//
//	stat → freshness → readable/non-empty → token → exec-status
//
// The four integrity checks take precedence over exec status: a clean exit
// over a stale/empty/tokenless artifact is still INTEGRITY_FAIL, and a valid
// artifact with a non-zero exit (or a non-nil ExecErr) is FAIL. Diagnostic
// wording matches the bash verify_artifact() this ladder was ported from.
func Verify(in VerifyInput) VerifyResult {
	var diags []core.Diagnostic
	if in.ExecErr != nil {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("bridge launch failed (exit=%d): %v", in.ExitCode, in.ExecErr),
		})
	}
	if in.StatErr != nil {
		return integrityFail(diags, fmt.Sprintf("artifact missing: %s", in.ArtifactPath))
	}
	if age := in.Now.Sub(in.MTime); age > in.MaxAge {
		return integrityFail(diags, fmt.Sprintf("artifact stale (%s old): %s", age.Round(time.Second), in.ArtifactPath))
	}
	if in.ReadErr != nil {
		return integrityFail(diags, fmt.Sprintf("artifact unreadable: %v", in.ReadErr))
	}
	if len(in.Body) == 0 {
		return integrityFail(diags, fmt.Sprintf("artifact empty: %s", in.ArtifactPath))
	}
	if !bytes.Contains(in.Body, []byte(in.Token)) {
		return integrityFail(diags, fmt.Sprintf("challenge token %q missing from artifact", in.Token))
	}
	if in.ExecErr != nil || in.ExitCode != 0 {
		return VerifyResult{Verdict: VerdictFAIL, Diagnostics: diags}
	}
	return VerifyResult{Verdict: VerdictPASS, Diagnostics: diags}
}

// integrityFail appends the integrity diagnostic (after any leading
// bridge-error diagnostic) and returns the INTEGRITY_FAIL verdict.
func integrityFail(diags []core.Diagnostic, msg string) VerifyResult {
	return VerifyResult{
		Verdict:     VerdictIntegrityFail,
		Diagnostics: append(diags, core.Diagnostic{Severity: "error", Message: msg}),
	}
}

// VerifyArtifact is the one I/O-bearing entry point every dispatch path shares.
// It gathers the artifact's stat (and, on success, its body) through the
// injected filesystem seams, then judges it with Verify against ArtifactMaxAge.
// The body is read only when stat succeeds, preserving the "missing artifact is
// judged before any read" short-circuit of the ladder it replaces.
func VerifyArtifact(
	stat func(path string) (time.Time, error),
	read func(path string) ([]byte, error),
	now func() time.Time,
	artifactPath, token string,
	exitCode int,
	execErr error,
) VerifyResult {
	in := VerifyInput{
		ExecErr:      execErr,
		ExitCode:     exitCode,
		Now:          now(),
		MaxAge:       ArtifactMaxAge,
		Token:        token,
		ArtifactPath: artifactPath,
	}
	mtime, statErr := stat(artifactPath)
	in.StatErr = statErr
	in.MTime = mtime
	if statErr == nil {
		body, readErr := read(artifactPath)
		in.ReadErr = readErr
		in.Body = body
	}
	return Verify(in)
}
