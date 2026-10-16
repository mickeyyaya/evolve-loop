// Package evidence implements the ADR-0027 commit-as-evidence trailer: the
// provenance block appended to a phase's commit on the per-cycle worktree
// branch.
package evidence

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	KeyPhase        = "Evolve-Phase"
	KeyCycle        = "Evolve-Cycle"
	KeyChallenge    = "Challenge-Token"
	KeyArtifactSHA  = "Evolve-Artifact-SHA256"
	KeyArtifactPath = "Evolve-Artifact-Path"
)

type Trailer struct {
	Phase        string
	Cycle        int
	Challenge    string
	ArtifactSHA  string
	ArtifactPath string
}

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

func Parse(msg string) Trailer {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
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

func (t Trailer) Verify(phase, expectedToken string) bool {
	return phase != "" && expectedToken != "" &&
		t.Phase == phase && t.Challenge == expectedToken
}

func isTrailerLine(ln string) bool {
	_, _, ok := cutTrailer(ln)
	return ok
}

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

func isKeyByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}
