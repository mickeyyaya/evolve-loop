package triage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const hostChainCycle = 7

type hostChain struct {
	forbidden func(string) bool
	decision  string
	report    string
	item      string
}

func (c hostChain) run(t *testing.T) (ws string, res deliverable.Result) {
	t.Helper()
	root := t.TempDir()
	ws = filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(hostChainCycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, root, "2026-01-01T00-00-00Z-x.json", c.item)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: hostChainCycle, RunID: "r", ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"triage-decision.json": c.decision, "triage-report.md": c.report} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	claim := func(dir string, cycle int, ids []string) error {
		return inboxmover.ClaimPending(inboxmover.Options{InboxDir: dir, Stderr: io.Discard, IsProtectedPath: c.forbidden}, cycle, ids)
	}
	in := core.ReviewInput{Cycle: hostChainCycle, Phase: "triage", Workspace: ws, ProjectRoot: root}
	fx := deliverable.NewHostEffects(cat, claim, c.forbidden)
	resolver := phasecontract.NewCatalogResolver(cat.Get)
	if err := fx.Perform(context.Background(), in); err != nil {
		t.Fatalf("host effects before the settle verify: %v", err)
	}
	if _, err = deliverable.VerifyWithStage("triage", deliverable.RootsFor(in), resolver, config.StageEnforce); err != nil {
		t.Fatal(err)
	}
	if verdict, _, _ := (hooks{forbidden: c.forbidden}).Classify(c.report, core.PhaseRequest{Cycle: hostChainCycle, ProjectRoot: root, Workspace: ws}, core.BridgeResponse{}); verdict != core.VerdictPASS {
		t.Fatalf("classify verdict %s", verdict)
	}
	if err := fx.Perform(context.Background(), in); err != nil {
		t.Fatalf("host effects before the review: %v", err)
	}
	res, err = deliverable.VerifyWithStage("triage", deliverable.RootsFor(in), resolver, config.StageEnforce)
	if err != nil {
		t.Fatal(err)
	}
	return ws, res
}

func TestHostChain_ARoutedPinnedItemPassesTheGateAndNothingIsLeftToBuild(t *testing.T) {
	t.Parallel()
	cases := map[string]hostChain{
		"route on the item, committed in top_n": {
			decision: `{"cycle":` + strconv.Itoa(hostChainCycle) + `,"top_n":[{"id":"x"}],"deferred":[]}`,
			report:   "# Triage Decision\n\n## top_n\n- x: fixture — priority=H, source={scout}\n",
			item:     `{"id":"x","title":"fixture","weight":0.9,"route":"console-only"}`,
		},
		"protected files on the item, empty top_n": {
			forbidden: func(p string) bool { return p == "go/internal/guards/role.go" },
			decision:  `{"cycle":` + strconv.Itoa(hostChainCycle) + `,"top_n":[],"deferred":[]}`,
			report:    "# Triage Decision\n\n## top_n\n",
			item:      `{"id":"x","title":"fixture","weight":0.9,"files":["go/internal/guards/role.go"]}`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws, res := c.run(t)
			if !res.OK {
				t.Fatalf("the gate must accept an item the host routed: %+v", res.Violations)
			}
			if got, ok := committedset.Committed(ws); !ok || len(got) != 0 {
				t.Fatalf("a lane never builds an operator-owned item; committed=%v", got)
			}
		})
	}
}

func TestHostChain_AnUnroutedPinnedItemTheHostClaimsPassesTheGate(t *testing.T) {
	t.Parallel()
	c := hostChain{
		decision: `{"cycle":` + strconv.Itoa(hostChainCycle) + `,"top_n":[{"id":"x"}],"deferred":[]}`,
		report:   "# Triage Decision\n\n## top_n\n- x: fixture — priority=H, source={scout}\n",
		item:     `{"id":"x","title":"fixture","weight":0.9}`,
	}
	ws, res := c.run(t)
	if !res.OK {
		t.Fatalf("%+v", res.Violations)
	}
	if got, _ := committedset.Committed(ws); len(got) != 1 {
		t.Fatalf("an unrouted item stays committed: %v", got)
	}
}
