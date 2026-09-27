package triage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type unifiedFixtureItem struct {
	id, campaign string
}

func unifiedItems(n int, campaign string) []unifiedFixtureItem {
	items := make([]unifiedFixtureItem, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, unifiedFixtureItem{id: fmt.Sprintf("unified-member-%d", i), campaign: campaign})
	}
	return items
}

func itemIDs(items []unifiedFixtureItem) []string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.id)
	}
	return ids
}

// writeUnifiedFixture lays out an inbox holding items and a triage decision
// committing topN; members (when non-nil) is declared as unified_commitment,
// and spoofProjection plants a projection the triage phase never validated.
// A stale campaign-plan.json is always present so every branch must decide it.
func writeUnifiedFixture(t *testing.T, items []unifiedFixtureItem, topN, members []string, spoofProjection bool) core.PhaseRequest {
	t.Helper()
	root, ws := t.TempDir(), t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		raw := fmt.Sprintf(`{"id":%q,"title":"fixture %s","weight":0.5,"campaign":%q}`, it.id, it.id, it.campaign)
		if err := os.WriteFile(filepath.Join(inbox, it.id+".json"), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	top := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		top = append(top, map[string]string{"id": id})
	}
	doc := map[string]any{"cycle": 1720, "top_n": top}
	if members != nil {
		declared := make([]inboxbatch.UnifiedMember, 0, len(members))
		for _, id := range members {
			declared = append(declared, inboxbatch.UnifiedMember{ID: id, Evidence: id + " shares the seam gap"})
		}
		doc["unified_commitment"] = inboxbatch.UnifiedCommitment{
			RootCauseHypothesis: "every member is one copy of the same seam gap",
			SharedSeam:          "go/internal/phases/ship/consume.go",
			DesignRequirements:  []string{"members close transactionally"},
			Members:             declared,
		}
	}
	if spoofProjection {
		doc["unified_projection"] = map[string]any{"size": "small", "member_count": 2}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "campaign-plan.json"), []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.PhaseRequest{Cycle: 1720, ProjectRoot: root, Workspace: ws}
}

// Each subtest pins one branch by what it writes: the projection, the campaign
// plan, the rejection diagnostic and the preserved top_n.
func TestProcessUnifiedCommitment(t *testing.T) {
	small := unifiedItems(2, "seam")
	large := unifiedItems(inboxbatch.DefaultMaxItems+1, "seam")
	mixed := []unifiedFixtureItem{{id: "unified-member-0", campaign: "seam"}, {id: "unified-member-1", campaign: "other"}}

	cases := []struct {
		name            string
		items           []unifiedFixtureItem
		topN, members   []string
		spoofProjection bool
		wantProjection  *unifiedProjection
		wantPlan        bool
		wantRejection   string
	}{
		{name: "accept-small", items: small, topN: itemIDs(small), members: itemIDs(small),
			wantProjection: &unifiedProjection{Size: "small", MemberCount: 2}},
		{name: "accept-large", items: large, topN: itemIDs(large), members: itemIDs(large),
			wantProjection: &unifiedProjection{Size: "large", MemberCount: len(large)}, wantPlan: true},
		{name: "reject-outside-top_n", items: small, topN: itemIDs(small)[:1], members: itemIDs(small),
			wantRejection: `member "unified-member-1" is outside top_n`},
		{name: "reject-heterogeneous", items: mixed, topN: itemIDs(mixed), members: itemIDs(mixed),
			wantRejection: "members span distinct campaigns"},
		{name: "reject-spoofed", items: small, topN: itemIDs(small), spoofProjection: true,
			wantRejection: "projection declared without unified_commitment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := writeUnifiedFixture(t, tc.items, tc.topN, tc.members, tc.spoofProjection)

			diags, err := processUnifiedCommitment(req)
			if err != nil {
				t.Fatalf("processUnifiedCommitment: %v", err)
			}

			assertUnifiedDiagnostics(t, diags, tc.wantRejection)
			fields := readDecisionFields(t, req.Workspace)
			assertUnifiedProjection(t, fields, tc.wantProjection)
			assertTopNPreserved(t, fields, tc.topN)
			assertCampaignPlan(t, req.Workspace, tc.wantPlan, len(tc.members))
		})
	}
}

func readDecisionFields(t *testing.T, ws string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, "triage-decision.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decision is not JSON after processing: %v\n%s", err, raw)
	}
	return fields
}

func assertUnifiedDiagnostics(t *testing.T, diags []core.Diagnostic, wantRejection string) {
	t.Helper()
	if wantRejection == "" {
		if len(diags) != 0 {
			t.Errorf("a valid claim must not warn, got %+v", diags)
		}
		return
	}
	if len(diags) != 1 || diags[0].Severity != "warning" ||
		!strings.Contains(diags[0].Message, "unified_commitment rejected") ||
		!strings.Contains(diags[0].Message, wantRejection) {
		t.Errorf("want one rejection warning naming %q, got %+v", wantRejection, diags)
	}
}

func assertUnifiedProjection(t *testing.T, fields map[string]json.RawMessage, want *unifiedProjection) {
	t.Helper()
	raw, present := fields["unified_projection"]
	if want == nil {
		if present {
			t.Errorf("a rejected claim must leave no unified_projection, got %s", raw)
		}
		return
	}
	if !present {
		t.Fatalf("a validated claim must be projected into the decision")
	}
	var got unifiedProjection
	if err := json.Unmarshal(raw, &got); err != nil || got != *want {
		t.Errorf("unified_projection = %s (err=%v), want %+v", raw, err, *want)
	}
}

func assertTopNPreserved(t *testing.T, fields map[string]json.RawMessage, want []string) {
	t.Helper()
	var top []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(fields["top_n"], &top); err != nil {
		t.Fatalf("top_n unreadable: %v", err)
	}
	got := make([]string, 0, len(top))
	for _, item := range top {
		got = append(got, item.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("top_n = %v, want the independent selection %v preserved", got, want)
	}
}

func assertCampaignPlan(t *testing.T, ws string, want bool, members int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, "campaign-plan.json"))
	if !want {
		if !os.IsNotExist(err) {
			t.Errorf("campaign-plan.json must not survive this branch (err=%v body=%s)", err, raw)
		}
		return
	}
	if err != nil {
		t.Fatalf("a large claim must write campaign-plan.json: %v", err)
	}
	var plan struct {
		Cycles []struct {
			ID string `json:"id"`
		} `json:"cycles"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil || len(plan.Cycles) != members {
		t.Errorf("campaign plan must carry one cycle per member (%d), got %d (err=%v)", members, len(plan.Cycles), err)
	}
}
