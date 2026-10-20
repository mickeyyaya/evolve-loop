package triagecap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestAutoFileDemotionDefect_ConcurrentCallsLeaveOneCompleteParseableFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	const writers = 16
	errs := make([]error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = autoFileDemotionDefect(root, 303, 301, 302, "identical rejection template", RemedyPending)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("writer %d: %v", i, err)
		}
	}
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("the inbox must hold one defect file and no temp file, got %v", names)
	}
	raw, err := os.ReadFile(demotionDefectPath(root, 301, 302))
	if err != nil {
		t.Fatal(err)
	}
	var rec DemotionLedgerRecord
	if err := json.Unmarshal(raw, &rec); err != nil || rec.ID != "auto-heuristic-demotion-triagecap-c301-c302" || rec.RelievedCycle != 303 {
		t.Fatalf("the defect file must be one complete record: err=%v rec=%+v", err, rec)
	}
}

func TestAutoFileDemotionDefect_ReportsAFailedWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := autoFileDemotionDefect(root, 303, 301, 302, "why", RemedyPending)

	if err == nil || !strings.Contains(err.Error(), "auto-heuristic-demotion-triagecap-c301-c302.json") {
		t.Fatalf("a failed write must be an error that names the defect file, got %v", err)
	}
}

func TestCapReviewer_AnUnwrittenDemotionDefectIsACodedWarn(t *testing.T) {
	t.Parallel()
	pair := []FailEntry{{Cycle: 301, Summary: summary301}, {Cycle: 302, Summary: summary302}}
	r, in, root := newDemotionFixture(t, "303", pair)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.Remove(inbox); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	in.Signals = center

	res := r.Review(context.Background(), in)

	if !res.Approve {
		t.Fatalf("the demotion still relieves the cycle when its defect file cannot be written: %s", res.Reason)
	}
	requireCodedWarn(t, center, CodeDemotionDefectUnwritten, "auto-heuristic-demotion-triagecap-c301-c302.json")
}

func TestCapReviewer_AMalformedCompanionIsACodedWarn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
	}{
		{"committed", `{"committed_floors":"bridge"}`, "committed_floors malformed"},
		{"deferred", `{"deferred_floors":7}`, "deferred_floors malformed"},
		{"invalid json", `{"committed_floors":`, "committed_floors companion malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, in, _ := newDemotionFixture(t, "", nil)
			if err := os.WriteFile(filepath.Join(in.Workspace, TriageDecisionName()), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			center := signalcenter.New()
			in.Signals = center

			_ = r.Review(context.Background(), in)

			requireCodedWarn(t, center, CodeCompanionMalformed, tc.want)
			if n := countCode(center, CodeCompanionMalformed); n != 1 {
				t.Fatalf("one malformed companion must give one WARN, got %d", n)
			}
		})
	}
}

func TestCapReviewer_AWellFormedCompanionEmitsNoMalformedWarn(t *testing.T) {
	t.Parallel()
	r, in, _ := newDemotionFixture(t, "", nil)
	if err := os.WriteFile(filepath.Join(in.Workspace, TriageDecisionName()), []byte(`{"committed_floors":["bridge"],"deferred_floors":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	in.Signals = center

	_ = r.Review(context.Background(), in)

	if n := countCode(center, CodeCompanionMalformed); n != 0 {
		t.Fatalf("a well-formed companion must not emit %s, got %d", CodeCompanionMalformed, n)
	}
}

func TestReadWindow_ACancelledContextReadsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeStateJSON(t, root, `{"triageThroughput":[{"cycle":290,"floors":3}]}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got := readWindow(ctx, root); got != nil {
		t.Fatalf("a cancelled context must read no window, got %+v", got)
	}
}

func requireCodedWarn(t *testing.T, center *signalcenter.Center, code signalcenter.Code, reasonPart string) {
	t.Helper()
	center.Flush()
	for _, e := range center.Recent() {
		if e.Code == code && e.Severity == signalcenter.SeverityWarn && e.Module == signalcenter.ModuleTriage && strings.Contains(e.Reason, reasonPart) {
			return
		}
	}
	t.Fatalf("want a %s WARN whose reason names %q, got %+v", code, reasonPart, center.Recent())
}

func TestReadWindow_AnUnreadableStateReadsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeStateJSON(t, root, `{"triageThroughput":`)

	if got := readWindow(context.Background(), root); got != nil {
		t.Fatalf("a malformed state.json must read no window, got %+v", got)
	}
}

func countCode(center *signalcenter.Center, code signalcenter.Code) int {
	center.Flush()
	n := 0
	for _, e := range center.Recent() {
		if e.Code == code {
			n++
		}
	}
	return n
}
