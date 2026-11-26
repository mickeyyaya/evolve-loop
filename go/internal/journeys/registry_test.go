package journeys

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadJSON(t *testing.T, content string) ([]Row, error) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, RegistryPath, content)
	return Load(filepath.Join(root, filepath.FromSlash(RegistryPath)))
}

const goodRows = `[
 {"id":"a-journey","journey":"A path works","test":"internal/x.TestA","issue":"#1"},
 {"id":"b-journey","journey":"B path works","test":"internal/y.TestB","issue":"#2","retired":"the path was removed"}
]`

func TestLoad_ReadsEachRowWithItsRetiredReason(t *testing.T) {
	rows, err := loadJSON(t, goodRows)
	if err != nil {
		t.Fatal(err)
	}
	want := []Row{
		{ID: "a-journey", Journey: "A path works", Test: "internal/x.TestA", Issue: "#1"},
		{ID: "b-journey", Journey: "B path works", Test: "internal/y.TestB", Issue: "#2", Retired: "the path was removed"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestLoad_RefusesARegistryThatCannotBeTrusted(t *testing.T) {
	tests := map[string]string{
		"not json":        `[{`,
		"duplicate id":    `[{"id":"a","journey":"j","test":"p.TestA","issue":"i"},{"id":"a","journey":"j","test":"p.TestB","issue":"i"}]`,
		"empty journey":   `[{"id":"a","journey":"","test":"p.TestA","issue":"i"}]`,
		"empty issue":     `[{"id":"a","journey":"j","test":"p.TestA","issue":""}]`,
		"empty id":        `[{"id":"","journey":"j","test":"p.TestA","issue":"i"}]`,
		"no package":      `[{"id":"a","journey":"j","test":"TestA","issue":"i"}]`,
		"leaves the tree": `[{"id":"a","journey":"j","test":"../../x.TestA","issue":"i"}]`,
		"not a Test":      `[{"id":"a","journey":"j","test":"p.helper","issue":"i"}]`,
		"unknown fields":  `[{"id":"a","journey":"j","test":"p.TestA","issue":"i","extra":1}]`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if rows, err := loadJSON(t, content); err == nil {
				t.Errorf("rows = %+v, want a refusal", rows)
			}
		})
	}
}

func TestLoad_ReportsAMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want not-exist", err)
	}
}

func TestRemoved_NamesEachRowThatLeftWithoutItsRetiredReason(t *testing.T) {
	active := Row{ID: "a", Journey: "j", Test: "p.TestA", Issue: "i"}
	retired := Row{ID: "b", Journey: "j", Test: "p.TestB", Issue: "i", Retired: "gone"}
	other := Row{ID: "c", Journey: "j", Test: "p.TestC", Issue: "i"}
	revived := retired
	revived.Retired = ""
	tests := []struct {
		name          string
		before, after []Row
		want          []string
	}{
		{"unchanged", []Row{active, retired}, []Row{active, retired}, nil},
		{"row deleted", []Row{active, retired}, []Row{retired}, []string{"a"}},
		{"retired row deleted", []Row{active, retired}, []Row{active}, []string{"b"}},
		{"retired reason cleared", []Row{retired}, []Row{revived}, []string{"b"}},
		{"row retired with a reason", []Row{active}, []Row{{ID: "a", Journey: "j", Test: "p.TestA", Issue: "i", Retired: "why"}}, nil},
		{"row added", []Row{active}, []Row{active, other}, nil},
		{"both deleted, sorted", []Row{other, active}, nil, []string{"a", "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Removed(tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Removed = %v, want %v", got, tc.want)
			}
		})
	}
}
