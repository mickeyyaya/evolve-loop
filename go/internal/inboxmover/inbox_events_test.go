package inboxmover

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func lifecycleEvents(c *signalcenter.Center) *[]signalcenter.Event {
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindInboxClaimed || e.Kind == signalcenter.KindInboxReleased {
			got = append(got, e)
		}
	})
	return &got
}

func eventsOfKind(all []signalcenter.Event, kind signalcenter.Kind) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range all {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func loneInboxInfo(t *testing.T, all []signalcenter.Event, kind signalcenter.Kind) signalcenter.Event {
	t.Helper()
	got := eventsOfKind(all, kind)
	if len(got) != 1 {
		t.Fatalf("%s events = %+v, want exactly one", kind, got)
	}
	e := got[0]
	if e.Module != signalcenter.ModuleInbox || e.Severity != signalcenter.SeverityInfo || e.Code != "" || e.Reason == "" || !signalcenter.ValidOrigin(e.Origin) {
		t.Errorf("%s = module %s severity %s code %q origin %q reason %q, want an INFO inbox event with no code and a reason", kind, e.Module, e.Severity, e.Code, e.Origin, e.Reason)
	}
	return e
}

func pendingItemAtRoot(t *testing.T, root, id, extra string) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".evolve", "inbox", id+".json"), `{"id":"`+id+`"`+extra+`}`)
}

func signalledOptions(t *testing.T) (Options, string, *[]signalcenter.Event) {
	t.Helper()
	opts, root := claimsFixture(t)
	opts.Stderr = io.Discard
	opts.Signals = signalcenter.New()
	return opts, root, lifecycleEvents(opts.Signals)
}

func TestInboxLifecycle_EmitsClaimedAndReleased(t *testing.T) {
	opts, root, got := signalledOptions(t)
	pendingItemAtRoot(t, root, "ship-it", "")

	if _, err := ClaimDispatchable(opts, "ship-it", "1828"); err != nil {
		t.Fatalf("ClaimDispatchable: %v", err)
	}
	claimed := loneInboxInfo(t, *got, signalcenter.KindInboxClaimed)
	if claimed.Cycle != 1828 || claimed.Fields["id"] != "ship-it" {
		t.Errorf("inbox.claimed = cycle %d fields %v, want cycle 1828 for id ship-it", claimed.Cycle, claimed.Fields)
	}

	res, err := ReleaseClaim(opts, "ship-it", "holder sealed FAIL in wave 77")
	if err != nil || res.Outcome != ClaimReleased {
		t.Fatalf("ReleaseClaim = %+v, %v; want released", res, err)
	}
	released := loneInboxInfo(t, *got, signalcenter.KindInboxReleased)
	if released.Cycle != 1828 || released.Fields["id"] != "ship-it" || released.Fields["outcome"] != string(ClaimReleased) || !strings.Contains(released.Reason, "holder sealed FAIL in wave 77") {
		t.Errorf("inbox.released = cycle %d fields %v reason %q, want cycle 1828, id ship-it, outcome released and the operator reason", released.Cycle, released.Fields, released.Reason)
	}
	if len(*got) != 2 {
		t.Errorf("lifecycle events = %d, want exactly the claim and the release", len(*got))
	}
}

func TestInboxLifecycle_ClaimPendingEmitsOneClaimedForEachItemItClaims(t *testing.T) {
	opts, root, got := signalledOptions(t)
	pendingItemAtRoot(t, root, "first", "")
	pendingItemAtRoot(t, root, "second", "")
	claimItem(t, root, 9, "sibling")

	if err := ClaimPending(opts, 7, []string{"first", "first", "second", "sibling", "never-filed"}); err != nil {
		t.Fatalf("ClaimPending: %v", err)
	}

	claimed := eventsOfKind(*got, signalcenter.KindInboxClaimed)
	ids := map[string]int{}
	for _, e := range claimed {
		ids[e.Fields["id"]]++
		if e.Cycle != 7 {
			t.Errorf("inbox.claimed %v at cycle %d, want 7", e.Fields, e.Cycle)
		}
	}
	if len(claimed) != 2 || ids["first"] != 1 || ids["second"] != 1 {
		t.Errorf("inbox.claimed ids = %v, want exactly one each for first and second (not the duplicate id, the sibling's claim or the unknown id)", ids)
	}
}

func TestInboxLifecycle_ARefusedClaimEmitsNoClaimedEvent(t *testing.T) {
	opts, root, got := signalledOptions(t)
	pendingItemAtRoot(t, root, "operator-owned", `,"route":"console-only"`)
	writeFile(t, filepath.Join(root, ".evolve", "inbox", "retry", "dep.json"), `{"id":"dep"}`)
	pendingItemAtRoot(t, root, "waiting", `,"deps":["dep"]`)

	if _, err := ClaimDispatchable(opts, "waiting", "7"); !errors.Is(err, ErrWaitingOnDependency) {
		t.Fatalf("ClaimDispatchable(waiting) = %v, want ErrWaitingOnDependency", err)
	}
	if _, err := ClaimDispatchable(opts, "operator-owned", "7"); !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("ClaimDispatchable(operator-owned) = %v, want ErrConsoleRouted", err)
	}
	if _, err := ClaimDispatchable(opts, "never-filed", "7"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ClaimDispatchable(never-filed) = %v, want ErrNotFound", err)
	}

	if n := len(eventsOfKind(*got, signalcenter.KindInboxClaimed)); n != 0 {
		t.Errorf("inbox.claimed events = %d, want none: no item moved", n)
	}
}

