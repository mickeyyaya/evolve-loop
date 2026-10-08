package lifecycle

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	keepClaimBody = `{"id":"k","title":"t","failure_count":0}`
	keepRootBody  = `{"failure_count":1,"id":"k","last_failure_reason":"cycle-failure-release","title":"t"}`
)

type keepFixture struct {
	inbox, root string
	loc         Location
	m           *Mover
	rec         *recordingAppender
}

func newKeepFixture(t *testing.T, claimBody, rootBody string) keepFixture {
	t.Helper()
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	f := keepFixture{
		inbox: inbox,
		root:  filepath.Join(inbox, "k.json"),
		loc:   Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "k.json"), Cycle: 1836},
		rec:   &recordingAppender{},
	}
	writeItem(t, f.loc.Path, claimBody)
	writeItem(t, f.root, rootBody)
	f.m = New(inbox, f.rec)
	return f
}

func readBody(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestMoverReleaseClaimKeeping_KeepRootRemovesAClaimThatIsAFieldSubset(t *testing.T) {
	f := newKeepFixture(t, keepClaimBody, keepRootBody)

	res, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepRoot, "operator")

	if err != nil || res != (ClaimReleaseResult{Path: f.root, Kept: KeepRoot}) {
		t.Fatalf("ReleaseClaimKeeping(root) = %+v, %v; want the root path and Kept root", res, err)
	}
	if _, statErr := os.Stat(f.loc.Path); !os.IsNotExist(statErr) {
		t.Errorf("the claim copy must be removed: %v", statErr)
	}
	if got := readBody(t, f.root); got != keepRootBody {
		t.Errorf("root copy = %s; want it unchanged", got)
	}
	want := ".evolve/inbox/processing/cycle-1836/k.json → .evolve/inbox/k.json: operator; keep: root"
	if len(f.rec.records) != 1 || f.rec.records[0].Action != "release" || f.rec.records[0].Message != want {
		t.Errorf("ledger = %+v; want one release line %q", f.rec.records, want)
	}
}

