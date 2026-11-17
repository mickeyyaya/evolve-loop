package inboxmover

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCommittedIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"invalid json", "not json{", nil},
		{"empty object", "{}", nil},
		{
			"top_n union skip_shipped, deduped and order-preserving",
			`{"top_n":[{"id":"a"},{"id":""},{"id":"b"},{"id":"a"}],"skip_shipped":[{"task_id":"b"},{"task_id":"c"}]}`,
			[]string{"a", "b", "c"},
		},
		{"skip_shipped only", `{"skip_shipped":[{"task_id":"z"}]}`, []string{"z"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CommittedIDs([]byte(tc.body))
			if len(got) != len(tc.want) {
				t.Fatalf("CommittedIDs(%s) = %v, want %v", tc.body, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("CommittedIDs(%s)[%d] = %q, want %q", tc.body, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestApplyCycleOutcome_ResultReportsMovedIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-a.json"), []byte(`{"id":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: io.Discard, IsLandedFn: func(string) (bool, error) { return true, nil }}
	oc := CycleOutcome{Cycle: 7, Passed: true, CommittedIDs: []string{"a", "a", ""}}

	var res OutcomeResult
	res, err := ApplyCycleOutcome(opts, oc)
	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 1 || res.Promoted[0] != "a" {
		t.Errorf("res.Promoted = %v, want [a] (duplicate and empty ids collapse)", res.Promoted)
	}

	res, err = ApplyCycleOutcome(opts, oc)
	if err != nil {
		t.Fatalf("second ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 0 {
		t.Errorf("res.Promoted = %v on re-apply, want empty: an already-promoted id moved nothing", res.Promoted)
	}
}

func TestClaimLaneScope_ToleratesUnresolvableIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-a.json"), []byte(`{"id":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	claimed := ClaimLaneScope(Options{ProjectRoot: root, Stderr: io.Discard}, 7, []string{"a", "ghost"})
	if len(claimed) != 1 || claimed[0] != "a" {
		t.Fatalf("claimed = %v, want [a]", claimed)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, "processing", "cycle-7", "item-a.json")); statErr != nil {
		t.Errorf("item-a.json not claimed into processing/cycle-7/: %v", statErr)
	}
}

func TestLandedClaims_ReadsTheShipShaAndTheTokenAfterTheColon(t *testing.T) {
	body := []byte(`{"skip_shipped":[{"task_id":"a","git_sha":" abc1234 "},{"task_id":"blank","git_sha":""}],
"dropped":[{"id":"b","reason":"Already-Shipped: def5678 (closed)"},{"id":"c","reason":"already-shipped"},{"id":"d","reason":"stale: 0123456"},{"id":"e","reason":"already-shipped:   "}]}`)
	got := LandedClaims(body)
	if len(got) != 2 || got["a"] != "abc1234" || got["b"] != "def5678" {
		t.Errorf("LandedClaims = %v, want a and b only", got)
	}
	if LandedClaims([]byte("not json")) != nil {
		t.Error("an unreadable body claims nothing")
	}
}

func TestDropReasonToken_SplitsTheTagFromItsFirstWord(t *testing.T) {
	for reason, want := range map[string][2]string{
		"protected-surface: go/a.go is fenced": {"protected-surface", "go/a.go"},
		"  Stale ":                             {"stale", ""},
		"duplicate:":                           {"duplicate", ""},
	} {
		if tag, token := DropReasonToken(reason); tag != want[0] || token != want[1] {
			t.Errorf("DropReasonToken(%q) = %q, %q, want %v", reason, tag, token, want)
		}
	}
}
