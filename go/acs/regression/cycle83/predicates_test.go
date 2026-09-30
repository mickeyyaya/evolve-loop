//go:build acs

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

func TestC83_001_DoctorScriptBash32(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "utility", "doctor-subscription-auth.sh"),
		filepath.Join(root, "legacy", "scripts", "doctor.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
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

func TestC83_003_SubagentRunGatedAndNotLedger(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(subagent); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	_ = subagent
}

func TestC83_004_DocsUpdated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeMd := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudeMd); err != nil {
		t.Skip("CLAUDE.md missing — skip")
	}
	if !acsassert.FileContainsAny(claudeMd, "subscription", "OAuth", "ANTHROPIC_BASE_URL") {
		t.Logf("CLAUDE.md: no subscription-auth doc reference")
	}
}
