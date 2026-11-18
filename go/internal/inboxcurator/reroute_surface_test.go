package inboxcurator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
)

const hiddenProtectedFile = "go/internal/guards/ship.go"

func itemFromJSON(t *testing.T, raw string) inboxbatch.Item {
	t.Helper()
	var it inboxbatch.Item
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		t.Fatalf("item %s: %v", raw, err)
	}
	return it
}

func onlyHiddenFileIsProtected(p string) bool { return p == hiddenProtectedFile }

func rerouted(proposals []Proposal) []string {
	var ids []string
	for _, p := range proposals {
		if p.Kind == ProposeReroute {
			ids = append(ids, p.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func TestReroutes_AnItemWhoseTextNamesAProtectedFileGetsNoRerouteProposal(t *testing.T) {
	declared := `"kind":"bug","route":"console-manual","files":["go/internal/fleet/pool.go"]`
	items := []inboxbatch.Item{
		itemFromJSON(t, `{"id":"hidden-in-summary",`+declared+`,"summary":"The fix also reaches go/internal/guards/ship.go."}`),
		itemFromJSON(t, `{"id":"hidden-in-fix",`+declared+`,"summary":"s","fix":"Change go/internal/guards/ship.go to refuse it."}`),
		itemFromJSON(t, `{"id":"hidden-in-acceptance",`+declared+`,"summary":"s","acceptance":["go/internal/guards/ship.go refuses the push"]}`),
		itemFromJSON(t, `{"id":"names-an-unprotected-file",`+declared+`,"summary":"The fix also reaches go/internal/fleet/queue.go."}`),
		itemFromJSON(t, `{"id":"stamp-only","kind":"bug","route":"console-manual","routed_by":"loop","routed_reason":"lane triage (cycle 7) dropped: protected-surface: go/internal/guards/ship.go","files":["go/internal/fleet/pool.go"],"summary":"s"}`),
	}

	got := rerouted(Reroutes(items, RerouteOptions{IsProtected: onlyHiddenFileIsProtected, BuildableKinds: []string{"bug"}}))

	want := []string{"names-an-unprotected-file", "stamp-only"}
	if !slices.Equal(got, want) {
		t.Errorf("rerouted = %v, want %v: text that names a protected file blocks the reroute, a stamp of an earlier verdict does not", got, want)
	}
}

func TestReroutes_TheFixSurfaceHookRefusesAProtectedFileTheItemNeverNames(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "reaches-gate", Kind: "bug", Route: "console-manual", Files: []string{"go/internal/fleet/pool.go"}},
		{ID: "stays-free", Kind: "bug", Route: "console-manual", Files: []string{"go/internal/fleet/pool.go"}},
	}

	got := rerouted(Reroutes(items, RerouteOptions{
		IsProtected:    onlyHiddenFileIsProtected,
		BuildableKinds: []string{"bug"},
		FixSurface: func(it inboxbatch.Item) []string {
			if it.ID == "reaches-gate" {
				return []string{hiddenProtectedFile}
			}
			return []string{"go/internal/fleet/queue.go"}
		},
	}))

	if !slices.Equal(got, []string{"stays-free"}) {
		t.Errorf("rerouted = %v, want only stays-free", got)
	}
}

func TestReroutes_TheStaleHookStopsTheRerouteOfAnItemWhoseFixIsAlreadyOnMain(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "stale", Kind: "bug", Route: "console-manual", Files: []string{"go/internal/fleet/pool.go"}},
		{ID: "fresh", Kind: "bug", Route: "console-manual", Files: []string{"go/internal/fleet/pool.go"}},
	}

	got := rerouted(Reroutes(items, RerouteOptions{
		IsProtected:    func(string) bool { return false },
		BuildableKinds: []string{"bug"},
		Stale: func(it inboxbatch.Item) string {
			if it.ID == "stale" {
				return "the fix is on main"
			}
			return ""
		},
	}))

	if !slices.Equal(got, []string{"fresh"}) {
		t.Errorf("rerouted = %v, want only fresh", got)
	}
}

func curateInputs(open ...inboxbatch.Item) Inputs {
	return Inputs{
		Open:                  open,
		IsProtected:           func(string) bool { return false },
		RerouteBuildableKinds: []string{"bug"},
		Backstop:              true,
		HasTest:               func(string) bool { return false },
		PathExists:            func(string) bool { return false },
		Thresholds:            Thresholds{Refuse: 0.99, Warn: 0.99},
		MinShare:              0.5,
	}
}

func proposalsFor(proposals []Proposal, id string) []Proposal {
	var out []Proposal
	for _, p := range proposals {
		if p.ID == id {
			out = append(out, p)
		}
	}
	return out
}

func TestCurate_AnItemWhoseProbesPassOnTheTreeGetsAConsumeAndNoReroute(t *testing.T) {
	probed := itemFromJSON(t, `{"id":"done","kind":"bug","route":"console-manual","files":["go/internal/fleet/pool.go"],"summary":"s","probes":[{"kind":"path-exists","arg":"go/internal/fleet/pool.go"}]}`)
	open := itemFromJSON(t, `{"id":"open","kind":"bug","route":"console-manual","files":["go/internal/fleet/queue.go"],"summary":"s"}`)
	in := curateInputs(probed, open)
	in.PathExists = func(p string) bool { return p == "go/internal/fleet/pool.go" }

	got := Curate(in)

	if !slices.Equal(rerouted(got), []string{"open"}) {
		t.Errorf("rerouted = %v, want only open: an item whose probes pass is done and must not move to a lane", rerouted(got))
	}
	done := proposalsFor(got, "done")
	if len(done) != 1 || done[0].Kind != ProposeConsume {
		t.Errorf("proposals for done = %+v, want one consume", done)
	}
}

