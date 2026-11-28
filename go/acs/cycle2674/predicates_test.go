//go:build acs

// Package cycle2674 holds the acceptance predicates of the agy rule file size cut.
package cycle2674

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const agyRuleFileCutBytes = 24000

var agyLoadedRuleFiles = []string{"AGENTS.md", "GEMINI.md"}

func copyTreeInto(t *testing.T, srcRoot, dstRoot, rel string) {
	t.Helper()
	files, err := acsassert.WalkModule(filepath.Join(srcRoot, filepath.FromSlash(rel)), func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk %s: %v", rel, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(srcRoot, filepath.FromSlash(rel), filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		writeAt(t, dstRoot, rel+"/"+f, raw)
	}
}

func writeAt(t *testing.T, root, rel string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectWithAgentsMd(t *testing.T, agents string) string {
	t.Helper()
	repo := acsassert.RepoRoot(t)
	root := t.TempDir()
	phaseRegistry, err := os.ReadFile(filepath.Join(repo, "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeAt(t, root, "docs/architecture/phase-registry.json", phaseRegistry)
	for _, dir := range []string{"skills", "commands", "agents", ".evolve/profiles"} {
		copyTreeInto(t, repo, root, dir)
	}
	writeAt(t, root, "AGENTS.md", []byte(agents))
	return root
}

func agentsMdOfExactSize(bytesWanted int, filler string) string {
	head := "# AGENTS.md\n\n<!-- instructions-digest BEGIN -->\n## Hard rules\n\nThe pipeline is the only controller.\n<!-- instructions-digest END -->\n\n## Full rules\n\n"
	body := head
	for len(body)+len(filler)+1 <= bytesWanted {
		body += filler + "\n"
	}
	for len(body) < bytesWanted {
		body += "."
	}
	return body
}

func runSkillsCheck(t *testing.T, root string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := skillcheck.Run(root, false, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

func agentsMdProblemLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "AGENTS.md") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestC2674_001_ASkillsCheckRejectsAnAgentsMdOverTheAgyCut(t *testing.T) {
	cases := []struct {
		name   string
		agents string
	}{
		{"one byte over, ascii", agentsMdOfExactSize(agyRuleFileCutBytes+1, "The pipeline is the only controller.")},
		{"over in bytes though under in characters", agentsMdOfExactSize(agyRuleFileCutBytes+3, "Rule …… …… ……")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, output := runSkillsCheck(t, projectWithAgentsMd(t, c.agents))
			if code == 0 {
				t.Fatalf("skills check exit = 0 for an AGENTS.md of %d bytes; want a non-zero exit\n%s", len(c.agents), acsassert.OutputTail(output, 12))
			}
			lines := agentsMdProblemLines(output)
			if len(lines) == 0 {
				t.Fatalf("no problem line names AGENTS.md for %d bytes\n%s", len(c.agents), acsassert.OutputTail(output, 12))
			}
			joined := strings.Join(lines, "\n")
			if !strings.Contains(joined, "24000") && !strings.Contains(joined, "24,000") {
				t.Errorf("the AGENTS.md problem does not state the 24,000-byte cut: %s", joined)
			}
		})
	}
}

func TestC2674_002_ASkillsCheckAcceptsAnAgentsMdAtTheAgyCut(t *testing.T) {
	agents := agentsMdOfExactSize(agyRuleFileCutBytes, "The pipeline is the only controller.")
	if len(agents) != agyRuleFileCutBytes {
		t.Fatalf("fixture is %d bytes, want %d", len(agents), agyRuleFileCutBytes)
	}
	_, output := runSkillsCheck(t, projectWithAgentsMd(t, agents))
	if lines := agentsMdProblemLines(output); len(lines) != 0 {
		t.Errorf("an AGENTS.md of exactly %d bytes is reported: %v", agyRuleFileCutBytes, lines)
	}
}

func TestC2674_003_EachRuleFileThatAgyLoadsInTheRepositoryFitsItsCut(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, name := range agyLoadedRuleFiles {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Size() > agyRuleFileCutBytes {
			t.Errorf("%s is %d bytes; agy cuts a rule file at %d bytes", name, info.Size(), agyRuleFileCutBytes)
		}
	}
	rules, err := filepath.Glob(filepath.Join(root, ".agents", "rules", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range rules {
		if info, err := os.Stat(path); err != nil || info.Size() > agyRuleFileCutBytes {
			t.Errorf("%s is over the %d-byte cut or unreadable: %v", path, agyRuleFileCutBytes, err)
		}
	}
}

func TestC2674_004_TheSkillsCheckOfTheRepositoryPasses(t *testing.T) {
	code, output := runSkillsCheck(t, acsassert.RepoRoot(t))
	if code != 0 {
		t.Fatalf("evolve skills check exit = %d\n%s", code, acsassert.OutputTail(output, 20))
	}
	problems, err := skillcheck.Check(acsassert.RepoRoot(t))
	if err != nil || len(problems) != 0 {
		t.Errorf("skillcheck.Check = %v, %v; want no problem", problems, err)
	}
}

var probeHeading = regexp.MustCompile(`(?mi)^#{2,4} .*probe`)

func TestC2674_005_TheAgyRuleFileProbeResultIsRecordedInADocument(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "docs")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files docs: code=%d err=%v", code, err)
	}
	required := []string{"agy", "AGENTS.md", "GEMINI.md", "24,000"}
	var recorded []string
	for _, rel := range strings.Fields(out) {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		text := string(raw)
		if !probeHeading.MatchString(text) {
			continue
		}
		all := true
		for _, need := range required {
			all = all && strings.Contains(text, need)
		}
		if all {
			recorded = append(recorded, rel)
		}
	}
	if len(recorded) == 0 {
		t.Errorf("no document under docs/ (not gitignored) has a probe heading and names agy, AGENTS.md, GEMINI.md and the 24,000-byte cut")
	}
}
