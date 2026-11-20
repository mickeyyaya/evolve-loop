package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const removalDesignPath = "docs/plans/harness-removal-test-2026-11.md"

var removalDesignHeadingRE = regexp.MustCompile(`^##\s+(?:\d+\.\s+)?(.+?)\s*$`)

var removalDesignRequiredSections = []string{"Component", "Window", "Measure", "Stop rule", "Precondition"}

func removalDesignSections(doc string) map[string]string {
	sections := map[string]string{}
	current := ""
	for _, line := range strings.Split(doc, "\n") {
		if m := removalDesignHeadingRE.FindStringSubmatch(line); m != nil {
			current = strings.ToLower(m[1])
			sections[current] = ""
			continue
		}
		if current != "" {
			sections[current] += line + "\n"
		}
	}
	return sections
}

func removalDesignFindings(doc string) []string {
	sections := removalDesignSections(doc)
	var findings []string
	body := map[string]string{}
	for _, name := range removalDesignRequiredSections {
		text, ok := sections[strings.ToLower(name)]
		switch {
		case !ok:
			findings = append(findings, fmt.Sprintf("missing section %q", name))
		case strings.TrimSpace(text) == "":
			findings = append(findings, fmt.Sprintf("empty section %q", name))
		default:
			body[name] = strings.ToLower(text)
		}
	}
	need := []struct {
		section, token, why string
	}{
		{"Component", "`", "one component named in code font"},
		{"Window", "wave", "a wave window"},
		{"Measure", "ship rate", "the ship rate"},
		{"Measure", "cost", "the cost"},
		{"Stop rule", "stop", "a stop rule"},
		{"Precondition", "m2 baseline", "the M2 baseline"},
		{"Precondition", "adr-0133", "ADR-0133"},
		{"Precondition", "wait", "that the test waits"},
	}
	for _, n := range need {
		if text, ok := body[n.section]; ok && !strings.Contains(text, n.token) {
			findings = append(findings, fmt.Sprintf("section %q lacks %s", n.section, n.why))
		}
	}
	for _, name := range []string{"Window", "Stop rule"} {
		if text, ok := body[name]; ok && !regexp.MustCompile(`\d`).MatchString(text) {
			findings = append(findings, fmt.Sprintf("section %q lacks a number", name))
		}
	}
	return findings
}

func removalDesignFixture(drop string, edits map[string]string) string {
	bodies := map[string]string{
		"Component":    "The test disables `planreview`, one component.",
		"Window":       "The window is 3 waves after the M2 baseline.",
		"Measure":      "Compare the ship rate and the cost of each shipped cycle.",
		"Stop rule":    "Stop after 2 waves with a ship rate drop of 20 percent.",
		"Precondition": "The test waits for the M2 baseline of ADR-0133.",
	}
	var doc strings.Builder
	doc.WriteString("# Design\n\n")
	for _, name := range removalDesignRequiredSections {
		if name == drop {
			continue
		}
		text := bodies[name]
		if edit, ok := edits[name]; ok {
			text = edit
		}
		fmt.Fprintf(&doc, "## %s\n\n%s\n\n", name, text)
	}
	return doc.String()
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(findRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func TestRemovalDesign_TheCheckerAcceptsAFullDesign(t *testing.T) {
	t.Parallel()
	if got := removalDesignFindings(removalDesignFixture("", nil)); len(got) != 0 {
		t.Fatalf("a full design has findings %v; want none", got)
	}
}

func TestRemovalDesign_TheCheckerNamesEachMissingSection(t *testing.T) {
	t.Parallel()
	for _, name := range removalDesignRequiredSections {
		got := removalDesignFindings(removalDesignFixture(name, nil))
		want := fmt.Sprintf("missing section %q", name)
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("design without %q gives findings %v; want %q", name, got, want)
		}
	}
}

func TestRemovalDesign_TheCheckerRefusesAnEmptySectionAndMissingParts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		edits map[string]string
		want  string
	}{
		{"empty stop rule", map[string]string{"Stop rule": ""}, `empty section "Stop rule"`},
		{"component without code font", map[string]string{"Component": "The test disables a reviewer."}, `section "Component" lacks one component`},
		{"window without a number", map[string]string{"Window": "The window is some waves of the wave pool."}, `section "Window" lacks a number`},
		{"measure without cost", map[string]string{"Measure": "Compare the ship rate."}, `section "Measure" lacks the cost`},
		{"stop rule without a threshold", map[string]string{"Stop rule": "Stop when it looks bad."}, `section "Stop rule" lacks a number`},
		{"precondition without the baseline", map[string]string{"Precondition": "The test waits for ADR-0133."}, `section "Precondition" lacks the M2 baseline`},
		{"precondition without the ADR", map[string]string{"Precondition": "The test waits for the M2 baseline."}, `section "Precondition" lacks ADR-0133`},
		{"precondition that does not wait", map[string]string{"Precondition": "The M2 baseline of ADR-0133 exists."}, `section "Precondition" lacks that the test waits`},
	}
	for _, c := range cases {
		got := strings.Join(removalDesignFindings(removalDesignFixture("", c.edits)), "\n")
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: findings %q; want %q", c.name, got, c.want)
		}
	}
}

func TestRemovalDesign_TheCheckerReadsNumberedHeadings(t *testing.T) {
	t.Parallel()
	doc := strings.NewReplacer("## Component", "## 1. Component", "## Window", "## 2. Window").Replace(removalDesignFixture("", nil))
	if got := removalDesignFindings(doc); len(got) != 0 {
		t.Fatalf("numbered headings give findings %v; want none", got)
	}
}

func TestRemovalDesign_TheDesignNamesTheComponentTheWindowTheMeasureAndTheStopRule(t *testing.T) {
	t.Parallel()
	got := removalDesignFindings(readRepoFile(t, removalDesignPath))
	if len(got) != 0 {
		t.Fatalf("%s has findings:\n%s", removalDesignPath, strings.Join(got, "\n"))
	}
}

func TestRemovalDesign_TheDesignWaitsForTheM2BaselineOfADR0133(t *testing.T) {
	t.Parallel()
	sections := removalDesignSections(readRepoFile(t, removalDesignPath))
	text := strings.ToLower(sections["precondition"])
	for _, token := range []string{"m2 baseline", "adr-0133", "wait"} {
		if !strings.Contains(text, token) {
			t.Errorf("precondition of %s lacks %q:\n%s", removalDesignPath, token, text)
		}
	}
}

func TestRemovalDesign_ThePlanLinksItsR9RowToTheDesign(t *testing.T) {
	t.Parallel()
	plan := readRepoFile(t, "docs/plans/harness-practices-2026-11.md")
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "| R9 |") {
			if !strings.Contains(line, "(harness-removal-test-2026-11.md)") {
				t.Fatalf("the R9 row has no link to the design: %s", line)
			}
			return
		}
	}
	t.Fatal("the plan has no R9 row")
}

func TestRemovalDesign_TheDocsEntryPointLintsTheDesignInSTE(t *testing.T) {
	t.Parallel()
	root := findRepoRoot(t)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"docs", "ste-lint", "--strict", "--project-root", root, filepath.Join(root, filepath.FromSlash(removalDesignPath))}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evolve docs ste-lint --strict %s exit %d\nstdout: %s\nstderr: %s", removalDesignPath, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 file(s) checked") {
		t.Fatalf("the lint did not check the design: %q", stdout.String())
	}
}
