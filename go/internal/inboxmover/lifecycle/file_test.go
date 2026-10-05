package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const validItem = `{"id":"cli-inbox-show","kind":"feature","priority_class":"debuggability","weight":0.5,` +
	`"title":"evolve inbox show prints one item","files":["go/cmd/evolve/cmd_inbox.go"],` +
	`"summary":"No verb prints one item.","fix":"Add evolve inbox show <id>.",` +
	`"acceptance":["evolve inbox show <id> prints the item (red: no such subcommand)"],"source":"console"}`

func filingClock() time.Time { return time.Date(2026, 9, 30, 9, 30, 0, 0, time.UTC) }

func jsonFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") {
			out = append(out, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMover_File_WritesAnItemTheLoaderReadsCleanly(t *testing.T) {
	inbox := newInbox(t)
	rec := &recordingAppender{}
	m := New(inbox, rec, WithNow(filingClock))

	res, err := m.File([]byte(validItem))

	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if want := filepath.Join(inbox, "2026-09-30T09-30-00Z-cli-inbox-show.json"); res.Path != want {
		t.Errorf("Path = %q, want %q", res.Path, want)
	}
	item, warnings, err := inboxbatch.LoadFile(res.Path)
	if err != nil || len(warnings) != 0 || item.ID != "cli-inbox-show" || item.CreatedAt != "2026-09-30T09:30:00Z" {
		t.Errorf("LoadFile = (%+v, %v, %v)", item, warnings, err)
	}
	doc := readItem(t, res.Path)
	if doc["summary"] != "No verb prints one item." || doc["source"] != "console" {
		t.Errorf("the item's other fields must survive: %v", doc)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "file" || rec.records[0].TaskID != "cli-inbox-show" {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMover_File_KeepsAnAuthoredCreatedAt(t *testing.T) {
	inbox := newInbox(t)
	body := strings.Replace(validItem, `"source":"console"}`, `"source":"console","created_at":"2026-09-29"}`, 1)

	res, err := New(inbox, nil, WithNow(filingClock)).File([]byte(body))

	if err != nil {
		t.Fatal(err)
	}
	if doc := readItem(t, res.Path); doc["created_at"] != "2026-09-29" {
		t.Errorf("created_at = %v, want the authored value", doc["created_at"])
	}
}

func TestMover_File_RefusesAnInvalidItem(t *testing.T) {
	for name, tc := range map[string]struct{ body, why string }{
		"not an object":            {`["x"]`, "JSON object"},
		"no id":                    {strings.Replace(validItem, `"id":"cli-inbox-show",`, ``, 1), "id"},
		"an id that is not kebab":  {strings.Replace(validItem, `"cli-inbox-show"`, `"Cli_Inbox"`, 1), "id"},
		"no title":                 {strings.Replace(validItem, `"title":"evolve inbox show prints one item",`, ``, 1), "title"},
		"a blank summary":          {strings.Replace(validItem, `"No verb prints one item."`, `"  "`, 1), "summary"},
		"no fix":                   {strings.Replace(validItem, `"fix":"Add evolve inbox show <id>.",`, ``, 1), "fix"},
		"no kind":                  {strings.Replace(validItem, `"kind":"feature",`, ``, 1), "kind"},
		"a zero weight":            {strings.Replace(validItem, `"weight":0.5`, `"weight":0`, 1), "weight"},
		"a weight above one":       {strings.Replace(validItem, `"weight":0.5`, `"weight":1.5`, 1), "weight"},
		"no acceptance":            {strings.Replace(validItem, `"acceptance":["evolve inbox show <id> prints the item (red: no such subcommand)"],`, ``, 1), "acceptance"},
		"a blank acceptance entry": {strings.Replace(validItem, `"evolve inbox show <id> prints the item (red: no such subcommand)"`, `" "`, 1), "acceptance"},
		"a lifecycle field":        {strings.Replace(validItem, `"source":"console"}`, `"source":"console","route":"lane"}`, 1), "route"},
		"a mistyped field":         {strings.Replace(validItem, `"files":["go/cmd/evolve/cmd_inbox.go"]`, `"files":"go/cmd/evolve/cmd_inbox.go"`, 1), "files"},
		"a control character":      {strings.Replace(validItem, `prints one item`, `prints\u0007one item`, 1), "title"},
		"an overlong title":        {strings.Replace(validItem, `evolve inbox show prints one item`, strings.Repeat("x", 161), 1), "title"},
	} {
		t.Run(name, func(t *testing.T) {
			inbox := newInbox(t)
			rec := &recordingAppender{}

			_, err := New(inbox, rec, WithNow(filingClock)).File([]byte(tc.body))

			if !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), tc.why) {
				t.Errorf("err = %v, want ErrInvalidItem naming %q", err, tc.why)
			}
			if files := jsonFiles(t, inbox); len(files) != 0 || len(rec.records) != 0 {
				t.Errorf("a refused item left files %v or ledger %+v", files, rec.records)
			}
		})
	}
}

func TestMover_File_RefusesAnIDTheInboxAlreadyHolds(t *testing.T) {
	for _, where := range []string{
		"2026-09-01T00-00-00Z-cli-inbox-show.json",
		filepath.Join("processing", "cycle-3", "cli-inbox-show.json"),
		filepath.Join("consumed", "2026-09-01T00-00-00Z-cli-inbox-show.json"),
		filepath.Join("processed", "cycle-9", "abc12345-cli-inbox-show.json"),
	} {
		t.Run(where, func(t *testing.T) {
			inbox := newInbox(t)
			writeItem(t, filepath.Join(inbox, where), `{"id":"cli-inbox-show"}`)

			_, err := New(inbox, nil, WithNow(filingClock)).File([]byte(validItem))

			if !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "already") {
				t.Errorf("err = %v, want ErrInvalidItem: the id is already filed", err)
			}
		})
	}
}

func TestMover_File_ADependencyMustNameAnItem(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "consumed", "done.json"), `{"id":"inbox-add-cli"}`)
	withDeps := func(deps string) []byte {
		return []byte(strings.Replace(validItem, `"source":"console"}`, `"source":"console","deps":`+deps+`}`, 1))
	}
	m := New(inbox, nil, WithNow(filingClock))

	if _, err := m.File(withDeps(`["no-such-item"]`)); !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "no-such-item") {
		t.Errorf("unknown dependency: err = %v, want ErrInvalidItem naming it", err)
	}
	if _, err := m.File(withDeps(`["cli-inbox-show"]`)); !errors.Is(err, ErrInvalidItem) {
		t.Errorf("self dependency: err = %v, want ErrInvalidItem", err)
	}
	if _, err := m.File(withDeps(`["inbox-add-cli"]`)); err != nil {
		t.Errorf("a dependency on a retired item is satisfied: %v", err)
	}
}

