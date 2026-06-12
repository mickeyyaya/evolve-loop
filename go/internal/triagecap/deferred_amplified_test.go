package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAmplifiedReadDeferredFloorsRejectsMalformedCompanion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "triage-decision.json")
	if err := os.WriteFile(path, []byte(`{"deferred_floors":"gc"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	floors, ok, err := ReadDeferredFloors(path)
	if err == nil {
		t.Fatalf("ReadDeferredFloors must reject non-array deferred_floors; floors=%v ok=%v", floors, ok)
	}
	if ok {
		t.Fatalf("malformed deferred_floors must not be reported as present")
	}
}

func TestAmplifiedDeferredFloorPackagesDeclNormalizesDeclarations(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "triage-decision.json")
	if err := os.WriteFile(path, []byte(`{"deferred_floors":["web","unknown","gc","gc"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := DeferredFloorPackagesDecl("", path, []string{"gc", "phase", "web"})
	want := []string{"gc", "web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("declaration-primary deferred floors should be filtered, distinct, and stable-sorted; got %v want %v", got, want)
	}
}

func TestAmplifiedDeferredFloorPackagesDeclEmptyDeclarationSuppressesProse(t *testing.T) {
	t.Parallel()

	artifact := strings.Join([]string{
		"# Triage Report",
		"",
		"## Deferred",
		"- coverage-gc: push gc coverage to >=98%",
		"",
	}, "\n")
	if legacy := DeferredFloorPackages(artifact, []string{"gc"}); !reflect.DeepEqual(legacy, []string{"gc"}) {
		t.Fatalf("test fixture must be recognized by the legacy prose parser; got %v", legacy)
	}

	path := filepath.Join(t.TempDir(), "triage-decision.json")
	if err := os.WriteFile(path, []byte(`{"deferred_floors":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := DeferredFloorPackagesDecl(artifact, path, []string{"gc"}); len(got) != 0 {
		t.Fatalf("present empty deferred_floors declaration must be authoritative over prose; got %v", got)
	}
}