func TestInboxLifecycle_AReleaseThatMovesNothingEmitsNoReleasedEvent(t *testing.T) {
	opts, root, got := signalledOptions(t)
	writeHolderRun(t, root, holderRun{cycle: 1837, goal: "g81", phase: "build", lease: "live"})
	claimItem(t, root, 1837, "held-item")
	pendingItemAtRoot(t, root, "unclaimed", "")

	if _, err := ReleaseClaim(opts, "held-item", "operator"); !errors.Is(err, ErrClaimHeld) {
		t.Fatalf("ReleaseClaim(held-item) = %v, want ErrClaimHeld", err)
	}
	if res, err := ReleaseClaim(opts, "unclaimed", "operator"); err != nil || res.Outcome != ClaimNotHeld {
		t.Fatalf("ReleaseClaim(unclaimed) = %+v, %v, want not-claimed", res, err)
	}
	if _, err := ReleaseClaim(opts, "never-filed", "operator"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReleaseClaim(never-filed) = %v, want ErrNotFound", err)
	}
	if _, err := ReleaseClaim(opts, "held-item", " "); !errors.Is(err, ErrBadArgs) {
		t.Fatalf("ReleaseClaim with no reason = %v, want ErrBadArgs", err)
	}

	if n := len(eventsOfKind(*got, signalcenter.KindInboxReleased)); n != 0 {
		t.Errorf("inbox.released events = %d, want none: no claim ended", n)
	}
}

func TestInboxLifecycle_AFailedReleaseEmitsNoReleasedEvent(t *testing.T) {
	opts, root, got := signalledOptions(t)
	claimItem(t, root, 1836, "diff-item")
	rootItem(t, root, "diff-item", `{"id":"diff-item","weight":0.9}`)

	if _, err := ReleaseClaim(opts, "diff-item", "restored by hand"); !errors.Is(err, ErrClaimConflict) {
		t.Fatalf("ReleaseClaim = %v, want ErrClaimConflict", err)
	}

	if n := len(eventsOfKind(*got, signalcenter.KindInboxReleased)); n != 0 {
		t.Errorf("inbox.released events = %d, want none: the conflict changed nothing", n)
	}
}

func TestInboxLifecycle_ReleaseStaleClaimsEmitsOneReleasedForEachClaimItEnds(t *testing.T) {
	opts, root, got := signalledOptions(t)
	writeHolderRun(t, root, holderRun{cycle: 1837, goal: "g81", phase: "build", lease: "live"})
	claimItem(t, root, 1837, "live-item")
	claimItem(t, root, 1828, "stale-a")
	claimItem(t, root, 1829, "stale-b")

	released, err := ReleaseStaleClaims(opts, "wave-plan")
	if err != nil || len(released) != 2 {
		t.Fatalf("ReleaseStaleClaims = %+v, %v, want the two stale claims", released, err)
	}

	byID := map[string]signalcenter.Event{}
	for _, e := range eventsOfKind(*got, signalcenter.KindInboxReleased) {
		byID[e.Fields["id"]] = e
	}
	if len(byID) != 2 || byID["stale-a"].Cycle != 1828 || byID["stale-b"].Cycle != 1829 {
		t.Errorf("inbox.released by id = %v, want stale-a at 1828 and stale-b at 1829 and nothing for the live holder", byID)
	}
	for id, e := range byID {
		if !strings.Contains(e.Reason, "wave-plan") {
			t.Errorf("inbox.released %s reason %q does not carry the sweep reason wave-plan", id, e.Reason)
		}
	}
}

func TestInboxLifecycle_ReleaseClaimKeepingAndADuplicateRemovalAreReleases(t *testing.T) {
	opts, root, got := signalledOptions(t)
	claimItem(t, root, 1836, "dup-item")
	rootItem(t, root, "dup-item", `{"id":"dup-item","weight":0.5}`)

	dup, err := ReleaseClaim(opts, "dup-item", "restored by hand")
	if err != nil || dup.Outcome != ClaimDuplicateRemoved {
		t.Fatalf("ReleaseClaim = %+v, %v, want duplicate-removed", dup, err)
	}
	claimItem(t, root, 1835, "keep-item")
	rootItem(t, root, "keep-item", `{"id":"keep-item","weight":0.9}`)
	kept, err := ReleaseClaimKeeping(opts, "keep-item", "keep the claim copy", KeepClaim)
	if err != nil || kept.Outcome != ClaimClaimKept {
		t.Fatalf("ReleaseClaimKeeping = %+v, %v, want claim-kept", kept, err)
	}

	outcomes := map[string]string{}
	for _, e := range eventsOfKind(*got, signalcenter.KindInboxReleased) {
		outcomes[e.Fields["id"]] = e.Fields["outcome"]
	}
	if outcomes["dup-item"] != string(ClaimDuplicateRemoved) || outcomes["keep-item"] != string(ClaimClaimKept) || len(outcomes) != 2 {
		t.Errorf("inbox.released outcomes = %v, want dup-item duplicate-removed and keep-item claim-kept", outcomes)
	}
}

func TestInboxLifecycle_ANilCenterIsTheNullObject(t *testing.T) {
	opts, root := claimsFixture(t)
	opts.Stderr = io.Discard
	pendingItemAtRoot(t, root, "quiet", "")

	if _, err := ClaimDispatchable(opts, "quiet", "1828"); err != nil {
		t.Fatalf("ClaimDispatchable with no Center: %v", err)
	}
	res, err := ReleaseClaim(opts, "quiet", "no listener")
	if err != nil || res.Outcome != ClaimReleased {
		t.Fatalf("ReleaseClaim with no Center = %+v, %v, want released", res, err)
	}
	loc, err := Locate(filepath.Join(root, ".evolve", "inbox"), "quiet")
	if err != nil || loc.Cycle != 0 {
		t.Errorf("Locate after the release = %+v, %v, want the item pending at the root", loc, err)
	}
}
