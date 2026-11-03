package reviewcheck_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/reviewcheck"
)

const deletedItem = ".evolve/inbox/2026-11-03T01-00-00Z-x.json"

func itemReader(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		body, ok := files[path]
		if !ok {
			return nil, errors.New("no such path at base: " + path)
		}
		return []byte(body), nil
	}
}

func deletionCase() reviewcheck.InboxChange {
	return reviewcheck.InboxChange{
		BasePaths: []string{deletedItem, ".evolve/inbox/keep.json", ".evolve/inbox/consumed/old.json"},
		HeadPaths: []string{".evolve/inbox/keep.json", ".evolve/inbox/consumed/old.json"},
		Bodies:    []string{"inbox: tidy the queue\n"},
		ReadBase:  itemReader(map[string]string{deletedItem: `{"id":"x-item"}`}),
	}
}

func TestInboxIntegrity_RefusesThe20261103DeletionWithNoConsumedCopy(t *testing.T) {
	got, err := reviewcheck.InboxIntegrity(deletionCase())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("refusals = %+v, want exactly one", got)
	}
	want := reviewcheck.Refusal{Check: reviewcheck.CheckInboxIntegrity, Subject: "x-item"}
	if got[0].Check != want.Check || got[0].Subject != want.Subject || !strings.Contains(got[0].Reason, deletedItem) {
		t.Errorf("refusal = %+v, want check %q, subject %q and a reason that names %s", got[0], want.Check, want.Subject, deletedItem)
	}
}

func TestInboxIntegrity_AConsumedClaimedOrQuarantinedCopyAccountsForTheItem(t *testing.T) {
	for _, copyPath := range []string{
		".evolve/inbox/consumed/2026-11-03T01-00-00Z-x.json",
		".evolve/inbox/processing/cycle-7/2026-11-03T01-00-00Z-x.json",
		".evolve/inbox/quarantine/2026-11-03T01-00-00Z-x.json",
	} {
		change := deletionCase()
		change.HeadPaths = append(change.HeadPaths, copyPath)
		if got, err := reviewcheck.InboxIntegrity(change); err != nil || len(got) != 0 {
			t.Errorf("copy %s: refusals = %+v, err = %v, want none", copyPath, got, err)
		}
	}
}

func TestInboxIntegrity_ARenameUnderANewNameAccountsForTheItem(t *testing.T) {
	change := deletionCase()
	change.Renamed = map[string]string{deletedItem: ".evolve/inbox/consumed/x-renamed.json"}
	if got, err := reviewcheck.InboxIntegrity(change); err != nil || len(got) != 0 {
		t.Errorf("refusals = %+v, err = %v, want none", got, err)
	}
}

func TestInboxIntegrity_AClosesInboxTrailerByIDOrByFileStemAccountsForTheItem(t *testing.T) {
	for _, trailer := range []string{"Closes-Inbox: x-item", "Closes-Inbox: 2026-11-03T01-00-00Z-x"} {
		change := deletionCase()
		change.Bodies = append(change.Bodies, "fix: land it\n\n"+trailer+"\n")
		if got, err := reviewcheck.InboxIntegrity(change); err != nil || len(got) != 0 {
			t.Errorf("%s: refusals = %+v, err = %v, want none", trailer, got, err)
		}
	}
}

func TestInboxIntegrity_AnItemWithNoIDIsNamedByItsFileStem(t *testing.T) {
	for _, body := range []string{`not json`, `{"title":"x"}`} {
		change := deletionCase()
		change.ReadBase = itemReader(map[string]string{deletedItem: body})
		got, err := reviewcheck.InboxIntegrity(change)
		if err != nil || len(got) != 1 || got[0].Subject != "2026-11-03T01-00-00Z-x" {
			t.Errorf("%s: refusals = %+v, err = %v, want one refusal named by the stem", body, got, err)
		}
	}
}

func TestInboxIntegrity_AClosesInboxOfTheInboxIdentityOfALongIDAccountsForTheItem(t *testing.T) {
	long := strings.Repeat("a", 170)
	change := deletionCase()
	change.ReadBase = itemReader(map[string]string{deletedItem: `{"id":"` + long + `"}`})
	change.Bodies = []string{"Closes-Inbox: " + inboxbatch.ItemID(long, deletedItem) + "\n"}
	if got, err := reviewcheck.InboxIntegrity(change); err != nil || len(got) != 0 {
		t.Errorf("refusals = %+v, err = %v, want none", got, err)
	}
}

func TestInboxIntegrity_AnUnreadableBaseItemIsAnError(t *testing.T) {
	change := deletionCase()
	change.ReadBase = itemReader(nil)
	got, err := reviewcheck.InboxIntegrity(change)
	if err == nil || !strings.Contains(err.Error(), deletedItem) || got != nil {
		t.Errorf("refusals = %+v, err = %v, want an error that names %s", got, err, deletedItem)
	}
}

func TestInboxIntegrity_AnItemOutsideThePendingRootIsNotPending(t *testing.T) {
	change := reviewcheck.InboxChange{BasePaths: []string{".evolve/inbox/consumed/old.json"}, ReadBase: itemReader(nil)}
	if got, err := reviewcheck.InboxIntegrity(change); err != nil || len(got) != 0 {
		t.Errorf("refusals = %+v, err = %v, want none", got, err)
	}
}

func TestInboxIntegrity_AnAddedOrUnchangedItemIsNotRefused(t *testing.T) {
	change := reviewcheck.InboxChange{
		BasePaths: []string{".evolve/inbox/a.json", ".evolve/inbox/.keep"},
		HeadPaths: []string{".evolve/inbox/a.json", ".evolve/inbox/b.json"},
		ReadBase:  itemReader(nil),
	}
	got, err := reviewcheck.InboxIntegrity(change)
	if err != nil || !reflect.DeepEqual(got, []reviewcheck.Refusal(nil)) {
		t.Errorf("refusals = %+v, err = %v, want none", got, err)
	}
}