func TestMover_File_AWriteFaultIsNotAnInvalidItem(t *testing.T) {
	inbox := newInbox(t)
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	rec := &recordingAppender{}

	_, err := New(inbox, rec, WithNow(filingClock)).File([]byte(validItem))

	if err == nil || errors.Is(err, ErrInvalidItem) || len(rec.records) != 0 {
		t.Errorf("err = %v ledger = %+v, want the write fault and no ledger line", err, rec.records)
	}
}

func TestMover_File_AnUnreadableInboxIsAScanFault(t *testing.T) {
	inbox := newInbox(t)
	if err := os.Chmod(inbox, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, nil, WithNow(filingClock)).File([]byte(validItem))

	if err == nil || errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "scan") {
		t.Errorf("err = %v, want the scan fault itself", err)
	}
}

func TestMover_File_NeverClobbersAFileAtTheDestination(t *testing.T) {
	inbox := newInbox(t)
	occupied := filepath.Join(inbox, "2026-09-30T09-30-00Z-cli-inbox-show.json")
	writeItem(t, occupied, `{"id":"a-different-item"}`)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, WithNow(filingClock)).File([]byte(validItem))

	if err == nil || errors.Is(err, ErrInvalidItem) || len(rec.records) != 0 {
		t.Errorf("err = %v ledger = %+v, want a publish fault and no ledger line", err, rec.records)
	}
	if doc := readItem(t, occupied); doc["id"] != "a-different-item" {
		t.Errorf("the file at the destination was overwritten: %v", doc)
	}
}

