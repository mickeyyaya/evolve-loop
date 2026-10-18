package sizeratchet

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fileWith(name string, lines int) string {
	return "package p\n\nfunc " + name + "() {\n" + body(lines) + "}\n"
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTighten_LowersEachAllowanceToItsFunctionAndDropsHealedOrGoneOnes(t *testing.T) {
	spans := []FuncSpan{{Key: "p.Shrunk", Lines: 55}, {Key: "p.Exact", Lines: 60}, {Key: "p.Healed", Lines: MaxLines}, {Key: "p.Twin", Lines: 52}, {Key: "p.Twin", Lines: 57}, {Key: "p.Over", Lines: 70}}
	offenders := map[string]int{"p.Shrunk": 60, "p.Exact": 60, "p.Healed": 60, "p.Gone": 60, "p.Twin": 60, "p.Over": 60}
	given := maps.Clone(offenders)

	got := Tighten(spans, offenders)

	want := map[string]int{"p.Shrunk": 55, "p.Exact": 60, "p.Twin": 57, "p.Over": 60}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tighten = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(offenders, given) {
		t.Errorf("Tighten mutated its input: %v", offenders)
	}
}

func TestEncode_ReproducesTheCheckedInListByteForByte(t *testing.T) {
	path := filepath.Join(moduleRoot(t), filepath.FromSlash(OffendersRelPath))
	offenders, err := LoadOffenders(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Encode(offenders)

	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("Encode(%s) err=%v differs from the file; the boundary tighten must be byte-stable", path, err)
	}
	if empty, err := Encode(map[string]int{}); err != nil || string(empty) != "{}\n" {
		t.Errorf("Encode(empty) = %q, %v", empty, err)
	}
}

func TestEncode_CarriesTheEncodersError(t *testing.T) {
	defer func(m func(any, string, string) ([]byte, error)) { marshalIndent = m }(marshalIndent)
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encoder down") }

	if _, err := Encode(map[string]int{"p.F": 60}); err == nil || !strings.Contains(err.Error(), "encoder down") {
		t.Errorf("Encode err = %v, want the encoder's error", err)
	}
}

func TestChanges_NamesEachRemovedOrLoweredAllowanceInKeyOrder(t *testing.T) {
	before := map[string]int{"q.G": 60, "p.F": 60, "p.Same": 60}
	after := map[string]int{"p.F": 55, "p.Same": 60}

	got := Changes(before, after)

	want := []string{"p.F: 60 -> 55", "q.G: 60 removed"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Changes = %v, want %v", got, want)
	}
	if tight := Changes(after, after); tight != nil {
		t.Errorf("Changes of a tight list = %v, want nil", tight)
	}
}

func TestTightenModule_WalksLoadsTightensAndEncodesFromOneRoot(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"p/p.go": fileWith("F", 55), "q/q.go": fileWith("G", MaxLines), OffendersRelPath: `{"p.F": 60, "q.G": 60, "r.Gone": 60}`})

	changes, encoded, err := TightenModule(root)

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"p.F: 60 -> 55", "q.G: 60 removed", "r.Gone: 60 removed"}; !reflect.DeepEqual(changes, want) {
		t.Errorf("changes = %v, want %v", changes, want)
	}
	if want := "{\n  \"p.F\": 55\n}\n"; string(encoded) != want {
		t.Errorf("encoded = %q, want %q", encoded, want)
	}
}

func TestTightenModule_CarriesTheWalkListAndEncoderFailures(t *testing.T) {
	if _, _, err := TightenModule(filepath.Join(t.TempDir(), "absent")); err == nil || !strings.Contains(err.Error(), "sizeratchet: walk") {
		t.Errorf("absent root: %v", err)
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{"p/p.go": fileWith("F", 55)})
	if _, _, err := TightenModule(root); err == nil || !strings.Contains(err.Error(), "read offender list") {
		t.Errorf("absent list: %v", err)
	}
	writeTree(t, root, map[string]string{OffendersRelPath: `{"p.F": 60}`})
	defer func(m func(any, string, string) ([]byte, error)) { marshalIndent = m }(marshalIndent)
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encoder down") }
	if _, _, err := TightenModule(root); err == nil || !strings.Contains(err.Error(), "encoder down") {
		t.Errorf("encoder failure: %v", err)
	}
}
