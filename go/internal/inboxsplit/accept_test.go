package inboxsplit

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type fakeInbox struct {
	pending    map[string]map[string]json.RawMessage
	dependents []string
	calls      []string
	failOn     map[string]error
}

var _ Inbox = (*fakeInbox)(nil)

func newFake(t *testing.T) *fakeInbox {
	return &fakeInbox{pending: map[string]map[string]json.RawMessage{"fix-it": mixedItem(t)}, failOn: map[string]error{}}
}

func (f *fakeInbox) record(call string) error {
	f.calls = append(f.calls, call)
	return f.failOn[call]
}

func (f *fakeInbox) Item(id string) (map[string]json.RawMessage, error) {
	item, ok := f.pending[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotPending, id)
	}
	return item, f.failOn["item"]
}

func (f *fakeInbox) Dependents(id string) ([]string, error) {
	return f.dependents, f.failOn["dependents"]
}

func (f *fakeInbox) File(raw []byte) error {
	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err != nil {
		return err
	}
	var id string
	_ = json.Unmarshal(item["id"], &id)
	if err := f.record("file " + id); err != nil {
		return err
	}
	f.pending[id] = item
	return nil
}

func (f *fakeInbox) RouteLane(id, reason string) error { return f.record("route-lane " + id) }

func (f *fakeInbox) Withdraw(id, reason string) error {
	if err := f.record("withdraw " + id); err != nil {
		return err
	}
	delete(f.pending, id)
	return nil
}

func (f *fakeInbox) Retire(id, resolution string) error {
	if err := f.record("retire " + id); err != nil {
		return err
	}
	delete(f.pending, id)
	return nil
}

func TestAccept_FilesBothPartsRoutesTheLanePartThenRetiresTheOriginal(t *testing.T) {
	f := newFake(t)

	res, err := Accept(f, "fix-it", isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{"file fix-it-console", "file fix-it-lane", "route-lane fix-it-lane", "retire fix-it"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
	if want := (Result{OriginalID: "fix-it", ConsoleID: "fix-it-console", LaneID: "fix-it-lane"}); res != want {
		t.Errorf("result = %+v, want %+v", res, want)
	}
}

func TestAccept_NeverWithdrawsTheOriginalSoItsEvidenceSurvives(t *testing.T) {
	f := newFake(t)
	if _, err := Accept(f, "fix-it", isProtected, false); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if c == "withdraw fix-it" {
			t.Error("the original was withdrawn, which deletes its file")
		}
	}
}

func TestAccept_AcceptingTwiceIsRefusedAndWritesNothingMore(t *testing.T) {
	f := newFake(t)
	if _, err := Accept(f, "fix-it", isProtected, false); err != nil {
		t.Fatal(err)
	}
	n := len(f.calls)

	_, err := Accept(f, "fix-it", isProtected, false)

	if !errors.Is(err, ErrNotPending) {
		t.Errorf("err = %v, want ErrNotPending", err)
	}
	if len(f.calls) != n {
		t.Errorf("the second accept wrote: %v", f.calls[n:])
	}
}

func TestAccept_RefusesAnItemThatNeedsNoSplitWithoutWriting(t *testing.T) {
	f := newFake(t)
	f.pending["fix-it"] = mkItem(t, map[string]any{"id": "fix-it", "files": []string{"guard/a.go"}})
	if _, err := Accept(f, "fix-it", isProtected, false); !errors.Is(err, ErrAllProtected) {
		t.Errorf("err = %v", err)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestAccept_RefusesWhileAPendingItemDependsOnTheOriginal(t *testing.T) {
	f := newFake(t)
	f.dependents = []string{"waiter"}
	_, err := Accept(f, "fix-it", isProtected, false)
	if !errors.Is(err, ErrHasDependents) {
		t.Errorf("err = %v, want ErrHasDependents", err)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestAccept_ReportsEachReadFailureAndWritesNothing(t *testing.T) {
	for _, key := range []string{"item", "dependents"} {
		f := newFake(t)
		f.failOn[key] = errors.New("boom")
		if _, err := Accept(f, "fix-it", isProtected, false); err == nil {
			t.Errorf("%s: no error", key)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s: calls = %v", key, f.calls)
		}
	}
}

func TestAccept_ALanePartThatCannotBeFiledRemovesTheConsolePart(t *testing.T) {
	f := newFake(t)
	f.failOn["file fix-it-lane"] = errors.New("boom")

	_, err := Accept(f, "fix-it", isProtected, false)

	if err == nil {
		t.Fatal("no error")
	}
	if _, ok := f.pending["fix-it-console"]; ok {
		t.Error("the console part stayed filed")
	}
	if _, ok := f.pending["fix-it"]; !ok {
		t.Error("the original was retired")
	}
}

func TestAccept_ARefusedLaneRouteRemovesBothPartsAndKeepsTheOriginal(t *testing.T) {
	f := newFake(t)
	f.failOn["route-lane fix-it-lane"] = errors.New("stays operator-owned")

	_, err := Accept(f, "fix-it", isProtected, false)

	if err == nil {
		t.Fatal("no error")
	}
	if len(f.pending) != 1 || f.pending["fix-it"] == nil {
		t.Errorf("pending = %v, want only the original", f.pending)
	}
}

func TestAccept_ARollbackThatFailsIsReportedWithTheCause(t *testing.T) {
	f := newFake(t)
	f.failOn["route-lane fix-it-lane"] = errors.New("cause-text")
	f.failOn["withdraw fix-it-lane"] = errors.New("rollback-text")

	_, err := Accept(f, "fix-it", isProtected, false)

	if err == nil || !strings.Contains(err.Error(), "cause-text") || !strings.Contains(err.Error(), "rollback-text") {
		t.Errorf("err = %v, want both the cause and the rollback failure", err)
	}
}

func TestAccept_AFailedRetireLeavesBothPartsAndSaysSo(t *testing.T) {
	f := newFake(t)
	f.failOn["retire fix-it"] = errors.New("disk")

	_, err := Accept(f, "fix-it", isProtected, false)

	if !errors.Is(err, ErrRetireFailed) {
		t.Errorf("err = %v, want ErrRetireFailed", err)
	}
	if f.pending["fix-it-console"] == nil || f.pending["fix-it-lane"] == nil {
		t.Error("a part was removed after the lane part was routed")
	}
}

func TestAccept_AConsolePartThatCannotBeFiledWritesNothingElse(t *testing.T) {
	f := newFake(t)
	f.failOn["file fix-it-console"] = errors.New("boom")

	_, err := Accept(f, "fix-it", isProtected, false)

	if err == nil || len(f.calls) != 1 || len(f.pending) != 1 {
		t.Errorf("err = %v, calls = %v, pending = %d", err, f.calls, len(f.pending))
	}
}
