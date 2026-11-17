package scopedset

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readRecords(t *testing.T, path string) []Record {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("line %q is not one JSON record: %v", sc.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func TestShadow_AppendWritesOneLinePerRecordUnderThePlane(t *testing.T) {
	t.Parallel()
	plane := filepath.Join(t.TempDir(), ".evolve")
	first := Record{Stage: "shadow", Cycle: 7, AuditedBase: "base-1", Lane: []string{"go/a/a.go"}, Main: []string{"go/d/d.go"},
		Packages: []string{"./e/..."}, Escapes: []Escape{{Gate: "test", Package: "example.com/m/b", Test: "TestB"}}}
	second := Record{Stage: "shadow", Cycle: 8, AuditedBase: "base-2", All: true, Reason: "go.mod changed"}

	path, err := Append(plane, first)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := Append(plane, second); err != nil || again != path {
		t.Fatalf("second append = (%q, %v), want the same file %q", again, err, path)
	}

	if want := filepath.Join(plane, ArtifactName); path != want {
		t.Errorf("record file = %q, want %q", path, want)
	}
	if got := readRecords(t, path); !reflect.DeepEqual(got, []Record{first, second}) {
		t.Errorf("records = %+v, want %+v", got, []Record{first, second})
	}
}

func TestShadow_AppendRefusesAnEmptyPlane(t *testing.T) {
	t.Parallel()
	if path, err := Append("", Record{Stage: "shadow"}); err == nil {
		t.Errorf("Append with no plane wrote %q; a record that was never written must not look like a quiet wave", path)
	}
}

func TestShadow_AppendReportsAPlaneOrFileThatCannotBeUsed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "plane-is-a-file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(filepath.Join(blocker, "sub"), Record{Stage: "shadow"}); err == nil {
		t.Error("a plane below a regular file must fail")
	}
	plane := filepath.Join(dir, "plane")
	if err := os.MkdirAll(filepath.Join(plane, ArtifactName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(plane, Record{Stage: "shadow"}); err == nil {
		t.Error("a record file that is a directory must fail")
	}
}
