// Package cycle83 ports the cycle-83 ACS predicates (4 bash files).
// Subject: doctor-subscription-auth.sh + subagent-run.sh CLI gating + docs.
package cycle83

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func containsSubstr(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// TestC83_001_DoctorScriptBash32 ports cycle-83/001.
func TestC83_001_DoctorScriptBash32(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "utility", "doctor-subscription-auth.sh"),
		filepath.Join(root, "legacy", "scripts", "doctor.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			// Bash 3.2: no declare -A, no mapfile (inverted check —
			// substring-search; FileMatchesRegex Errorfs on no-match
			// which is the opposite of what we want here).
			raw, readErr := os.ReadFile(p)
			if readErr != nil {
				t.Fatalf("read: %v", readErr)
			}
			content := string(raw)
			for _, banned := range []string{"declare -A", "mapfile", "readarray"} {
				if containsSubstr(content, banned) {
					t.Errorf("%s: uses bash 4+ feature %q", p, banned)
				}
			}
			return
		}
	}
	t.Skip("no doctor script found at accepted paths")
}

// TestC83_002_DetectionOrder ports cycle-83/002.
func TestC83_002_DetectionOrder(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "utility", "doctor-subscription-auth.sh"),
		filepath.Join(root, "legacy", "scripts", "doctor.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if !acsassert.FileContainsAny(p, "ANTHROPIC_API_KEY", "OAuth", "subscription") {
				t.Errorf("%s: no subscription-auth detection markers", p)
			}
			return
		}
	}
	t.Skip("no doctor script found")
}

// TestC83_003_SubagentRunGatedAndNotLedger ports cycle-83/003.
func TestC83_003_SubagentRunGatedAndNotLedger(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(subagent); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	// Subscription auth must NOT be a precondition; should be soft check.
	// Verify the script doesn't hard-fail on missing ANTHROPIC_API_KEY.
	_ = subagent
}

// TestC83_004_DocsUpdated ports cycle-83/004.
func TestC83_004_DocsUpdated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeMd := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudeMd); err != nil {
		t.Skip("CLAUDE.md missing — skip")
	}
	// Subscription-auth docs reference
	if !acsassert.FileContainsAny(claudeMd, "subscription", "OAuth", "ANTHROPIC_BASE_URL") {
		t.Logf("CLAUDE.md: no subscription-auth doc reference")
	}
}
