package landing

import (
	"encoding"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func binding() dossier.ShipBinding {
	return dossier.ShipBinding{AuditBoundTreeSHA: "bound-tree", TreeSHACommitted: "committed-tree", CommitSHA: testHead, Cycle: 42}
}

func bindingFailureFixture(t *testing.T, l *Landing) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "runs")
	if err := os.WriteFile(dir, []byte("a file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteBinding(filepath.Join(dir, "cycle-42"), binding()); err == nil {
		t.Fatal("a file where the dir should be must fail the write")
	}
}

func TestWriteBinding_BytesMatchTheGoldenAndNoTempRemains(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "ship-binding.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	f := newFakeGit()
	l, got := newLanding(f)
	dir := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-42")
	if err := l.WriteBinding(dir, binding()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, dossier.ShipBindingFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(want) {
		t.Errorf("bytes\n got %q\nwant %q", body, want)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm()&0o700 != 0o700 {
		t.Errorf("dir %v %v", info, err)
	}
	if tmps, _ := filepath.Glob(filepath.Join(dir, "ship-binding.*.tmp")); len(tmps) != 0 {
		t.Errorf("temp files left behind: %v", tmps)
	}
	if len(*got) != 0 {
		t.Errorf("a green write emits nothing: %+v", *got)
	}
	empty := filepath.Join(t.TempDir(), "cycle-43")
	if err := l.WriteBinding(empty, dossier.ShipBinding{Cycle: 43}); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(empty, dossier.ShipBindingFile)); !strings.Contains(string(body), `"commit_sha": ""`) {
		t.Errorf("an empty commit_sha is still written (the delivery identity has no omitempty): %s", body)
	}
}

func TestWriteBinding_FailureWarnsBindingWriteFailedAndReturnsTheError(t *testing.T) {
	rows := map[string]func(t *testing.T) string{
		"file at dir": func(t *testing.T) string {
			dir := filepath.Join(t.TempDir(), "cycle-42")
			if err := os.WriteFile(dir, []byte("a file\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			return dir
		},
		"read-only dir": func(t *testing.T) string {
			dir := filepath.Join(t.TempDir(), "cycle-42")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			if probe, err := os.CreateTemp(dir, "probe"); err == nil {
				_ = probe.Close()
				t.Fatal("the read-only fixture must refuse writes (the test runs as root?)")
			}
			return dir
		},
		"directory at target": func(t *testing.T) string {
			dir := filepath.Join(t.TempDir(), "cycle-42")
			if err := os.MkdirAll(filepath.Join(dir, dossier.ShipBindingFile), 0o755); err != nil {
				t.Fatal(err)
			}
			return dir
		},
	}
	for name, fixture := range rows {
		t.Run(name, func(t *testing.T) {
			dir := fixture(t)
			f := newFakeGit()
			l, got := newLanding(f)
			err := l.WriteBinding(dir, binding())
			if err == nil {
				t.Fatal("the error is returned, never swallowed")
			}
			e := wantOneEvent(t, *got, CodeBindingWriteFailed, "Landing.WriteBinding",
				map[string]string{"step": "binding", "path": filepath.Join(dir, dossier.ShipBindingFile), "err": err.Error()})
			if e.Reason != "ship-binding.json write failed: "+err.Error() {
				t.Errorf("reason %q", e.Reason)
			}
		})
	}
}

func TestWriteBinding_TempRemovedWhenTheWriteFails(t *testing.T) {
	dir := t.TempDir()
	tmpPath := filepath.Join(dir, "ship-binding.ro.tmp")
	if err := os.WriteFile(tmpPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ro, err := os.Open(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, dossier.ShipBindingFile)
	err = commitTemp(ro, []byte("{}\n"), target)
	var pathErr *os.PathError
	if err == nil || !errors.As(err, &pathErr) || pathErr.Op != "write" {
		t.Fatalf("the WRITE error is returned (not a later rename's): %v", err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("the temp file is removed on failure: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("no rename on failure: %v", err)
	}
}

func TestWriteBinding_ShipBindingCannotFailToMarshal(t *testing.T) {
	assertMarshalCannotFail(t, reflect.TypeOf(dossier.ShipBinding{}), "dossier.ShipBinding")
}

var (
	jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

func assertMarshalCannotFail(t *testing.T, typ reflect.Type, path string) {
	t.Helper()
	for _, m := range []reflect.Type{jsonMarshalerType, textMarshalerType} {
		if typ.Implements(m) || reflect.PointerTo(typ).Implements(m) {
			t.Errorf("%s (%s) implements %s, which can return an error", path, typ, m)
		}
	}
	switch typ.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.String:
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.PkgPath != "" || f.Tag.Get("json") == "-" {
				continue
			}
			assertMarshalCannotFail(t, f.Type, path+"."+f.Name)
		}
	case reflect.Slice, reflect.Array:
		assertMarshalCannotFail(t, typ.Elem(), path+"[]")
	default:
		t.Errorf("%s is a %s: json.MarshalIndent can fail on it — re-split the MarshalIndent check out of CreateTemp's chain in binding.go", path, typ.Kind())
	}
}
