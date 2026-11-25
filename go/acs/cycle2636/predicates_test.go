//go:build acs

package cycle2636

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const waveOperationsRelPath = "docs/operations/wave-operations.md"

func readDoc(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(waveOperationsRelPath)))
	if err != nil {
		t.Fatalf("read %s: %v", waveOperationsRelPath, err)
	}
	return string(raw)
}

func headingLevel(line string) int {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n >= len(line) || line[n] != ' ' {
		return 0
	}
	return n
}

func protectionSection(t *testing.T, doc string) []string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start, level := -1, 0
	for i, line := range lines {
		l := headingLevel(line)
		if l > 0 && strings.Contains(strings.ToLower(line), "protection") {
			start, level = i, l
			break
		}
	}
	if start < 0 {
		t.Fatalf("RED: %s has no heading about the protection of main", waveOperationsRelPath)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if l := headingLevel(lines[i]); l > 0 && l <= level {
			end = i
			break
		}
	}
	return lines[start:end]
}

func lineWithAll(section []string, needles ...string) bool {
	for _, line := range section {
		low := strings.ToLower(line)
		ok := true
		for _, n := range needles {
			if !strings.Contains(low, strings.ToLower(n)) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// acs-predicate: config-check
func TestC2636_001_DocStatesTheLiveProtectionOfMainStepOne(t *testing.T) {
	section := protectionSection(t, readDoc(t))
	for _, needle := range []string{
		"2026-11-24",
		"allow_force_pushes=false",
		"allow_deletions=false",
		"enforce_admins=false",
	} {
		if !lineWithAll(section, needle) {
			t.Errorf("RED: the protection section never states %q", needle)
		}
	}
	if !lineWithAll(section, "step 1", "applied") {
		t.Errorf("RED: no line says step 1 is applied")
	}
	if !lineWithAll(section, "no required checks") {
		t.Errorf("RED: no line says main has no required checks")
	}
	if !lineWithAll(section, "no required reviews") {
		t.Errorf("RED: no line says main has no required reviews")
	}
}

// acs-predicate: config-check
func TestC2636_002_DocShowsHowToReadTheProtectionBack(t *testing.T) {
	section := protectionSection(t, readDoc(t))
	if !lineWithAll(section, "gh api", "branches/main/protection") {
		t.Errorf("RED: no line gives the read-back command `gh api .../branches/main/protection`")
	}
}

// acs-predicate: config-check
func TestC2636_003_DocSaysStepTwoWaitsForI9AndIsNotLive(t *testing.T) {
	section := protectionSection(t, readDoc(t))
	if !lineWithAll(section, "step 2", "one required check", "I9") {
		t.Errorf("RED: no line says step 2 (one required check) waits for I9")
	}
	for _, line := range section {
		low := strings.ToLower(line)
		if strings.Contains(low, "step 2") && !strings.Contains(line, "I9") {
			t.Errorf("RED: a step 2 line never ties step 2 to I9, so it can read as live: %q", line)
		}
		if strings.Contains(low, "step 2") && strings.Contains(low, "applied") && !strings.Contains(low, "not") && !strings.Contains(low, "wait") {
			t.Errorf("RED: a step 2 line claims step 2 is applied: %q", line)
		}
	}
}

// acs-predicate: config-check
func TestC2636_004_DocNamesTheEnforceAdminsConsequence(t *testing.T) {
	section := protectionSection(t, readDoc(t))
	if !lineWithAll(section, "enforce_admins=false", "admin") {
		t.Fatalf("RED: no line ties enforce_admins=false to admin identities")
	}
	consequence := false
	for _, line := range section {
		low := strings.ToLower(line)
		if !strings.Contains(low, "admin") {
			continue
		}
		bindsNot := strings.Contains(low, "do not bind") || strings.Contains(low, "does not bind") || strings.Contains(low, "never bind")
		namesUsers := strings.Contains(low, "console") && strings.Contains(low, "lane") && strings.Contains(low, "train")
		if bindsNot && namesUsers {
			consequence = true
		}
	}
	if !consequence {
		t.Errorf("RED: no line says the rules do not bind admin identities and names the console, the lanes and the train")
	}
}

func TestC2636_005_ProtectionSectionPassesSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	section := strings.Join(protectionSection(t, readDoc(t)), "\n")
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, f := range stelint.Check([]byte(section), stelint.Options{Words: words}) {
		if f.IsSkipNotice() {
			continue
		}
		t.Errorf("RED: STE finding at section line %d [%s]: %s — %s", f.Line, f.Rule, f.Message, f.Excerpt)
	}
}

func TestC2636_006_DocKeepsItsExistingSectionsAndStaysTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := readDoc(t)
	for _, heading := range []string{
		"# Run a wave with `evolve wave`",
		"## 3. At the boundary",
		"## 6. Other cases",
		"## The console PR train",
	} {
		if !strings.Contains(doc, "\n"+heading+"\n") && !strings.HasPrefix(doc, heading+"\n") {
			t.Errorf("preservation: heading %q is gone", heading)
		}
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", waveOperationsRelPath); code != 0 {
		t.Errorf("%s is not tracked by git", waveOperationsRelPath)
	}
}
