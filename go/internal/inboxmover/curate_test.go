package inboxmover

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func readItemFields(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestEdit_RewritesACurationFieldThroughTheLeaf(t *testing.T) {
	root, inboxDir := seedInbox(t, "task-a")

	path, err := Edit(Options{ProjectRoot: root}, "task-a", []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.4"}, {Op: EditAdd, Field: "files", Value: "go/a.go"}})

	if err != nil || path != filepath.Join(inboxDir, "task-a.json") {
		t.Fatalf("Edit = (%q, %v)", path, err)
	}
	if item := readItemFields(t, path); item["weight"] != 0.4 || len(item["files"].([]any)) != 1 {
		t.Errorf("item = %v", item)
	}
	var remove EditOp = EditRemove
	if _, err := Edit(Options{ProjectRoot: root}, "task-a", []FieldEdit{{Op: remove, Field: "route", Value: "x"}}); !errors.Is(err, ErrBadArgs) {
		t.Errorf("a route edit: err = %v, want ErrBadArgs", err)
	}
}

func TestWithdraw_RemovesAnAsFiledItemAndRefusesARoutedOne(t *testing.T) {
	root, inboxDir := seedInbox(t, "task-a")
	opts := Options{ProjectRoot: root}
	if _, err := RouteConsole(opts, "task-a", "operator", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := Withdraw(opts, "task-a", "mistake"); !errors.Is(err, ErrNotWithdrawable) {
		t.Errorf("a routed item: err = %v, want ErrNotWithdrawable", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "task-b.json"), []byte(`{"id":"task-b"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := Withdraw(opts, "task-b", "mistake")

	if err != nil || path != filepath.Join(inboxDir, "task-b.json") {
		t.Fatalf("Withdraw = (%q, %v)", path, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("the item is still on disk: %v", statErr)
	}
}

func TestVerifyPremise_StampsTheShaOfOriginMain(t *testing.T) {
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	onOrigin := r.Git("rev-parse", "HEAD")
	r.Git("update-ref", "refs/remotes/origin/main", onOrigin)
	r.Git("commit", "-q", "--allow-empty", "-m", "local only")
	onMain := r.Git("rev-parse", "HEAD")
	repo := r.Dir
	inboxDir := filepath.Join(repo, ".evolve", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inboxDir, "task-a.json")
	if err := os.WriteFile(path, []byte(`{"id":"task-a"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := VerifyPremise(Options{ProjectRoot: repo}, "task-a", "reproduced at HEAD"); err != nil {
		t.Fatal(err)
	}

	if item := readItemFields(t, path); item["premise_verified_sha"] != onOrigin || item["premise_verified_sha"] == onMain {
		t.Errorf("item = %v, want the sha of origin/main (%s), not the local main (%s)", item, onOrigin, onMain)
	}
}

func TestVerifyPremise_WithoutOriginMainStampsNothing(t *testing.T) {
	root, inboxDir := seedInbox(t, "task-a")

	_, err := VerifyPremise(Options{ProjectRoot: root}, "task-a", "reproduced")

	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want the unresolvable origin/main reported", err)
	}
	if item := readItemFields(t, filepath.Join(inboxDir, "task-a.json")); len(item) != 1 {
		t.Errorf("item = %v, want it unstamped", item)
	}
	stamped, err := VerifyPremise(Options{ProjectRoot: root, MainHeadFn: func() (string, error) { return "abc123", nil }}, "task-a", "reproduced")
	if err != nil || readItemFields(t, stamped)["premise_verified_sha"] != "abc123" {
		t.Errorf("an injected main head: (%q, %v), want its sha stamped", stamped, err)
	}
}

func TestWithdraw_RefusesAnItemAContinuationBinds(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, "2026-09-30T00-00-00Z-x.json")
	item := `{"id":"x","title":"t","kind":"bug","summary":"s","fix":"f","weight":0.5,"acceptance":["a"]}`
	if err := os.WriteFile(path, []byte(item), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(root, "x", continuation.Continuation{SnapshotSHA: "0123456789abcdef0123", Branch: "lane/x", Cycle: 7}); err != nil {
		t.Fatal(err)
	}

	_, err := Withdraw(Options{ProjectRoot: root}, "x", "mistaken filing")

	if !errors.Is(err, ErrNotWithdrawable) || !strings.Contains(err.Error(), "evolve continuation release x") {
		t.Errorf("err = %v, want ErrNotWithdrawable naming the release", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the bound item is gone: %v", statErr)
	}
	if c, bound, rerr := continuation.ReadRegistryEntry(root, "x"); !bound || rerr != nil || c.SnapshotSHA != "0123456789abcdef0123" {
		t.Errorf("the binding = (%+v, %v, %v), want it kept", c, bound, rerr)
	}
}

func TestVerifyPremise_EvidenceNamingAProtectedFileNeverRoutesTheItem(t *testing.T) {
	root, inboxDir := seedInbox(t, "task-a")
	isProtected := func(p string) bool { return p == "go/internal/guards/phase.go" }
	opts := Options{ProjectRoot: root, IsProtectedPath: isProtected, MainHeadFn: func() (string, error) { return "abc123", nil }}

	path, err := VerifyPremise(opts, "task-a", "go/internal/guards/phase.go:12 still reads the old field")

	if err != nil || path != filepath.Join(inboxDir, "task-a.json") {
		t.Fatalf("VerifyPremise = (%q, %v)", path, err)
	}
	item, _, err := inboxbatch.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if routed, why := inboxbatch.ConsoleRouted(item, isProtected); routed {
		t.Errorf("ConsoleRouted = (true, %q): a premise stamp must not route the item", why)
	}
}
