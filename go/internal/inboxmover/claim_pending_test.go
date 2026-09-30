package inboxmover

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestClaimPending_ClaimsOnlyWhatIsPendingAtTheRoot(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	place := func(dir, id, extra string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`"`+extra+`}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	place(inbox, "pending", "")
	place(inbox, "operator-owned", `,"route":"console-only"`)
	place(filepath.Join(inbox, "processing", "cycle-7"), "mine", "")
	place(filepath.Join(inbox, "processing", "cycle-9"), "sibling", "")

	c := signalcenter.New()
	var codes []signalcenter.Code
	c.Subscribe(func(e signalcenter.Event) { codes = append(codes, e.Code) })
	opts := Options{ProjectRoot: root, Stderr: io.Discard, Signals: c}

	if err := ClaimPending(opts, 7, []string{"pending", "pending", "mine", "sibling", "scout-originated", "operator-owned"}); err != nil {
		t.Fatalf("ClaimPending: %v", err)
	}
	for id, dir := range map[string]string{
		"pending":        filepath.Join(inbox, "processing", "cycle-7"),
		"mine":           filepath.Join(inbox, "processing", "cycle-7"),
		"sibling":        filepath.Join(inbox, "processing", "cycle-9"),
		"operator-owned": inbox,
	} {
		if _, err := os.Stat(filepath.Join(dir, id+".json")); err != nil {
			t.Errorf("%s must sit in %s: %v", id, dir, err)
		}
	}
	if want := []signalcenter.Code{"INBOX_CLAIM_REFUSED"}; !reflect.DeepEqual(codes, want) {
		t.Errorf("signals = %v, want %v (the refusal only — no false INBOX_CLAIM_NOT_FOUND for the ids the gate judges)", codes, want)
	}
}

func TestClaimPending_AReadFaultIsReported(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(filepath.Dir(inbox), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("a file where the inbox dir should be"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ClaimPending(Options{ProjectRoot: root, Stderr: io.Discard}, 7, []string{"x"}); err == nil {
		t.Fatal("an inbox that cannot be read must be reported, not read as 'nothing pending'")
	}
}

func TestClaimPending_AFailedMoveIsReported(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "a.json"), []byte(`{"id":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "processing"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ClaimPending(Options{ProjectRoot: root, Stderr: io.Discard}, 7, []string{"a"})
	if !errors.Is(err, ErrMvFailed) {
		t.Fatalf("a move that failed must be returned, got %v", err)
	}
}

func TestClaimDispatchable_RefusesAnItemWaitingOnAnUnlandedDependency(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	place := func(dir, id, extra string) string {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, id+".json")
		if err := os.WriteFile(p, []byte(`{"id":"`+id+`"`+extra+`}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	place(filepath.Join(inbox, "retry"), "dep", "")
	waiting := place(inbox, "waiting", `,"deps":["dep"]`)
	place(filepath.Join(inbox, "processed", "cycle-3"), "landed", "")
	place(inbox, "ready", `,"deps":["landed"]`)
	var stderr strings.Builder
	opts := Options{ProjectRoot: root, Stderr: &stderr}

	_, err := ClaimDispatchable(opts, "waiting", "7")
	if !errors.Is(err, ErrWaitingOnDependency) {
		t.Fatalf("ClaimDispatchable(waiting) err = %v, want ErrWaitingOnDependency", err)
	}
	if !strings.Contains(err.Error(), "dep") || !strings.Contains(stderr.String(), "needs dep") {
		t.Errorf("the refusal must name the blocking dependency: err=%v stderr=%q", err, stderr.String())
	}
	if _, serr := os.Stat(waiting); serr != nil {
		t.Errorf("a refused claim must leave the item at the root: %v", serr)
	}

	if _, err := ClaimDispatchable(opts, "ready", "7"); err != nil {
		t.Fatalf("ClaimDispatchable(ready) with a landed dependency: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "processing", "cycle-7", "ready.json")); err != nil {
		t.Errorf("a dispatchable item must be claimed: %v", err)
	}
}
