// Package evidence implements the ADR-0027 commit-as-evidence trailer: the
// provenance block appended to a phase's commit on the per-cycle worktree
// branch. A phase has delivered iff it produced a commit carrying a well-formed
// Evolve-Phase trailer with the cycle's challenge token — completion is "HEAD
// advanced + trailer verified", never "a file appeared at the polled path".
//
// This package is PURE (stdlib only) and imports nothing from the bridge,
// orchestrator, or git layers, so both the completion detector (bridge) and the
// commit emitter (orchestrator, a later PR) can depend on it without a cycle.
package evidence

import (
	"fmt"
	"strconv"
	"strings"
)

// Trailer keys. Evolve-* are this project's namespace; Challenge-Token reuses
// the existing per-launch challenge token already minted by the bridge.
const (
	KeyPhase        = "Evolve-Phase"
	KeyCycle        = "Evolve-Cycle"
	KeyChallenge    = "Challenge-Token"
	KeyArtifactSHA  = "Evolve-Artifact-SHA256"
	KeyArtifactPath = "Evolve-Artifact-Path"
)

// Trailer is the parsed/buildable provenance block for one phase commit.
// Phase + Challenge are load-bearing (completion verification); the artifact
// fields are forensic provenance (which deliverable this commit evidences).
type Trailer struct {
	Phase        string
	Cycle        int
	Challenge    string
	ArtifactSHA  string
	ArtifactPath string
}

// Build renders the trailer as the block appended to a commit message: a
// leading blank line then one "Key: Value" per non-empty field, in a stable
// order (deterministic output ⇒ reproducible commits + cache-friendly). Cycle
// is emitted when > 0. Empty optional fields are omitted.
//
// Emitter contract (load-bearing for Parse round-trip): the trailing block must
// be CONTIGUOUS — no blank line between trailer lines — because a blank line
// terminates the git-trailer run that Parse reads (Parse stops at the first
// non-trailer line scanning back from the end). Build guarantees this. Also:
// a Trailer with an empty Phase Builds a block that Verify can never accept
// (Verify is fail-closed on Phase) — the emitter (PR3) must always set Phase.
func (t Trailer) Build() string {
	var b strings.Builder
	b.WriteString("\n")
	writeLine(&b, KeyPhase, t.Phase)
	if t.Cycle > 0 {
		fmt.Fprintf(&b, "%s: %d\n", KeyCycle, t.Cycle)
	}
	writeLine(&b, KeyChallenge, t.Challenge)
	writeLine(&b, KeyArtifactSHA, t.ArtifactSHA)
	writeLine(&b, KeyArtifactPath, t.ArtifactPath)
	return b.String()
}

func writeLine(b *strings.Builder, key, val string) {
	if val != "" {
		fmt.Fprintf(b, "%s: %s\n", key, val)
	}
}

// Parse extracts a Trailer from a full commit message. It reads the trailing
// "Key: Value" block (the git-trailer convention: the contiguous run of trailer
// lines at the END of the message), so an earlier body line that happens to
// contain a colon is never mistaken for a trailer. Unknown keys are ignored;
// missing keys leave their zero value. A message with no trailer block yields a
// zero Trailer (Phase == "").
func Parse(msg string) Trailer {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	// Walk backward over the contiguous trailing run of "Key: Value" lines.
	start := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		if isTrailerLine(lines[i]) {
			start = i
			continue
		}
		break
	}
	var t Trailer
	for _, ln := range lines[start:] {
		key, val, ok := cutTrailer(ln)
		if !ok {
			continue
		}
		switch key {
		case KeyPhase:
			t.Phase = val
		case KeyCycle:
			if n, err := strconv.Atoi(val); err == nil {
				t.Cycle = n
			}
		case KeyChallenge:
			t.Challenge = val
		case KeyArtifactSHA:
			t.ArtifactSHA = val
		case KeyArtifactPath:
			t.ArtifactPath = val
		}
	}
	return t
}

// Verify reports whether the trailer evidences the given phase with the
// expected challenge token — the completion-detection predicate. Both must be
// non-empty and match; an empty expectedToken is rejected (fail-closed) so a
// missing token can never satisfy verification.
func (t Trailer) Verify(phase, expectedToken string) bool {
	return phase != "" && expectedToken != "" &&
		t.Phase == phase && t.Challenge == expectedToken
}

// isTrailerLine reports whether ln has the "Token: value" trailer shape. The
// key is a single token of letters/digits/hyphens (the git-trailer grammar),
// which excludes prose lines like "see: http://..." only loosely — but the
// backward contiguous-run scan in Parse is what bounds the block.
func isTrailerLine(ln string) bool {
	_, _, ok := cutTrailer(ln)
	return ok
}

// cutTrailer splits "Key: Value" into (key, value, ok). ok is false unless the
// part before ": " is a non-empty run of [A-Za-z0-9-] (the trailer-key grammar).
func cutTrailer(ln string) (key, val string, ok bool) {
	idx := strings.Index(ln, ": ")
	if idx <= 0 {
		return "", "", false
	}
	key = ln[:idx]
	for i := 0; i < len(key); i++ {
		if !isKeyByte(key[i]) {
			return "", "", false
		}
	}
	return key, strings.TrimSpace(ln[idx+2:]), true
}

// isKeyByte reports whether c is valid in a trailer key: [A-Za-z0-9-].
func isKeyByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}
