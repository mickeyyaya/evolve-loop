package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var removalAmpLink = regexp.MustCompile(`\]\(([^)#\s]+)(?:#[^)\s]*)?\)`)

func removalAmpRelativeLinks(doc string) []string {
	var out []string
	for _, m := range removalAmpLink.FindAllStringSubmatch(doc, -1) {
		target := m[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		out = append(out, target)
	}
	return out
}

func TestRemovalDesignAmp_EveryRelativeLinkResolves(t *testing.T) {
	t.Parallel()
	root := findRepoRoot(t)
	for _, rel := range []string{removalDesignPath, "docs/plans/harness-practices-2026-11.md"} {
		body := readRepoFile(t, rel)
		dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
		for _, target := range removalAmpRelativeLinks(body) {
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(target))); err != nil {
				t.Errorf("%s links to %s which does not resolve: %v", rel, target, err)
			}
		}
	}
}

func TestRemovalDesignAmp_TheLinkCheckerSeesBrokenAndSkipsExternal(t *testing.T) {
	t.Parallel()
	got := removalAmpRelativeLinks("[a](x.md) [b](https://e.com/y) [c](z.md#h) [d](mailto:q@e.com) []()")
	if len(got) != 2 || got[0] != "x.md" || got[1] != "z.md" {
		t.Fatalf("links = %v", got)
	}
}

func TestRemovalDesignAmp_TheR9RowLinksOnlyToAnExistingDesign(t *testing.T) {
	t.Parallel()
	root := findRepoRoot(t)
	plan := readRepoFile(t, "docs/plans/harness-practices-2026-11.md")
	rows := 0
	for _, line := range strings.Split(plan, "\n") {
		if !strings.HasPrefix(line, "| R9 ") {
			continue
		}
		rows++
		if !strings.Contains(line, "(harness-removal-test-2026-11.md)") {
			t.Errorf("R9 row misses the design link: %s", line)
		}
	}
	if rows != 1 {
		t.Fatalf("expected exactly one R9 row, found %d", rows)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(removalDesignPath))); err != nil {
		t.Fatal(err)
	}
}

func TestRemovalDesignAmp_ThePlanStillLintsInSTEAlongsideTheDesign(t *testing.T) {
	t.Parallel()
	root := findRepoRoot(t)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"docs", "ste-lint", "--strict", "--project-root", root,
		filepath.Join(root, filepath.FromSlash(removalDesignPath)),
		filepath.Join(root, "docs", "plans", "harness-practices-2026-11.md")}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "2 file(s) checked") {
		t.Fatalf("unexpected lint scope: %q", stdout.String())
	}
}

func TestRemovalDesignAmp_TheDesignHasOneSectionPerRequiredHeading(t *testing.T) {
	t.Parallel()
	body := readRepoFile(t, removalDesignPath)
	heading := regexp.MustCompile(`(?m)^## (?:\d+\. )?(Component|Window|Measure|Stop rule|Precondition)\b`)
	seen := map[string]int{}
	for _, m := range heading.FindAllStringSubmatch(body, -1) {
		seen[m[1]]++
	}
	for _, name := range []string{"Component", "Window", "Measure", "Stop rule", "Precondition"} {
		if seen[name] != 1 {
			t.Errorf("heading %q appears %d times", name, seen[name])
		}
	}
}

func TestRemovalDesignAmp_TheStopRuleHoldsANumber(t *testing.T) {
	t.Parallel()
	body := readRepoFile(t, removalDesignPath)
	sections := removalDesignSections(body)
	var stop string
	for name, text := range sections {
		if strings.Contains(strings.ToLower(name), "stop") {
			stop = text
		}
	}
	if !regexp.MustCompile(`\d`).MatchString(stop) {
		t.Fatalf("stop rule has no numeric threshold: %q", stop)
	}
}
