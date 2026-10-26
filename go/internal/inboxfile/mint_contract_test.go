package inboxfile_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func tick(second int) func() time.Time {
	return func() time.Time { return time.Date(2026, 10, 27, 9, 0, second, 0, time.UTC) }
}

func itemJSON(t *testing.T, override map[string]any) []byte {
	t.Helper()
	doc := map[string]any{
		"id":             "minted-defect-one",
		"title":          "Fix the minted defect",
		"kind":           "fix",
		"summary":        "A minter found a defect.",
		"fix":            "Repair the defect.",
		"acceptance":     []string{"the defect no longer reproduces"},
		"weight":         0.5,
		"priority_class": "correctness",
	}
	for k, v := range override {
		if v == nil {
			delete(doc, k)
			continue
		}
		doc[k] = v
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func jsonFiles(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestMint_FilesAStampedItemUnderTheInbox(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")

	res, err := inboxfile.Mint(inbox, itemJSON(t, nil), tick(7))

	if err != nil || res.Skipped {
		t.Fatalf("Mint = %+v, %v; want a filed item", res, err)
	}
	if want := "2026-10-27T09-00-07Z-minted-defect-one.json"; filepath.Base(res.Path) != want {
		t.Errorf("filed as %q, want %q", filepath.Base(res.Path), want)
	}
	if names := jsonFiles(t, inbox); len(names) != 1 {
		t.Errorf("inbox holds %v, want one item", names)
	}
}

func TestMint_ARepeatedIdIsASkipNotAFailure(t *testing.T) {
	inbox := t.TempDir()
	if _, err := inboxfile.Mint(inbox, itemJSON(t, nil), tick(1)); err != nil {
		t.Fatal(err)
	}

	res, err := inboxfile.Mint(inbox, itemJSON(t, nil), tick(2))

	if err != nil {
		t.Fatalf("a repeated filed id must not fail: %v", err)
	}
	if !res.Skipped || res.Path != "" {
		t.Errorf("Mint = %+v, want Skipped with no path", res)
	}
	if names := jsonFiles(t, inbox); len(names) != 1 {
		t.Errorf("inbox holds %v, want one item", names)
	}
}

func TestMint_AnIdParkedInASubfolderIsStillFiled(t *testing.T) {
	inbox := t.TempDir()
	if err := os.MkdirAll(filepath.Join(inbox, "retry"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "retry", "2020-01-01T00-00-00Z-minted-defect-one.json"), itemJSON(t, nil), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := inboxfile.Mint(inbox, itemJSON(t, nil), tick(1))

	if err != nil || !res.Skipped {
		t.Fatalf("Mint = %+v, %v; want a skip for an id parked in retry/", res, err)
	}
	if names := jsonFiles(t, inbox); len(names) != 0 {
		t.Errorf("top level holds %v, want nothing", names)
	}
}

func TestMint_ConcurrentMintsOfOneIdFileExactlyOneItem(t *testing.T) {
	const mints = 8
	inbox := t.TempDir()
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]inboxfile.Result, mints)
	errs := make([]error, mints)
	for i := 0; i < mints; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = inboxfile.Mint(inbox, itemJSON(t, nil), tick(i))
		}()
	}
	close(start)
	wg.Wait()

	filed := 0
	for i := range results {
		if errs[i] != nil {
			t.Errorf("mint %d: %v", i, errs[i])
		}
		if !results[i].Skipped {
			filed++
		}
	}
	if filed != 1 {
		t.Errorf("%d mints filed the item, want exactly 1", filed)
	}
	if names := jsonFiles(t, inbox); len(names) != 1 {
		t.Errorf("inbox holds %v, want one item", names)
	}
}

func TestMint_RefusesAnInvalidItemAndFilesNothing(t *testing.T) {
	cases := map[string]map[string]any{
		"no fix":                 {"fix": nil},
		"blank summary":          {"summary": "  "},
		"no acceptance":          {"acceptance": nil},
		"blank acceptance entry": {"acceptance": []string{" "}},
		"id not kebab-case":      {"id": "Not Kebab"},
		"weight out of range":    {"weight": 1.5},
		"unknown priority class": {"priority_class": "no-such-class"},
		"lifecycle-owned field":  {"status": "claimed"},
	}
	for name, override := range cases {
		t.Run(name, func(t *testing.T) {
			inbox := t.TempDir()

			res, err := inboxfile.Mint(inbox, itemJSON(t, override), tick(1))

			if err == nil {
				t.Fatalf("Mint = %+v, want a refusal", res)
			}
			if names := jsonFiles(t, inbox); len(names) != 0 {
				t.Errorf("a refused item left %v in the inbox", names)
			}
		})
	}
}