func TestPublishNewItem_AnUnencodableFieldIsAFault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.json")

	err := publishNewItem(path, map[string]json.RawMessage{"id": json.RawMessage(`{`)})

	if err == nil {
		t.Fatal("an invalid raw field must not be published")
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a failed encode left %s behind", path)
	}
}

func TestMover_File_AcceptsTheBoundaryWeightAndKeepsTextReadable(t *testing.T) {
	inbox := newInbox(t)

	res, err := New(inbox, nil, WithNow(filingClock)).File([]byte(strings.Replace(validItem, `"weight":0.5`, `"weight":1`, 1)))

	if err != nil {
		t.Fatalf("a weight of exactly 1 is in range: %v", err)
	}
	if raw, _ := os.ReadFile(res.Path); !strings.Contains(string(raw), "evolve inbox show <id>") {
		t.Errorf("the filed text must stay readable, not HTML-escaped:\n%s", raw)
	}
}

func TestMover_File_AnAuthoredConsoleRouteIsKeptAndReported(t *testing.T) {
	inbox := newInbox(t)
	body := strings.Replace(validItem, `"source":"console"}`, `"source":"console","route":"console-manual"}`, 1)

	res, err := New(inbox, nil, WithNow(filingClock)).File([]byte(body))

	if err != nil || readItem(t, res.Path)["route"] != "console-manual" || !strings.Contains(res.ConsoleReason, "console-manual") {
		t.Errorf("File = (%+v, %v), want the console route kept and reported", res, err)
	}
}

func TestMover_File_ReportsWhatTheClaimFloorWouldRefuse(t *testing.T) {
	inbox := newInbox(t)
	body := strings.Replace(validItem, `"files":["go/cmd/evolve/cmd_inbox.go"]`, `"files":["go/internal/guards/phase.go"]`, 1)
	isGuard := func(p string) bool { return p == "go/internal/guards/phase.go" }

	res, err := New(inbox, nil, WithNow(filingClock), WithProtectedPath(isGuard)).File([]byte(body))

	if err != nil || !strings.Contains(res.ConsoleReason, "protected fix surface") {
		t.Errorf("File = (%+v, %v), want the claim floor's own refusal reported", res, err)
	}
	if res, err := New(newInbox(t), nil, WithNow(filingClock)).File([]byte(validItem)); err != nil || res.ConsoleReason != "" {
		t.Errorf("a lane item: File = (%+v, %v), want no console reason", res, err)
	}
}

func TestMover_File_RefusesEveryLifecycleOwnedField(t *testing.T) {
	for _, field := range []string{`"route":"lane"`, `"routed_reason":"x"`, `"routed_cycle":3`, `"retired_cycle":3`,
		`"last_failure_reason":"x"`, `"failure_count":1`, `"consumed":{}`, `"git_sha":"abc"`, `"unbacked":true`,
		`"continuation":{}`, `"released_continuations":[]`, `"premise_verified_at":"2026-09-30T00:00:00Z"`,
		`"premise_verified_sha":"abc"`, `"premise_verified_evidence":"x"`} {
		t.Run(field, func(t *testing.T) {
			body := strings.Replace(validItem, `"source":"console"}`, `"source":"console",`+field+`}`, 1)

			_, err := New(newInbox(t), nil, WithNow(filingClock)).File([]byte(body))

			if !errors.Is(err, ErrInvalidItem) {
				t.Errorf("err = %v, want ErrInvalidItem", err)
			}
		})
	}
}

func TestMover_File_LeavesOnlyTheFiledItemBehind(t *testing.T) {
	inbox := newInbox(t)

	res, err := New(inbox, nil, WithNow(filingClock)).File([]byte(validItem))

	if err != nil {
		t.Fatal(err)
	}
	if files := jsonFilesAndTemps(t, inbox); len(files) != 1 || files[0] != res.Path {
		t.Errorf("inbox holds %v, want only %s: a filing must not leave its temp file", files, res.Path)
	}
}

func jsonFilesAndTemps(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out
}