func TestCurate_AnItemTheStaleHookNamesGetsAJudgmentProposalAndNoReroute(t *testing.T) {
	const reason = "go/internal/bridge/driver_tmux_submitverify.go already enforces submitVerifyMinEchoRunes = 8"
	stale := itemFromJSON(t, `{"id":"stale","kind":"bug","route":"console-manual","files":["go/internal/fleet/pool.go"],"summary":"s"}`)
	fresh := itemFromJSON(t, `{"id":"fresh","kind":"bug","route":"console-manual","files":["go/internal/fleet/queue.go"],"summary":"s"}`)
	in := curateInputs(stale, fresh)
	in.Stale = func(it inboxbatch.Item) string {
		if it.ID == "stale" {
			return reason
		}
		return ""
	}

	got := Curate(in)

	if !slices.Equal(rerouted(got), []string{"fresh"}) {
		t.Errorf("rerouted = %v, want only fresh", rerouted(got))
	}
	staleProposals := proposalsFor(got, "stale")
	if len(staleProposals) != 1 {
		t.Fatalf("proposals for stale = %+v, want exactly one", staleProposals)
	}
	p := staleProposals[0]
	if p.Kind != ProposeConsume && p.Kind != ProposeQuarantine {
		t.Errorf("kind = %q, want consume or quarantine", p.Kind)
	}
	if p.Deterministic {
		t.Error("a stale claim from a hook is a judgment: the curator must not apply it by itself")
	}
	if !slices.ContainsFunc(p.Evidence, func(e string) bool { return strings.Contains(e, reason) }) {
		t.Errorf("evidence = %v, want it to carry the stale reason", p.Evidence)
	}
}

const wave178Dir = "testdata/wave178"

var wave179FixSurface = map[string][]string{
	"resumed-build-abort-skips-the-retrospective-and-the-correction": {"go/internal/core/resume_execution.go"},
	"agensh-ship-landed-outbox":                                      {"go/internal/phases/ship/landing/intent.go"},
	"phaseidentity-neutral-leaf":                                     {"go/internal/bridge/driver_tmux_prepare.go"},
	"history-refusal-names-the-evolve-comments-command-lane":         {"go/internal/commitgate/history_recorded.go"},
}

var wave179Stale = map[string]string{
	"tmux-submit-verify-eight-rune-floor":              "go/internal/bridge/driver_tmux_submitverify.go already enforces submitVerifyMinEchoRunes = 8",
	"main-red-stop-provable-means-a-lane-can-prove-it": "go/internal/phases/ship/gate_step_runner.go and main_red_ci.go are absent at HEAD; PR #1524 is not merged",
	"gate-child-env-allowlist":                         "go/internal/gaterun/ is absent at HEAD; the package is only on unmerged commits",
}

func loadWave178(t *testing.T, set string) []inboxbatch.Item {
	t.Helper()
	scan, err := inboxbatch.ScanDir(filepath.Join(wave178Dir, set))
	if err != nil || len(scan.Warnings) != 0 {
		t.Fatalf("scan %s: %v %v", set, err, scan.Warnings)
	}
	return scan.Items
}

func idsOf(items []inboxbatch.Item) []string {
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	return ids
}

func TestCurate_TheWave178BoundaryProposesTheNineLaneTriageKeptAndNoneOfTheSevenItBounced(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "profiles", "builder.json")); err != nil {
		t.Fatalf("the builder profile that the lane fence reads: %v", err)
	}
	fence, err := lanerouting.Forbidden(root, "builder")
	if err != nil {
		t.Fatal(err)
	}
	kept, bounced := loadWave178(t, "kept"), loadWave178(t, "bounced")
	if len(kept) != 9 || len(bounced) != 7 {
		t.Fatalf("fixture = %d kept and %d bounced items, want 9 and 7", len(kept), len(bounced))
	}
	in := curateInputs(slices.Concat(kept, bounced)...)
	in.IsProtected = fence
	in.RerouteBuildableKinds = []string{"bug", "fix", "hardening", "pipeline-repair", "refactor", "security", "techdebt", "test"}
	in.FixSurface = func(it inboxbatch.Item) []string { return wave179FixSurface[it.ID] }
	in.Stale = func(it inboxbatch.Item) string { return wave179Stale[it.ID] }

	got := rerouted(Curate(in))

	if want := idsOf(kept); !slices.Equal(got, want) {
		t.Errorf("rerouted = %v\nwant the 9 that lane triage kept = %v", got, want)
	}
	for _, it := range bounced {
		if slices.Contains(got, it.ID) {
			t.Errorf("%s was bounced by lane triage and must get no reroute proposal", it.ID)
		}
	}
	if len(wave179FixSurface)+len(wave179Stale) != len(bounced) {
		t.Errorf("the fact tables hold %d+%d entries, want one per bounced item (%d)", len(wave179FixSurface), len(wave179Stale), len(bounced))
	}
}