func TestMint_RefusesNonObjectInput(t *testing.T) {
	for _, raw := range []string{"", "null", "[]", "not json"} {
		inbox := t.TempDir()
		if _, err := inboxfile.Mint(inbox, []byte(raw), tick(1)); err == nil {
			t.Errorf("Mint(%q) = nil error, want a refusal", raw)
		}
		if names := jsonFiles(t, inbox); len(names) != 0 {
			t.Errorf("Mint(%q) left %v in the inbox", raw, names)
		}
	}
}

func TestMint_WaitsForTheBumpLockBeforeFiling(t *testing.T) {
	inbox := t.TempDir()
	release, err := verifylock.AcquireAt(context.Background(), filepath.Join(inbox, ".bump.lock"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := inboxfile.Mint(inbox, itemJSON(t, nil), tick(1))
		done <- err
	}()

	time.Sleep(400 * time.Millisecond)
	if names := jsonFiles(t, inbox); len(names) != 0 {
		t.Errorf("Mint filed %v while the lock was held", names)
	}
	release()
	if err := <-done; err != nil {
		t.Fatalf("Mint after release: %v", err)
	}
	if names := jsonFiles(t, inbox); len(names) != 1 {
		t.Errorf("inbox holds %v, want one item", names)
	}
}

func TestMint_ANilClockStampsWithTheCurrentTime(t *testing.T) {
	inbox := t.TempDir()

	res, err := inboxfile.Mint(inbox, itemJSON(t, nil), nil)

	if err != nil || res.Skipped || res.Path == "" {
		t.Fatalf("Mint = %+v, %v; want a filed item", res, err)
	}
}

func TestMint_UnwritableInboxDirReturnsError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := inboxfile.Mint(filepath.Join(file, "sub"), itemJSON(t, nil), tick(1))
	if err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("Mint err = %v, want create error", err)
	}
}

func TestPublish_UnacquirableLockReturnsError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := inboxfile.Publish(filepath.Join(file, "sub"), itemJSON(t, nil), []string{"correctness"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("Publish err = %v, want lock error", err)
	}
}

