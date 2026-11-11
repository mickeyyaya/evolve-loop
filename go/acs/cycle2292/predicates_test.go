//go:build acs

// Package cycle2292 holds the ACS predicates of cycle 2292.
package cycle2292

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	trainSectionHeading = "## The console PR train"
	twoCommitTest       = "TestConsoleTwoCommitPRLandsAsTwoIntegrateCommits"
)

func trainSection(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "operations", "wave-operations.md"))
	if err != nil {
		t.Fatalf("read wave-operations.md: %v", err)
	}
	doc := string(raw)
	start := strings.Index(doc, trainSectionHeading)
	if start < 0 {
		t.Fatalf("wave-operations.md has no %q section", trainSectionHeading)
	}
	section := doc[start+len(trainSectionHeading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

func TestC2292_001_DocsStateAMultiCommitPRIsNotSquashed(t *testing.T) {
	section := trainSection(t)
	// acs-predicate: config-check
	notSquashed := regexp.MustCompile(`(?i)(does not|do not|never|no)\s+squash|without\s+(a\s+)?squash`)
	if !notSquashed.MatchString(section) {
		t.Errorf("RED: the console PR train section does not say that the train never squashes the commits of a PR")
	}
	// acs-predicate: config-check
	perCommit := regexp.MustCompile(`(?i)\b(two|2|N|more than one|many|several)\s+commits`)
	if !perCommit.MatchString(section) {
		t.Errorf("RED: the console PR train section does not say how the train lands a PR with more than one commit")
	}
}

func TestC2292_002_DocsDropTheOneCommitPerPRWording(t *testing.T) {
	section := trainSection(t)
	// acs-predicate: config-check
	stale := regexp.MustCompile(`(?i)PR\s+as\s+one\s+commit`)
	if loc := stale.FindString(section); loc != "" {
		t.Errorf("RED: the section still says %q; a PR of N commits lands as N commits", loc)
	}
}

type testEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
}

func TestC2292_003_TwoCommitTrainTestExistsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-json",
		"-run", "^"+twoCommitTest+"$", "./internal/integration")
	actions := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var ev testEvent
		if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Test == twoCommitTest {
			actions[ev.Action] = true
		}
	}
	if !actions["run"] {
		t.Fatalf("RED: %s does not exist in go/internal/integration (exit %d)\n%s", twoCommitTest, code, acsassert.OutputTail(stdout+stderr, 15))
	}
	if actions["skip"] {
		t.Errorf("RED: %s is skipped", twoCommitTest)
	}
	if !actions["pass"] || actions["fail"] || code != 0 {
		t.Errorf("RED: %s did not pass (exit %d)\n%s", twoCommitTest, code, acsassert.OutputTail(stdout+stderr, 15))
	}
}
