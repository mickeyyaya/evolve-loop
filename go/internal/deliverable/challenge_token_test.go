package deliverable

// challenge_token_test.go — cycle-269 incident RED tests: the challenge-token
// protocol (scout mints <workspace>/challenge-token.txt + embeds it in
// scout-report.md; downstream reports must echo it as proof-of-read) was
// enforced ONLY at audit — unrecoverable: a perfect EGPS-green build FAILed
// the whole cycle over a missing echo. The bash→Go migration had also dropped
// the prompt-side injection (resolved-prompt.txt: zero mentions), so fallback
// builders never even saw the instruction. This moves the invariant to the
// machine-checkable boundary where the EXISTING correction loop (PR #60) can
// re-dispatch with the exact fix BEFORE audit: contracts opt in via
// Contract.RequireChallengeToken (the RequireFailureContext precedent).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
)

const tokenFixture = "67dffdcb2fb3ab46"

// writeTokenWorkspace lays out a workspace with challenge-token.txt and a
// build-report.md of the given content; returns the roots.
func writeTokenWorkspace(t *testing.T, report string, withTokenFile bool) phasecontract.Roots {
	t.Helper()
	ws := t.TempDir()
	if withTokenFile {
		if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte(tokenFixture+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	return phasecontract.Roots{Workspace: ws}
}

// wellFormedBuildReport satisfies the build contract's section/verdict rules
// so the ONLY variable under test is the token echo.
func wellFormedBuildReport(token string) string {
	tok := ""
	if token != "" {
		tok = "<!-- challenge-token: " + token + " -->\n"
	}
	return "# Build Report — Cycle 269\n" + tok + `
## Task
wire-it

**Status:** PASS

## Changes
- did the thing

## Build Steps
- S1

## Self-Verification
- tests green

## Verdict
PASS
`
}

func TestVerify_Build_MissingChallengeToken_Violation(t *testing.T) {
	t.Parallel()
	roots := writeTokenWorkspace(t, wellFormedBuildReport(""), true)
	res, err := Verify("build", roots)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.OK {
		t.Fatal("a token-required report missing the minted token must be a violation (cycle-269: this reached AUDIT and killed the cycle; the contract gate is the correctable boundary)")
	}
	var hit *Violation
	for i := range res.Violations {
		if res.Violations[i].Code == CodeMissingChallengeToken {
			hit = &res.Violations[i]
		}
	}
	if hit == nil {
		t.Fatalf("want a %s violation; got %+v", CodeMissingChallengeToken, res.Violations)
	}
	// The message feeds composeCorrection verbatim — it must carry the exact
	// token so the re-dispatched agent can fix it without guessing.
	if !strings.Contains(hit.Message, tokenFixture) {
		t.Errorf("violation message must carry the exact token for the correction directive; got %q", hit.Message)
	}
}

func TestVerify_Build_TokenEchoed_OK(t *testing.T) {
	t.Parallel()
	roots := writeTokenWorkspace(t, wellFormedBuildReport(tokenFixture), true)
	res, err := Verify("build", roots)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !res.OK {
		t.Fatalf("token echoed anywhere in the report must satisfy the check; violations=%+v", res.Violations)
	}
}

func TestVerify_Build_NoTokenFile_FailOpen(t *testing.T) {
	t.Parallel()
	// No minted token (token-less runs, unit harnesses, resets) ⇒ nothing to
	// echo ⇒ the check is silent. Ambiguity never blocks (house posture).
	roots := writeTokenWorkspace(t, wellFormedBuildReport(""), false)
	res, err := Verify("build", roots)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !res.OK {
		t.Fatalf("absent challenge-token.txt must fail open; violations=%+v", res.Violations)
	}
}

func TestContract_RequireChallengeToken_BuildOnlyAtV1(t *testing.T) {
	t.Parallel()
	b, ok := phasecontract.For("build")
	if !ok || !b.RequireChallengeToken {
		t.Fatal("build's contract must require the challenge-token echo (cycle-269)")
	}
	// scout MINTS the token — requiring it to echo itself would be circular.
	s, ok := phasecontract.For("scout")
	if !ok || s.RequireChallengeToken {
		t.Fatal("scout mints the token and must not be token-required")
	}
}