func TestMoverReleaseClaimKeeping_KeepRootRefusesAClaimThatIsNotAFieldSubset(t *testing.T) {
	cases := []struct {
		name, claim, root, want string
	}{
		{"a curated value differs", `{"id":"k","title":"old"}`, `{"id":"k","title":"new"}`, `"title"`},
		{"an author key is absent from the root", `{"id":"k","kind":"bug"}`, `{"id":"k","title":"t"}`, `"kind"`},
		{"a loop stamp is absent from the root", `{"id":"k","failure_count":1}`, `{"id":"k","title":"t"}`, `"failure_count"`},
		{"an operator stamp differs", `{"id":"k","premise_verified_sha":"a"}`, `{"id":"k","premise_verified_sha":"b"}`, `"premise_verified_sha"`},
		{"a counter is lower in the root", `{"id":"k","failure_count":3}`, `{"id":"k","failure_count":0}`, `"failure_count"; to keep the claim copy, use --keep claim`},
		{"a counter is not a number", `{"id":"k","failure_count":"3"}`, `{"id":"k","failure_count":4}`, `"failure_count"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newKeepFixture(t, tc.claim, `{"id":"k","x":1}`)
			writeItem(t, f.root, tc.root)
			f.m = New(f.inbox, f.rec)

			_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepRoot, "operator")

			if !errors.Is(err, ErrClaimConflict) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v; want ErrClaimConflict naming %s", err, tc.want)
			}
			if readBody(t, f.loc.Path) != tc.claim || readBody(t, f.root) != tc.root || len(f.rec.records) != 0 {
				t.Errorf("a refusal must keep both copies and write no ledger line (records %d)", len(f.rec.records))
			}
		})
	}
}

func TestMoverReleaseClaimKeeping_KeepRootAcceptsAHigherRootCounterAndOtherStamps(t *testing.T) {
	f := newKeepFixture(t, `{"failure_count":1,"id":"k","routed_cycle":1836}`, `{"failure_count":2,"id":"k","routed_cycle":1838}`)

	res, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepRoot, "operator")

	if err != nil || res.Kept != KeepRoot {
		t.Errorf("ReleaseClaimKeeping(claim count 1, root count 2) = %+v, %v; want root-kept", res, err)
	}
}

func TestMoverReleaseClaimKeeping_KeepRootRefusesACopyThatIsNotAJSONObject(t *testing.T) {
	cases := []struct {
		name, claim, root, want string
	}{
		{"the claim copy is a list", `[1]`, `{"id":"k"}`, "processing/cycle-1836/k.json is not a JSON object"},
		{"the claim copy is null", `null`, `{"id":"k"}`, "processing/cycle-1836/k.json is not a JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newKeepFixture(t, tc.claim, tc.root)

			_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepRoot, "operator")

			if !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v; want ErrInvalidItem containing %q", err, tc.want)
			}
			if readBody(t, f.loc.Path) != tc.claim || readBody(t, f.root) != tc.root || len(f.rec.records) != 0 {
				t.Error("a refusal must keep both copies and write no ledger line")
			}
		})
	}
}

func TestMoverReleaseClaimKeeping_KeepClaimMovesTheClaimAndParksTheRootCopy(t *testing.T) {
	f := newKeepFixture(t, `{"id":"k","title":"lane"}`, `{"id":"k","title":"origin"}`)

	res, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepClaim, "operator")

	parked := filepath.Join(f.inbox, "origin-conflicts", "cycle-1836", "k.json")
	if err != nil || res != (ClaimReleaseResult{Path: f.root, Kept: KeepClaim, ParkedPath: parked}) {
		t.Fatalf("ReleaseClaimKeeping(claim) = %+v, %v; want the root path, Kept claim and the park path", res, err)
	}
	if readBody(t, f.root) != `{"id":"k","title":"lane"}` || readBody(t, parked) != `{"id":"k","title":"origin"}` {
		t.Errorf("root = %s, parked = %s; want the claim at the root and origin's copy parked", readBody(t, f.root), readBody(t, parked))
	}
	if _, statErr := os.Stat(f.loc.Path); !os.IsNotExist(statErr) {
		t.Errorf("the claim copy must leave processing/: %v", statErr)
	}
	wantPark := ".evolve/inbox/k.json → .evolve/inbox/origin-conflicts/cycle-1836/k.json: keep claim: the root copy differs from the claim of cycle-1836; operator"
	wantRelease := ".evolve/inbox/processing/cycle-1836/k.json → .evolve/inbox/k.json: operator; keep: claim"
	if len(f.rec.records) != 2 || f.rec.records[0].Action != "park" || f.rec.records[0].Message != wantPark ||
		f.rec.records[1].Action != "release" || f.rec.records[1].Message != wantRelease {
		t.Errorf("ledger = %+v; want a park line %q and a release line %q", f.rec.records, wantPark, wantRelease)
	}
}

func TestMoverReleaseClaimKeeping_WithoutAConflictItIsAPlainRelease(t *testing.T) {
	t.Run("no root copy", func(t *testing.T) {
		f := newKeepFixture(t, keepClaimBody, keepRootBody)
		if err := os.Remove(f.root); err != nil {
			t.Fatal(err)
		}

		res, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepRoot, "operator")

		if err != nil || res != (ClaimReleaseResult{Path: f.root}) || readBody(t, f.root) != keepClaimBody {
			t.Errorf("ReleaseClaimKeeping = %+v, %v; want the claim moved to the root", res, err)
		}
	})
	t.Run("a root copy with the same bytes", func(t *testing.T) {
		f := newKeepFixture(t, keepClaimBody, keepClaimBody)

		res, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepClaim, "operator")

		if err != nil || res != (ClaimReleaseResult{Path: f.root, Duplicate: true}) {
			t.Errorf("ReleaseClaimKeeping = %+v, %v; want the duplicate removed", res, err)
		}
	})
}

func TestMoverReleaseClaimKeeping_RefusesAnUnknownKeepValue(t *testing.T) {
	f := newKeepFixture(t, keepClaimBody, keepRootBody)

	_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepCopy("both"), "operator")

	if !errors.Is(err, ErrBadArgs) || !strings.Contains(err.Error(), `"both"`) || len(f.rec.records) != 0 {
		t.Errorf("err = %v; want ErrBadArgs naming the value, and no ledger line", err)
	}
}

func TestMoverReleaseClaimKeeping_ReportsEachFileSystemFault(t *testing.T) {
	cases := []struct {
		name, lock string
		mode       os.FileMode
		keep       KeepCopy
		sentinel   error
		want       string
	}{
		{"the claim dir refuses the removal", "processing/cycle-1836", 0o555, KeepRoot, ErrMvFailed, "remove the claim copy"},
		{"the claim copy cannot be read", "processing/cycle-1836/k.json", 0o000, KeepRoot, fs.ErrPermission, "compare the claim copy"},
		{"the inbox refuses the park dir", ".", 0o555, KeepClaim, ErrMvFailed, "park the root copy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newKeepFixture(t, keepClaimBody, keepRootBody)
			chmod(t, filepath.Join(f.inbox, tc.lock), tc.mode)

			_, err := f.m.ReleaseClaimKeeping("k", f.loc, tc.keep, "operator")

			if !errors.Is(err, tc.sentinel) || !strings.Contains(err.Error(), tc.want) || len(f.rec.records) != 0 {
				t.Errorf("err = %v, records = %d; want %v containing %q and no ledger line", err, len(f.rec.records), tc.sentinel, tc.want)
			}
			if _, statErr := os.Stat(f.loc.Path); statErr != nil || readBody(t, f.root) != keepRootBody {
				t.Errorf("a fault before the move must keep both copies: %v", statErr)
			}
		})
	}
}

func TestMoverReleaseClaimKeeping_KeepClaimRefusesARootFileOfTheClaimNameBeforeItParks(t *testing.T) {
	f := newKeepFixture(t, keepClaimBody, `{"id":"other"}`)
	twin := filepath.Join(f.inbox, "k-origin.json")
	writeItem(t, twin, keepRootBody)

	_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepClaim, "operator")

	if !errors.Is(err, ErrClaimConflict) || len(f.rec.records) != 0 {
		t.Errorf("err = %v, records = %d; want ErrClaimConflict and no ledger line", err, len(f.rec.records))
	}
	if readBody(t, f.loc.Path) != keepClaimBody || readBody(t, f.root) != `{"id":"other"}` || readBody(t, twin) != keepRootBody {
		t.Error("the claim, the other item and the root copy must stay where they are")
	}
	if _, statErr := os.Stat(filepath.Join(f.inbox, "origin-conflicts")); !os.IsNotExist(statErr) {
		t.Errorf("a refusal must park nothing: %v", statErr)
	}
}

func TestMoverReleaseClaimKeeping_KeepClaimMovesTheRootCopyBackWhenTheClaimCannotMove(t *testing.T) {
	f := newKeepFixture(t, keepClaimBody, keepRootBody)
	chmod(t, filepath.Join(f.inbox, "processing", "cycle-1836"), 0o555)

	_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepClaim, "operator")

	if !errors.Is(err, ErrMvFailed) || len(f.rec.records) != 0 {
		t.Errorf("err = %v, records = %d; want ErrMvFailed and no ledger line", err, len(f.rec.records))
	}
	if readBody(t, f.loc.Path) != keepClaimBody || readBody(t, f.root) != keepRootBody {
		t.Error("a failed claim move must leave the claim and the root copy in place")
	}
	if _, statErr := os.Stat(filepath.Join(f.inbox, "origin-conflicts", "cycle-1836", "k.json")); !os.IsNotExist(statErr) {
		t.Errorf("the root copy must not stay parked: %v", statErr)
	}
}

func TestMoverReleaseClaimKeeping_ReturnsAnInboxScanFault(t *testing.T) {
	f := newKeepFixture(t, keepClaimBody, keepRootBody)
	chmod(t, f.inbox, 0o300)

	_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepRoot, "operator")

	if err == nil || !strings.Contains(err.Error(), "keep root: scan the inbox root") || len(f.rec.records) != 0 {
		t.Errorf("err = %v; want the scan fault of keep root and no ledger line", err)
	}
}

func TestKeepCopy_ValidNamesOnlyRootAndClaim(t *testing.T) {
	for keep, want := range map[KeepCopy]bool{KeepRoot: true, KeepClaim: true, "": false, "both": false, "Root": false} {
		if got := keep.Valid(); got != want {
			t.Errorf("KeepCopy(%q).Valid() = %v, want %v", keep, got, want)
		}
	}
}

func TestMoverReleaseClaimKeeping_KeepClaimRefusesAnOccupiedParkPath(t *testing.T) {
	f := newKeepFixture(t, keepClaimBody, keepRootBody)
	parked := filepath.Join(f.inbox, "origin-conflicts", "cycle-1836", "k.json")
	writeItem(t, parked, `{"id":"k","older":true}`)

	_, err := f.m.ReleaseClaimKeeping("k", f.loc, KeepClaim, "operator")

	if !errors.Is(err, ErrClaimConflict) || !strings.Contains(err.Error(), "park the root copy") || len(f.rec.records) != 0 {
		t.Errorf("err = %v; want ErrClaimConflict that names the park, and no ledger line", err)
	}
	if readBody(t, f.loc.Path) != keepClaimBody || readBody(t, f.root) != keepRootBody || readBody(t, parked) != `{"id":"k","older":true}` {
		t.Error("an occupied park path must change no file")
	}
}

func TestAPI_TheKeepExportsAreNamed(t *testing.T) {
	var (
		_ func(*Mover, string, Location, KeepCopy, string) (ClaimReleaseResult, error) = (*Mover).ReleaseClaimKeeping
		_ func(KeepCopy) bool                                                          = KeepCopy.Valid
		_ KeepCopy                                                                     = KeepRoot
		_ KeepCopy                                                                     = KeepClaim
	)
}
