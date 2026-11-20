package testaudit

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	okFakes  = `{"rows":[{"tool":"git","suite_dir":"pkg/x","real_side":"git"}]}`
	okGuards = `{"max_rows":2,"rows":[{"package":"pkg/x","test":"TestA","reason":"r"}]}`
)

func TestParseConfig_ReadsBothFilesAndValidates(t *testing.T) {
	t.Parallel()
	got, err := parseConfig([]byte(okFakes), []byte(okGuards))
	if err != nil {
		t.Fatalf("parseConfig = %v", err)
	}
	want := Config{
		Guards:       []Guard{{Package: "pkg/x", Test: "TestA", Reason: "r"}},
		Fakes:        []Fake{{Tool: "git", SuiteDir: "pkg/x", RealSide: "git"}},
		MaxGuardRows: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseConfig = %+v, want %+v", got, want)
	}
	for name, in := range map[string][2]string{
		"bad fakes":   {"{", okGuards},
		"bad guards":  {okFakes, "{"},
		"max_rows 0":  {okFakes, `{"max_rows":0,"rows":[]}`},
		"invalid row": {`{"rows":[{"tool":""}]}`, okGuards},
	} {
		if _, err := parseConfig([]byte(in[0]), []byte(in[1])); err == nil {
			t.Errorf("parseConfig(%s) = nil, want an error", name)
		}
	}
}

func TestLoadConfig_ReadsTheConfigOfTheGivenModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "internal", "testaudit", "fakes.json"), okFakes)
	mustWrite(t, filepath.Join(root, "internal", "testaudit", "guards.json"), okGuards)
	got, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if len(got.Fakes) != 1 || got.Fakes[0].Tool != "git" || got.MaxGuardRows != 2 {
		t.Errorf("LoadConfig = %+v, want the fixture config", got)
	}
}

func TestLoadConfig_ReportsAMissingOrBadFile(t *testing.T) {
	t.Parallel()
	if _, err := LoadConfig(t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadConfig(empty) = %v, want ErrNotExist", err)
	}
	onlyFakes := t.TempDir()
	mustWrite(t, filepath.Join(onlyFakes, "internal", "testaudit", "fakes.json"), okFakes)
	if _, err := LoadConfig(onlyFakes); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadConfig(no guards) = %v, want ErrNotExist", err)
	}
	bad := t.TempDir()
	mustWrite(t, filepath.Join(bad, "internal", "testaudit", "fakes.json"), "{")
	mustWrite(t, filepath.Join(bad, "internal", "testaudit", "guards.json"), okGuards)
	if _, err := LoadConfig(bad); err == nil {
		t.Error("LoadConfig accepted a malformed fakes file")
	}
}

func TestLoadConfig_EqualsTheEmbeddedConfigOnTheRealModule(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, embedded) {
		t.Errorf("LoadConfig(module) differs from DefaultConfig:\n%+v\n%+v", loaded, embedded)
	}
}