func TestFile_UnreadableInboxOrWriteFaultReturnsError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := inboxfile.File(filepath.Join(file, "sub"), itemJSON(t, nil), []string{"correctness"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "scan") {
		t.Fatalf("File err = %v, want scan error", err)
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err = inboxfile.File(dir, itemJSON(t, nil), []string{"correctness"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("File err = %v, want write error", err)
	}
}

func TestFile_EmptyClassesReturnsError(t *testing.T) {
	inbox := t.TempDir()
	_, err := inboxfile.File(inbox, itemJSON(t, nil), nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "no priority class order is wired") {
		t.Fatalf("File err = %v, want errNoClassOrder", err)
	}
}

func TestMint_DecodeItemTypeError(t *testing.T) {
	inbox := t.TempDir()
	raw := []byte(`{"id":"item-one","title":"Title","kind":"fix","summary":"Sum","fix":"Fix","weight":"invalid"}`)
	_, err := inboxfile.Mint(inbox, raw, tick(1))
	if err == nil || !strings.Contains(err.Error(), "invalid inbox item") {
		t.Fatalf("Mint err = %v, want decode error", err)
	}
}

func TestCheckFieldRules_JudgedAndSanitizedFields(t *testing.T) {
	fields := map[string]json.RawMessage{
		"id":         json.RawMessage(`"not_kebab"`),
		"title":      json.RawMessage(`"Valid Title"`),
		"kind":       json.RawMessage(`"fix"`),
		"summary":    json.RawMessage(`"Valid summary"`),
		"fix":        json.RawMessage(`"Valid fix"`),
		"weight":     json.RawMessage(`0.5`),
		"acceptance": json.RawMessage(`["criterion"]`),
	}
	item := inboxbatch.Item{
		ID:         "not_kebab",
		Title:      "Valid Title",
		Kind:       "fix",
		Summary:    json.RawMessage(`"Valid summary"`),
		Weight:     0.5,
		Acceptance: []string{"criterion"},
	}

	err := inboxfile.CheckFieldRules(fields, item, func(field string) bool {
		return field != "id"
	})
	if err != nil {
		t.Fatalf("CheckFieldRules err = %v, want nil when id is not judged", err)
	}

	itemWithControl := item
	itemWithControl.ID = "valid-kebab"
	fields["id"] = json.RawMessage(`"valid-kebab"`)
	itemWithControl.Title = "bad\x01title"
	rawTitle, _ := json.Marshal("bad\x01title")
	fields["title"] = rawTitle

	err = inboxfile.CheckFieldRules(fields, itemWithControl, func(string) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "control characters") {
		t.Fatalf("CheckFieldRules err = %v, want control characters error", err)
	}

	err = inboxfile.CheckFieldRules(fields, itemWithControl, func(field string) bool {
		return field != "title"
	})
	if err != nil {
		t.Fatalf("CheckFieldRules err = %v, want nil when title is unjudged", err)
	}
}

func TestMint_AuthoredNonConsoleRouteIsRefused(t *testing.T) {
	inbox := t.TempDir()
	_, err := inboxfile.Mint(inbox, itemJSON(t, map[string]any{"route": "lane"}), tick(1))
	if err == nil || !strings.Contains(err.Error(), "authored route may only send the item to the console") {
		t.Fatalf("Mint err = %v, want route refusal", err)
	}
}

func TestReadID_InvalidOrMissingPath(t *testing.T) {
	if _, ok := inboxfile.ReadID(filepath.Join(t.TempDir(), "nonexistent.json")); ok {
		t.Fatal("ReadID of nonexistent file must return false")
	}

	badFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badFile, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := inboxfile.ReadID(badFile); ok {
		t.Fatal("ReadID of invalid json file must return false")
	}
}

func TestMint_DepsValidation(t *testing.T) {
	inbox := t.TempDir()
	_, err := inboxfile.Mint(inbox, itemJSON(t, map[string]any{"deps": []string{"unfiled-dep"}}), tick(1))
	if err == nil || !strings.Contains(err.Error(), "deps names no other inbox item") {
		t.Fatalf("Mint err = %v, want unfiled deps error", err)
	}

	res1, err := inboxfile.Mint(inbox, itemJSON(t, map[string]any{"id": "dep-item"}), tick(1))
	if err != nil || res1.Skipped {
		t.Fatalf("Mint dep-item = %+v, %v", res1, err)
	}
	res2, err := inboxfile.Mint(inbox, itemJSON(t, map[string]any{"id": "dependent-item", "deps": []string{"dep-item"}}), tick(2))
	if err != nil || res2.Skipped {
		t.Fatalf("Mint dependent-item = %+v, %v", res2, err)
	}
}

func TestPublishNewItem_EncodeWriteAndLinkErrors(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.json")
	err := inboxfile.PublishNewItem(target, map[string]json.RawMessage{"id": json.RawMessage("{")})
	if err == nil || !strings.Contains(err.Error(), "encode") {
		t.Fatalf("PublishNewItem err = %v, want encode error", err)
	}

	err = inboxfile.PublishNewItem(filepath.Join(t.TempDir(), "no-such-dir", "sub", "out.json"), map[string]json.RawMessage{"id": json.RawMessage(`"ok"`)})
	if err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("PublishNewItem err = %v, want write error", err)
	}

	occupied := filepath.Join(t.TempDir(), "occupied.json")
	if err := os.WriteFile(occupied, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = inboxfile.PublishNewItem(occupied, map[string]json.RawMessage{"id": json.RawMessage(`"ok"`)})
	if err == nil || !strings.Contains(err.Error(), "publish") {
		t.Fatalf("PublishNewItem err = %v, want publish/link error", err)
	}
}

func TestMint_AnIdOnlyInConsumedIsFiledAgainAsNewEvidence(t *testing.T) {
	inbox := t.TempDir()
	if err := os.MkdirAll(filepath.Join(inbox, "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "consumed", "2020-01-01T00-00-00Z-minted-defect-one.json"), itemJSON(t, nil), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := inboxfile.Mint(inbox, itemJSON(t, nil), tick(1))

	if err != nil || res.Skipped || res.Path == "" {
		t.Fatalf("Mint = %+v, %v; a consumed id must not silence a new red", res, err)
	}
	if names := jsonFiles(t, inbox); len(names) != 1 {
		t.Errorf("top level holds %v, want the new item", names)
	}
}

func TestMint_ADepOnAConsumedItemStillResolves(t *testing.T) {
	inbox := t.TempDir()
	if err := os.MkdirAll(filepath.Join(inbox, "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "consumed", "2020-01-01T00-00-00Z-done.json"), itemJSON(t, map[string]any{"id": "done-item"}), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := inboxfile.Mint(inbox, itemJSON(t, map[string]any{"deps": []string{"done-item"}}), tick(1))

	if err != nil || res.Path == "" {
		t.Fatalf("Mint = %+v, %v; a dep on a consumed item names a real item", res, err)
	}
}

func TestPendingIDs_LeavesOutConsumedAndFiledIDsKeepsThem(t *testing.T) {
	inbox := t.TempDir()
	if err := os.MkdirAll(filepath.Join(inbox, "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "consumed", "2020-01-01T00-00-00Z-done.json"), itemJSON(t, map[string]any{"id": "done-item"}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "2020-01-01T00-00-00Z-open.json"), itemJSON(t, map[string]any{"id": "open-item"}), 0o644); err != nil {
		t.Fatal(err)
	}

	pending, err := inboxfile.PendingIDs(inbox)
	if err != nil || !pending["open-item"] || pending["done-item"] {
		t.Fatalf("PendingIDs = %v, %v; want the open item only", pending, err)
	}
	filed, err := inboxfile.FiledIDs(inbox)
	if err != nil || !filed["open-item"] || !filed["done-item"] {
		t.Fatalf("FiledIDs = %v, %v; want both items", filed, err)
	}
}
