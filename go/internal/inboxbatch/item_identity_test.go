package inboxbatch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestItem_KeyIsTheIDAndThePath(t *testing.T) {
	if got := (Item{ID: "x", Path: "a.json", Weight: 0.5}).Key(); got != (ItemKey{ID: "x", Path: "a.json"}) {
		t.Errorf("Key = %+v", got)
	}
}

func TestLoadFile_RecordsAnIDTakenFromTheFileName(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"no-id.json": `{"weight":0.5}`, "with-id.json": `{"id":"named","weight":0.5}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fallback, _, err := LoadFile(filepath.Join(dir, "no-id.json"))
	if err != nil || fallback.ID != "no-id" || !fallback.IDFromFileName {
		t.Errorf("an id-less record takes its file name and says so: %+v %v", fallback, err)
	}
	named, _, err := LoadFile(filepath.Join(dir, "with-id.json"))
	if err != nil || named.ID != "named" || named.IDFromFileName {
		t.Errorf("a recorded id is not a fallback: %+v %v", named, err)
	}
}

func TestRequirePriorityClass_RefusesOnlyABlankClass(t *testing.T) {
	for _, blank := range []string{"", "   "} {
		if err := RequirePriorityClass(blank); !errors.Is(err, ErrNoPriorityClass) {
			t.Errorf("RequirePriorityClass(%q) = %v, want ErrNoPriorityClass", blank, err)
		}
	}
	if err := RequirePriorityClass(ClassHygiene); err != nil {
		t.Errorf("a named class is a filing's class: %v", err)
	}
}

func TestClassNames_AreThePolicysCompiledClassOrder(t *testing.T) {
	named := []string{
		ClassCorrectness, ClassStability, ClassPerformance, ClassDebuggability,
		ClassFeature, ClassMaintainability, ClassHygiene, ClassSecurity,
	}
	if want := (policy.Policy{}).InboxPriorityConfig().ClassOrder; !slices.Equal(named, want) {
		t.Errorf("the writers' class names %v must be the policy's compiled class_order %v", named, want)
	}
}
