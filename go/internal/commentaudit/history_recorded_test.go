package commentaudit_test

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

const recordedBefore = "package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc F() {}\n\n// G is the other.\n// It used to live in cycle 7's queue.\nfunc G() {}\n"

const recordedAfter = "package x\n\nfunc F() {}\n\nfunc G() {}\n"

func recordedSides(page string) (before, after func(string) ([]byte, error)) {
	before = func(p string) ([]byte, error) {
		if p == "x.go" {
			return []byte(recordedBefore), nil
		}
		return nil, fs.ErrNotExist
	}
	after = func(p string) ([]byte, error) {
		switch p {
		case "x.go":
			return []byte(recordedAfter), nil
		case commentaudit.HistoryArchiveDir + "/root.md":
			if page != "" {
				return []byte(page), nil
			}
		}
		return nil, fs.ErrNotExist
	}
	return before, after
}

func TestUnrecordedRemovals_EqualsTheTwoRuleCallsComposed(t *testing.T) {
	t.Parallel()
	before, after := recordedSides("")
	removed, err := commentaudit.RemovedHistoryAcrossDiff([]string{"x.go"}, before, after)
	if err != nil || len(removed) != 2 {
		t.Fatalf("fixture must remove two history groups: %v %v", removed, err)
	}
	page := commentaudit.RenderHistorySection("round 1", removed[:1])
	before, after = recordedSides(page)
	want, err := commentaudit.UnrecordedHistory(removed, before, after)
	if err != nil {
		t.Fatal(err)
	}

	got, err := commentaudit.UnrecordedRemovals([]string{"x.go"}, before, after)

	if err != nil || len(got) != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("UnrecordedRemovals = %v %v, want the composed %v", got, err, want)
	}
}

func TestUnrecordedRemovals_ReturnsTheRemovalErrorUnchanged(t *testing.T) {
	t.Parallel()
	bad := func(string) ([]byte, error) { return []byte("package x\nfunc ("), nil }

	if _, err := commentaudit.UnrecordedRemovals([]string{"x.go"}, bad, bad); !commentaudit.IsUnparsable(err) {
		t.Fatalf("an unparsable file must surface as unparsable: %v", err)
	}
}

func TestRecordHistoryCommand_CarriesTheBaseAndTheArchiveDir(t *testing.T) {
	t.Parallel()

	got := commentaudit.RecordHistoryCommand("abc123")

	want := "cd go && go run ./cmd/commentaudit history -base abc123 -label \"<this change>\" -out " + commentaudit.HistoryArchiveDir
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
	if !strings.Contains(commentaudit.RecordHistoryCommand("HEAD"), "-base HEAD ") {
		t.Error("the commit gate's base is HEAD")
	}
}
