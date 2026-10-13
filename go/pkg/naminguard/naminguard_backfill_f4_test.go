package naminguard

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFix_BlankLinesInTheGrepListingAreSkippedAndEveryNamedFileIsRewritten(t *testing.T) {
	root := t.TempDir()
	token := "old" + "-name"
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("uses "+token+" here\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	orig := gitGrep
	t.Cleanup(func() { gitGrep = orig })
	gitGrep = func(string, ...string) (string, int, error) { return "a.txt\n\nb.txt\n", 0, nil }
	m := &Manifest{Forbidden: []Forbidden{{Token: token, Replacement: "new-name", Reason: "renamed"}}}

	changed, err := Fix(root, m)

	if err != nil || !reflect.DeepEqual(changed, []string{"a.txt", "b.txt"}) {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if got, _ := os.ReadFile(filepath.Join(root, name)); string(got) != "uses new-name here\n" {
			t.Errorf("%s = %q", name, got)
		}
	}
}
