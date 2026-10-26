package guidance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputs_SplicesTheL0RegionAndKeepsTheRestOfAGENTS(t *testing.T) {
	root := fixtureTree(t, ruleSpec{id: "safe"})
	rules, _ := Load(root)

	outs, err := Outputs(root, rules)

	if err != nil || len(outs) != 3 {
		t.Fatalf("Outputs = %+v, %v", outs, err)
	}
	agents := outs[2].Content
	if outs[2].Path != AgentsPath || !strings.Contains(agents, "- Rule safe holds.\n") || strings.Contains(agents, "\nold\n") || !strings.HasSuffix(agents, "<!-- instructions-digest END -->\n") {
		t.Fatalf("AGENTS output =\n%s", agents)
	}
}

func TestOutputs_FailsClosedOnAnAbsentOrBrokenRegion(t *testing.T) {
	cases := map[string]struct {
		agents     string
		isNoRegion bool
	}{
		"no region":     {"# AGENTS\n", true},
		"no end marker": {"# AGENTS\n" + L0Begin + " -->\n", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := fixtureTree(t)
			writeFile(t, root, AgentsPath, c.agents)
			_, err := Outputs(root, nil)
			if err == nil || errors.Is(err, ErrNoL0Region) != c.isNoRegion {
				t.Fatalf("Outputs error = %v, want no-region=%v", err, c.isNoRegion)
			}
		})
	}
}

func TestOutputs_AnAbsentAGENTSIsAnIOError(t *testing.T) {
	_, err := Outputs(t.TempDir(), nil)

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Outputs error = %v, want not-exist", err)
	}
}

func TestWrite_WritesOnlyTheStaleOutputsAndASecondRunChangesNothing(t *testing.T) {
	root := fixtureTree(t, ruleSpec{id: "safe"})
	rules, _ := Load(root)
	outs, _ := Outputs(root, rules)

	first, err := Write(root, outs)
	if err != nil || len(first) != 3 {
		t.Fatalf("first Write = %v, %v; want 3 written", first, err)
	}
	second, err := Write(root, outs)

	if err != nil || len(second) != 0 {
		t.Fatalf("second Write = %v, %v; want nothing written", second, err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "guidance", "index.json"))
	if string(got) != outs[1].Content {
		t.Fatalf("index.json = %s", got)
	}
}

func TestWrite_ReportsAnUnreadableOrUnwritableOutput(t *testing.T) {
	t.Run("an output path is a directory", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "guidance", "index.md"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Write(root, []Output{{Path: IndexMarkdownPath, Content: "x"}}); err == nil {
			t.Fatal("Write = nil error, want a read error")
		}
	})
	t.Run("the parent directory is absent", func(t *testing.T) {
		if _, err := Write(t.TempDir(), []Output{{Path: IndexMarkdownPath, Content: "x"}}); err == nil {
			t.Fatal("Write = nil error, want a write error")
		}
	})
}
