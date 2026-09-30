package subagentrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type IntegrityReason string

const (
	IntegrityMissing      IntegrityReason = "missing"
	IntegrityStale        IntegrityReason = "stale"
	IntegrityUnreadable   IntegrityReason = "unreadable"
	IntegrityEmpty        IntegrityReason = "empty"
	IntegrityTokenMissing IntegrityReason = "token_missing"
)

type VerifyInput struct {
	ExecErr  error
	ExitCode int

	StatErr error
	MTime   time.Time
	Now     time.Time
	MaxAge  time.Duration

	ReadErr error
	Body    []byte

	Token        string
	ArtifactPath string
}

type VerifyResult struct {
	Verdict     string
	Diagnostics []cyclestate.Diagnostic
	Reason      IntegrityReason
}

func Verify(in VerifyInput) VerifyResult {
	var diags []cyclestate.Diagnostic
	if in.ExecErr != nil {
		diags = append(diags, cyclestate.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("bridge launch failed (exit=%d): %v", in.ExitCode, in.ExecErr),
		})
	}
	if in.StatErr != nil {
		return integrityFail(diags, IntegrityMissing, fmt.Sprintf("artifact missing: %s", in.ArtifactPath))
	}
	if age := in.Now.Sub(in.MTime); age > in.MaxAge {
		return integrityFail(diags, IntegrityStale, fmt.Sprintf("artifact stale (%s old): %s", age.Round(time.Second), in.ArtifactPath))
	}
	if in.ReadErr != nil {
		return integrityFail(diags, IntegrityUnreadable, fmt.Sprintf("artifact unreadable: %v", in.ReadErr))
	}
	if len(in.Body) == 0 {
		return integrityFail(diags, IntegrityEmpty, fmt.Sprintf("artifact empty: %s", in.ArtifactPath))
	}
	if !bytes.Contains(in.Body, []byte(in.Token)) {
		return integrityFail(diags, IntegrityTokenMissing, fmt.Sprintf("challenge token %q missing from artifact", in.Token))
	}
	if in.ExecErr != nil || in.ExitCode != 0 {
		return VerifyResult{Verdict: VerdictFAIL, Diagnostics: diags}
	}
	return VerifyResult{Verdict: VerdictPASS, Diagnostics: diags}
}

func integrityFail(diags []cyclestate.Diagnostic, rung IntegrityReason, msg string) VerifyResult {
	return VerifyResult{
		Verdict:     VerdictIntegrityFail,
		Diagnostics: append(diags, cyclestate.Diagnostic{Severity: "error", Message: msg}),
		Reason:      rung,
	}
}

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

func StatMTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
