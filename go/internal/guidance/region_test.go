package guidance

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestL0Region_RoundTripsTheBlockThatBuildWrites(t *testing.T) {
	t.Parallel()
	root := fixtureTree(t, ruleSpec{id: "alpha", statement: "Alpha holds."}, ruleSpec{id: "beta", statement: "Beta holds."})
	rules, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := Outputs(root, rules)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, outs); err != nil {
		t.Fatal(err)
	}
	got, err := L0Region(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- Alpha holds.", "- Beta holds.", PointerLine} {
		if !strings.Contains(got, want) {
			t.Errorf("region lacks %q:\n%s", want, got)
		}
	}
	for _, marker := range []string{L0Begin, L0End} {
		if strings.Contains(got, marker) {
			t.Errorf("region must not carry the marker %q", marker)
		}
	}
	if !strings.HasPrefix(got, "### Hard rules") || !strings.HasSuffix(got, PointerLine) {
		t.Errorf("region must run from the heading to the pointer line:\n%s", got)
	}
}

func TestL0Region_RefusesAFileWithoutBothMarkers(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"none":    "# AGENTS\n",
		"no end":  "# AGENTS\n" + L0Begin + " x -->\nbody\n",
		"no line": L0Begin,
	} {
		root := t.TempDir()
		writeFile(t, root, AgentsPath, text)
		if _, err := L0Region(root); !errors.Is(err, ErrNoL0Region) {
			t.Errorf("%s: err = %v, want ErrNoL0Region", name, err)
		}
	}
}

func TestL0Region_ReportsAMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := L0Region(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want a missing-file error", err)
	}
}

func TestL0Region_TheCheckedInAgentsFileMatchesTheRegistry(t *testing.T) {
	t.Parallel()
	rules, err := Load("../../..")
	if err != nil {
		t.Fatal(err)
	}
	got, err := L0Region("../../..")
	if err != nil {
		t.Fatal(err)
	}
	block := L0Block(rules)
	if want := strings.TrimRight(block[strings.Index(block, "\n")+1:strings.Index(block, L0End)], "\n"); got != want {
		t.Errorf("AGENTS.md L0 region differs from the registry; run evolve guidance build")
	}
}
