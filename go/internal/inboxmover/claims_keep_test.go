package inboxmover

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const liveCaseRootBody = `{"failure_count":1,"id":"stuck-item","last_failure_reason":"cycle-failure-release","weight":0.5}`

func TestReleaseClaimKeeping_KeepRootResolvesARootCopyWithNewerStamps(t *testing.T) {
	opts, root := claimsFixture(t)
	rec := &recordingAppender{}
	opts.Ledger = rec
	src := claimItem(t, root, 1836, "stuck-item")
	kept := rootItem(t, root, "stuck-item", liveCaseRootBody)
	if _, err := ReleaseClaim(opts, "stuck-item", "operator"); !errors.Is(err, ErrClaimConflict) {
		t.Fatalf("a plain release must still refuse the conflict: %v", err)
	}

	res, err := ReleaseClaimKeeping(opts, "stuck-item", "operator", KeepRoot)

	want := ClaimRelease{ID: "stuck-item", Cycle: 1836, Outcome: ClaimRootKept, Path: kept, Holder: res.Holder}
	if err != nil || res != want || res.Holder.Verdict != HolderStale {
		t.Fatalf("ReleaseClaimKeeping(root) = %+v, %v; want %+v from a stale holder", res, err, want)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Errorf("the claim copy must be removed: %v", statErr)
	}
	if loc, err := Locate(filepath.Join(root, ".evolve", "inbox"), "stuck-item"); err != nil || loc.Cycle != 0 || loc.Path != kept {
		t.Errorf("the item must be pending at the root copy: %+v %v", loc, err)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "release" || !strings.HasSuffix(rec.records[0].Message, "operator; keep: root") {
		t.Errorf("ledger = %+v; want one release line that names keep: root", rec.records)
	}
}

func TestReleaseClaimKeeping_KeepClaimPutsTheClaimCopyAtTheRoot(t *testing.T) {
	opts, root := claimsFixture(t)
	src := claimItem(t, root, 1836, "stuck-item")
	kept := rootItem(t, root, "stuck-item", `{"id":"stuck-item","weight":0.9}`)

	res, err := ReleaseClaimKeeping(opts, "stuck-item", "operator", KeepClaim)

	parked := filepath.Join(root, ".evolve", "inbox", "origin-conflicts", "cycle-1836", filepath.Base(kept))
	if err != nil || res.Outcome != ClaimClaimKept || res.Path != kept || res.Cycle != 1836 || res.ParkedPath != parked {
		t.Fatalf("ReleaseClaimKeeping(claim) = %+v, %v; want claim-kept at %s, parked at %s", res, err, kept, parked)
	}
	body, readErr := os.ReadFile(kept)
	if readErr != nil || string(body) != `{"id":"stuck-item","weight":0.5}` {
		t.Errorf("the root holds %s, %v; want the claim copy", body, readErr)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Errorf("the claim copy must leave processing/: %v", statErr)
	}
}

func TestReleaseClaimKeeping_KeepsTheContractOfRelease(t *testing.T) {
	cases := []struct {
		name     string
		id, keep string
		live     bool
		want     error
	}{
		{"a live holder", "stuck-item", "root", true, ErrClaimHeld},
		{"an unknown id", "never-filed", "root", false, ErrNotFound},
		{"an unknown keep value", "stuck-item", "both", false, ErrBadArgs},
		{"a claim that is not a field subset", "stuck-item", "root", false, ErrClaimConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, root := claimsFixture(t)
			if tc.live {
				writeHolderRun(t, root, holderRun{cycle: 1836, goal: "g81", phase: "build", lease: "live"})
			}
			src := claimItem(t, root, 1836, "stuck-item")
			rootItem(t, root, "stuck-item", `{"id":"stuck-item","weight":0.9}`)

			_, err := ReleaseClaimKeeping(opts, tc.id, "operator", KeepCopy(tc.keep))

			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v; want %v", err, tc.want)
			}
			if _, statErr := os.Stat(src); statErr != nil {
				t.Errorf("a refusal must keep the claim copy: %v", statErr)
			}
		})
	}
}
