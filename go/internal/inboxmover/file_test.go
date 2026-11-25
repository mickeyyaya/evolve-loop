package inboxmover

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFile_FilesAnItemTheClaimFloorCanHandToALane(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr, PriorityClasses: []string{"stability", "feature"}, Origin: "console"}
	item := `{"id":"cli-x","kind":"feature","priority_class":"feature","weight":0.4,"title":"t","summary":"s","fix":"f","acceptance":["a"]}`

	res, err := File(opts, []byte(item))

	if err != nil || filepath.Dir(res.Path) != filepath.Join(root, ".evolve", "inbox") {
		t.Fatalf("File = (%+v, %v)", res, err)
	}
	var _ FileResult = res
	if _, err := Claim(opts, "cli-x", "1790"); err != nil {
		t.Errorf("Claim of the filed item: %v", err)
	}
	if _, err := File(opts, []byte(item)); !errors.Is(err, ErrInvalidItem) || !errors.Is(err, ErrAlreadyFiled) {
		t.Errorf("filing the same id again: err = %v, want ErrInvalidItem and ErrAlreadyFiled", err)
	}
}

func TestFile_JudgesThePriorityClassAgainstTheOptionsOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr, PriorityClasses: []string{"stability"}}
	item := `{"id":"cli-y","kind":"feature","priority_class":"feature","weight":0.4,"title":"t","summary":"s","fix":"f","acceptance":["a"]}`

	_, err := File(opts, []byte(item))

	if !errors.Is(err, ErrInvalidItem) {
		t.Errorf("a class outside Options.PriorityClasses: err = %v, want ErrInvalidItem", err)
	}
}

func TestFile_StampsTheSourceOfTheOptions(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: os.Stderr, PriorityClasses: []string{"feature"}, Origin: "cycle", Source: "request:3/inbox-add-1"}
	item := `{"id":"cli-z","kind":"feature","priority_class":"feature","weight":0.4,"title":"t","summary":"s","fix":"f","acceptance":["a"]}`

	res, err := File(opts, []byte(item))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["source"] != "request:3/inbox-add-1" || doc["origin"] != "cycle" {
		t.Errorf("filed = %v, want the source and origin of the options", doc)
	}
}
