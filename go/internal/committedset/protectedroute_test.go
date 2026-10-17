package committedset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProtectedRoute_RoundTripsWhatWriteProtectedRouteStoresAndRejectsTheRest(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if _, ok := ReadProtectedRoute(ws); ok {
		t.Error("no artifact, no route")
	}
	if err := WriteProtectedRoute(ws, ProtectedRoute{Cycle: 7, ItemID: "x"}); err != nil {
		t.Fatal(err)
	}
	if got, ok := ReadProtectedRoute(ws); !ok || got.Cycle != 7 || got.ItemID != "x" {
		t.Errorf("got %+v ok=%v", got, ok)
	}
	if err := os.WriteFile(filepath.Join(ws, ProtectedRouteFile), []byte(`{"cycle":7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadProtectedRoute(ws); ok {
		t.Error("a route with no item is not a route")
	}
	if err := os.WriteFile(filepath.Join(ws, ProtectedRouteFile), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadProtectedRoute(ws); ok {
		t.Error("malformed JSON is not a route")
	}
}

func TestProtectedPredicatesDir_IsTheRunDirFolderTheCloseoutRescans(t *testing.T) {
	t.Parallel()
	const want = "tdd-protected-predicates"
	if ProtectedPredicatesDir != want {
		t.Fatalf("ProtectedPredicatesDir = %q, want %q: the host copy and the closeout re-scan must agree on it", ProtectedPredicatesDir, want)
	}
}
