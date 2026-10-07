package rankinputs_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

var loadNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func TestLoad_ReadsThePolicysBlockAndTheLedgersItemCounts(t *testing.T) {
	evolveDir := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"inbox_priority":{"class_order":["security","correctness"]}}`)
	fixtures.MustWrite(t, filepath.Join(evolveDir, "recurrence-ledger.json"), `{"entries":{"flaky-gate":{"pattern":"flaky-gate","count":4,"fix_item_id":"fix-the-gate"}}}`)
	in, warnings := rankinputs.Load(evolveDir, loadNow)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if !reflect.DeepEqual(in.Config.ClassOrder, []string{"security", "correctness"}) || !in.Now.Equal(loadNow) {
		t.Errorf("the policy's block and the clock pass through: %+v", in)
	}
	if in.Recurrence["fix-the-gate"] != 4 || in.Recurrence["flaky-gate"] != 4 {
		t.Errorf("the ledger's item counts are the recurrence input: %v", in.Recurrence)
	}
}

func TestLoad_AbsentFilesAreTheCompiledDefaultAndNoCounts(t *testing.T) {
	in, warnings := rankinputs.Load(t.TempDir(), loadNow)
	if len(warnings) != 0 || len(in.Recurrence) != 0 {
		t.Fatalf("an absent policy and ledger are not problems: %v %v", warnings, in.Recurrence)
	}
	if !reflect.DeepEqual(in.Config, policy.Policy{}.InboxPriorityConfig()) {
		t.Errorf("the config is the compiled default: %+v", in.Config)
	}
}

func TestLoad_AMalformedPolicyRanksWithTheCompiledDefaultAndSaysSo(t *testing.T) {
	evolveDir := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"inbox_priority":{"class_order":[]}}`)
	in, warnings := rankinputs.Load(evolveDir, loadNow)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "policy unreadable") || !strings.Contains(warnings[0], "compiled inbox_priority default") {
		t.Fatalf("warnings = %v", warnings)
	}
	if !reflect.DeepEqual(in.Config, policy.Policy{}.InboxPriorityConfig()) {
		t.Errorf("the config falls back to the compiled default: %+v", in.Config)
	}
}

func TestLoad_AMalformedLedgerCountsNothingAndSaysSo(t *testing.T) {
	evolveDir := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(evolveDir, "recurrence-ledger.json"), `{not json`)
	in, warnings := rankinputs.Load(evolveDir, loadNow)
	want := "recurrence ledger unreadable"
	if len(warnings) != 1 || !strings.Contains(warnings[0], want) || !strings.HasSuffix(warnings[0], "the recurrence factor is 0 for every item") {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(in.Recurrence) != 0 {
		t.Errorf("no counts: %v", in.Recurrence)
	}
}
