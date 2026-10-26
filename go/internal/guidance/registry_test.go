package guidance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_KeepsTheValidRulesAndJoinsEachDecodeError(t *testing.T) {
	root := fixtureTree(t, ruleSpec{id: "good"})
	writeFile(t, root, "guidance/rules/bad.md", "no frontmatter\n")
	writeFile(t, root, "guidance/rules/README.txt", "not a rule\n")
	if err := os.Mkdir(filepath.Join(root, "guidance", "rules", "sub.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	rules, err := Load(root)

	if len(rules) != 1 || rules[0].ID != "good" {
		t.Fatalf("Load rules = %+v, want only good", rules)
	}
	if !IsDecodeError(err) {
		t.Fatalf("Load error = %v, want a DecodeError for bad.md", err)
	}
}

func TestLoad_AnAbsentRegistryIsAnIOError(t *testing.T) {
	_, err := Load(t.TempDir())

	if err == nil || IsDecodeError(err) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load error = %v, want a not-exist I/O error", err)
	}
}

func TestLoad_AnUnreadableRuleFileIsAnIOError(t *testing.T) {
	root := fixtureTree(t)
	if err := os.Symlink(filepath.Join(root, "absent"), filepath.Join(root, "guidance", "rules", "dangling.md")); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root)

	if err == nil || IsDecodeError(err) {
		t.Fatalf("Load error = %v, want an I/O error", err)
	}
}

func TestActive_DropsInactiveRulesAndOrdersByLayerOrderAndID(t *testing.T) {
	rules := []Rule{
		{ID: "z", Layer: "L2", Status: StatusActive},
		{ID: "b", Layer: "L0", Status: StatusActive},
		{ID: "a", Layer: "L0", Status: StatusActive},
		{ID: "first", Layer: "L0", Order: 1, Status: StatusActive},
		{ID: "second", Layer: "L0", Order: 2, Status: StatusActive},
		{ID: "gone", Layer: "L0", Status: "retired"},
	}

	got := Active(rules)

	want := []string{"first", "second", "a", "b", "z"}
	if len(got) != len(want) {
		t.Fatalf("Active = %+v, want %v", got, want)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("Active[%d] = %s, want %s (all %+v)", i, got[i].ID, id, got)
		}
	}
}

func TestBodyPath_IsTheRegistryFileOfTheID(t *testing.T) {
	if got := BodyPath("x-y"); got != "guidance/rules/x-y.md" {
		t.Fatalf("BodyPath = %q", got)
	}
}
