package ledger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
)

const notAPatchF2 = "this is prose, not a unified diff\n"

func TestCheckCompositionLedgerPath_OnlyALedgerJSONLIsAcceptedF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"a ledger.jsonl", filepath.Join(dir, "ledger.jsonl"), false},
		{"a differently named file", filepath.Join(dir, "other.jsonl"), true},
		{"a suffixed name", filepath.Join(dir, "ledger.jsonl.bak"), true},
		{"a prefixed name", filepath.Join(dir, "my-ledger.jsonl"), true},
		{"the empty path", "", true},
		{"a dir named like the ledger parent", dir, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkCompositionLedgerPath(tc.path)

			if (err != nil) != tc.wantErr || (err != nil && !strings.Contains(err.Error(), "must be a ledger.jsonl")) {
				t.Errorf("checkCompositionLedgerPath(%q) = %v, wantErr %v", tc.path, err, tc.wantErr)
			}
		})
	}
}

func TestWriteCompositionVerdict_RefusesALedgerPathThatIsNotALedgerAndWritesNothingF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := WriteCompositionVerdict(filepath.Join(dir, "notes.jsonl"), honestWriteInput(t))

	entries, _ := os.ReadDir(dir)
	if err == nil || len(entries) != 0 {
		t.Errorf("WriteCompositionVerdict = %v with %d files written, want a refusal and nothing on disk", err, len(entries))
	}
}

func TestValidateCompositionInput_NamesTheDiffThatIsNotAPatchF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		audited, comped string
		wantLabel       string
	}{
		{"audited diff is prose", notAPatchF2, compTestDiff, "audited diff"},
		{"composed diff is prose", compTestDiff, notAPatchF2, "composed diff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := honestWriteInput(t)
			in.AuditedDiff, in.ComposedDiff = []byte(tc.audited), []byte(tc.comped)

			err := validateCompositionInput(in)

			if err == nil || !strings.Contains(err.Error(), tc.wantLabel+": git patch-id") || !strings.Contains(err.Error(), "empty output") {
				t.Errorf("validateCompositionInput = %v, want the patch-id failure of the %s", err, tc.wantLabel)
			}
		})
	}
}

func TestValidateCompositionInput_AcceptsAnHonestInputAndRejectsTheEmptyAndTheMismatchedF2(t *testing.T) {
	t.Parallel()
	honest := honestWriteInput(t)
	empty, mismatched, missingGate := honest, honest, honest
	empty.ComposedDiff = []byte(" \n\t")
	mismatched.PatchID = "0000000000000000000000000000000000000000"
	missingGate.GateResults = map[string]string{}
	cases := []struct {
		name string
		in   CompositionVerdictInput
		want string
	}{
		{"honest", honest, ""},
		{"whitespace-only composed diff", empty, "composed diff is empty or whitespace-only"},
		{"claimed patch-id differs", mismatched, "recomputes patch-id"},
		{"gates not green", missingGate, "composed-tree gates not green"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateCompositionInput(tc.in)

			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Errorf("validateCompositionInput = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestVerifyCompositionLine_BreaksTheChainForEachUnreadableOrNonPatchLineF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := ledgerartifacts.Open(dir)
	proseDigest, err := store.Put([]byte(notAPatchF2))
	if err != nil {
		t.Fatal(err)
	}
	patchID := honestWriteInput(t).PatchID
	index := indexCompositionEvidence(store, nil)
	cases := []struct {
		name string
		line string
		want string
	}{
		{"line is not JSON", `{"kind": not json`, "line 7 composition-verdict unmarshal"},
		{"line has no patch id", `{"kind":"composition-verdict"}`, "has no patch_id"},
		{"stored diff is not a patch", `{"kind":"composition-verdict","patch_id":"` + patchID + `","audited_diff_sha256":"` + proseDigest + `","composed_diff_sha256":"` + proseDigest + `"}`, "audited_diff_path: git patch-id"},
		{"stored diff is gone", `{"kind":"composition-verdict","patch_id":"` + patchID + `","audited_diff_sha256":"` + strings.Repeat("a", 64) + `","composed_diff_sha256":"` + proseDigest + `"}`, "audited_diff_path unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := verifyCompositionLine(7, []byte(tc.line), index)

			if !errors.Is(err, core.ErrLedgerChainBroken) || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "line 7") {
				t.Errorf("verifyCompositionLine = %v, want a chain break containing %q", err, tc.want)
			}
		})
	}
}

func TestVerify_ACompositionLineWhoseStoredDiffIsNotAPatchIsAChainBreakF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	digest, err := ledgerartifacts.Open(dir).Put([]byte(notAPatchF2))
	if err != nil {
		t.Fatal(err)
	}
	appendCompositionLine(t, dir, compositionRecord{Cycle: 1766, PatchID: honestWriteInput(t).PatchID,
		AuditedDiffSHA256: digest, ComposedDiffSHA256: digest})

	err = New(dir).Verify(context.Background())

	if !errors.Is(err, core.ErrLedgerChainBroken) || !strings.Contains(err.Error(), "git patch-id") {
		t.Errorf("Verify = %v, want a chain break naming the patch-id failure", err)
	}
}

func TestLatestCompositionVerdict_ReportsAnUnreadableLedgerAndAnAbsentOneDifferentlyF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	asDir := filepath.Join(dir, "ledger.jsonl")
	if err := os.Mkdir(asDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, found, err := LatestCompositionVerdict(asDir, TrivialRebaseMethod, "ref")

	if err == nil || found || got.Cycle != 0 || got.PatchID != "" {
		t.Errorf("LatestCompositionVerdict over a directory = (%+v, %v, %v), want the read error and no verdict", got, found, err)
	}
	got, found, err = LatestCompositionVerdict(filepath.Join(dir, "absent.jsonl"), TrivialRebaseMethod, "ref")
	if err != nil || found || got.PatchID != "" {
		t.Errorf("LatestCompositionVerdict over an absent ledger = (%+v, %v, %v), want no carry and no error", got, found, err)
	}
}

func TestLatestCompositionVerdict_TheNewestMatchingLineWinsAndOthersAreSkippedF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	for _, rec := range []compositionRecord{
		{Cycle: 1, Method: TrivialRebaseMethod, LaneAuditRef: "ref", PatchID: "p1"},
		{Cycle: 2, Method: TrivialRebaseMethod, LaneAuditRef: "other-ref", PatchID: "p2"},
		{Cycle: 3, Method: IdenticalRebaseMethod, LaneAuditRef: "ref", PatchID: "p3"},
		{Cycle: 4, Method: TrivialRebaseMethod, LaneAuditRef: "ref", PatchID: "p4"},
	} {
		appendCompositionLine(t, dir, rec)
	}
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath, append(append([]byte("not json\n\n"), raw...), []byte("{\"kind\":\"phase\",\"method\":\""+TrivialRebaseMethod+"\"}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	got, found, err := LatestCompositionVerdict(ledgerPath, TrivialRebaseMethod, "ref")

	if err != nil || !found || got.Cycle != 4 || got.PatchID != "p4" {
		t.Errorf("LatestCompositionVerdict = (%+v, %v, %v), want cycle 4 p4", got, found, err)
	}
}
