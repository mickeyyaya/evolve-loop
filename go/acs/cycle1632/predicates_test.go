//go:build acs

package cycle1632

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

const carryoverLine = "- carryover_summary: "

type tempWorktree struct{ path string }

func (w *tempWorktree) Create(string, int) (string, error) { return w.path, nil }
func (w *tempWorktree) Cleanup(string, string) error       { return nil }

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.email=acs@evolve", "-c", "user.name=acs"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("commit", "-q", "--allow-empty", "-m", "base")
	return dir
}

func routingCfg(stage config.Stage) config.RoutingConfig {
	cfg := config.RoutingConfig{
		Stage:         config.StageEnforce,
		Mode:          config.ModeStaticPreset,
		Mandatory:     []string{"scout", "triage", "build", "audit", "ship"},
		Conditional:   map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers:      map[string]config.RoutingBlock{},
	}
	cfg.PhaseIO = stage
	return cfg
}

type cycleRun struct {
	runners map[core.Phase]core.PhaseRunner
	ledger  *fixtures.FakeLedger
	triage  *fixtures.FakeRunner
}

func runCycle(t *testing.T, stage config.Stage, reqCtx map[string]string, backlog []core.CarryoverTodo) cycleRun {
	t.Helper()
	st := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 0, CarryoverTodos: backlog}}
	led := &fixtures.FakeLedger{}
	runners := fixtures.BuildRunners(nil)
	o := core.NewOrchestrator(st, led, runners,
		core.WithRouting(routingCfg(stage), router.StaticPreset{}),
		core.WithWorktreeProvisioner(&tempWorktree{path: gitRepo(t)}))
	if _, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "c1632", Context: reqCtx,
	}); err != nil {
		t.Fatalf("RunCycle(PhaseIO=%v): %v", stage, err)
	}
	tr := runners[core.PhaseTriage].(*fixtures.FakeRunner)
	if len(tr.Requests) == 0 {
		t.Fatalf("triage was never dispatched at PhaseIO=%v — prompt assertions would be vacuous", stage)
	}
	return cycleRun{runners: runners, ledger: led, triage: tr}
}

func triagePrompt(stage config.Stage, req core.PhaseRequest) string {
	return triage.New(triage.Config{PhaseIO: stage}).ComposePrompt("BODY", req)
}

func overCap(r rune, extra int) string {
	w := len(string(r))
	return strings.Repeat(string(r), (phaseio.MaxFieldBytes+extra+w-1)/w)
}

func TestC1632_001_OversizedFieldIsCappedWithVisibleMarker(t *testing.T) {
	if phaseio.MaxFieldBytes <= 0 || phaseio.MaxFieldBytes > 64*1024 {
		t.Fatalf("MaxFieldBytes=%d: want 0 < cap <= 64KiB (cycle-1593 measured a 218,480-byte carryover; a cap above that bounds nothing)", phaseio.MaxFieldBytes)
	}
	if phaseio.TruncationMarker == "" || !utf8.ValidString(phaseio.TruncationMarker) || len(phaseio.TruncationMarker) >= phaseio.MaxFieldBytes {
		t.Fatalf("TruncationMarker=%q: want non-empty valid UTF-8 shorter than the cap", phaseio.TruncationMarker)
	}
	for _, in := range []string{overCap('x', 218480-phaseio.MaxFieldBytes), overCap('y', 1)} {
		got := phaseio.CapField(in)
		if len(got) > phaseio.MaxFieldBytes {
			t.Errorf("len(CapField(%d bytes))=%d > MaxFieldBytes=%d", len(in), len(got), phaseio.MaxFieldBytes)
		}
		if got == in {
			t.Errorf("CapField passed a %d-byte value through unchanged — cap inert", len(in))
		}
		if !strings.Contains(got, phaseio.TruncationMarker) {
			t.Errorf("capped value carries no visible marker %q", phaseio.TruncationMarker)
		}
		if got == in[:phaseio.MaxFieldBytes] {
			t.Errorf("CapField is a silent prefix cut with no marker")
		}
		if i := strings.Index(got, phaseio.TruncationMarker); i >= 0 && !strings.HasPrefix(in, got[:i]) {
			t.Errorf("retained text is not a prefix of the input (value rewritten, not truncated)")
		}
		if twice := phaseio.CapField(got); twice != got {
			t.Errorf("CapField is not idempotent: %d bytes → %d bytes on a second pass", len(got), len(twice))
		}
	}
	big := overCap('z', 7)
	ci := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: big, PreviousVerdict: big})
	if ci.Carryover() != phaseio.CapField(big) || ci.PreviousVerdict() != phaseio.CapField(big) {
		t.Errorf("NewCycleInputs did not apply CapField to Carryover/PreviousVerdict: carryover=%d previous_verdict=%d bytes, want %d", len(ci.Carryover()), len(ci.PreviousVerdict()), len(phaseio.CapField(big)))
	}
}

func TestC1632_002_EnforceDispatchBoundsCarryoverInRealTriagePrompt(t *testing.T) {
	raw := overCap('r', 218480-phaseio.MaxFieldBytes)
	run := runCycle(t, config.StageEnforce, map[string]string{"goal": "g", "strategy": "s", "carryover_summary": raw}, nil)
	for i, req := range run.triage.Requests {
		if !req.Input.Active() {
			t.Fatalf("triage request[%d]: typed PhaseInput inactive at enforce — the seam under test was not reached", i)
		}
		typed := req.Input.CycleInputs().Carryover()
		if typed != phaseio.CapField(raw) {
			t.Errorf("request[%d]: typed Carryover() (%d bytes) != CapField(raw) (%d bytes) — cap not applied on the dispatch path", i, len(typed), len(phaseio.CapField(raw)))
		}
		prompt := triagePrompt(config.StageEnforce, req)
		if !strings.Contains(prompt, carryoverLine+phaseio.CapField(raw)+"\n") {
			t.Errorf("request[%d]: real triage prompt does not render the bounded carryover line", i)
		}
		if strings.Contains(prompt, raw) {
			t.Errorf("request[%d]: real triage prompt embeds the full %d-byte raw carryover — the cap is inert on the prompt path", i, len(raw))
		}
		if len(prompt) >= len(raw) {
			t.Errorf("request[%d]: triage prompt is %d bytes for a %d-byte carryover — no reduction reached the prompt", i, len(prompt), len(raw))
		}
	}
}

func TestC1632_003_BoundaryAndMultibyteInputsStaySafe(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"empty", ""},
		{"cap-minus-one", strings.Repeat("u", phaseio.MaxFieldBytes-1)},
		{"exact-boundary", strings.Repeat("e", phaseio.MaxFieldBytes)},
	} {
		ci := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: tc.in, PreviousVerdict: tc.in})
		if ci.Carryover() != tc.in || ci.PreviousVerdict() != tc.in || phaseio.CapField(tc.in) != tc.in {
			t.Errorf("%s: at/under-cap value must round-trip byte-identical (len=%d)", tc.name, len(tc.in))
		}
	}
	for _, tc := range []struct {
		name string
		r    rune
	}{{"2-byte", 'é'}, {"3-byte", '日'}, {"4-byte", '😀'}} {
		in := overCap(tc.r, 1)
		got := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: in}).Carryover()
		if len(got) > phaseio.MaxFieldBytes || !utf8.ValidString(got) {
			t.Errorf("%s: capped value len=%d valid=%v — want <= %d and valid UTF-8", tc.name, len(got), utf8.ValidString(got), phaseio.MaxFieldBytes)
		}
		idx := strings.Index(got, phaseio.TruncationMarker)
		if idx < 0 {
			t.Errorf("%s: no marker on a multibyte over-cap value", tc.name)
			continue
		}
		for _, rr := range got[:idx] {
			if rr != tc.r {
				t.Errorf("%s: retained prefix holds rune %U, want only %U — a rune was split at the boundary", tc.name, rr, tc.r)
				break
			}
		}
	}
}

type shadowDoc struct {
	Phase      string `json:"phase"`
	Mismatches []struct {
		Field string `json:"field"`
		Want  string `json:"want"`
		Got   string `json:"got"`
	} `json:"mismatches"`
}

func TestC1632_004_ShadowDispatchEmitsNoFalseMismatchForCappedCarryover(t *testing.T) {
	raw := overCap('s', 218480-phaseio.MaxFieldBytes)
	run := runCycle(t, config.StageShadow, map[string]string{"goal": "g", "strategy": "s", "carryover_summary": raw}, nil)
	req := run.triage.Requests[0]
	if got := phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: req.Context["carryover_summary"]}).Carryover(); len(got) > phaseio.MaxFieldBytes {
		t.Fatalf("NewCycleInputs does not bound the legacy value on this path (%d bytes) — AC1 must land first", len(got))
	}
	path := filepath.Join(req.Workspace, "phaseio-shadow-triage.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("shadow artifact not written at StageShadow: %v", err)
	}
	var doc shadowDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("shadow artifact %s unparsable: %v", path, err)
	}
	for _, m := range doc.Mismatches {
		if m.Field == "cycle_inputs.carryover" {
			t.Errorf("false shadow mismatch on cycle_inputs.carryover for a cap-equivalent value: want=%d bytes got=%d bytes", len(m.Want), len(m.Got))
		}
		if len(m.Want) > phaseio.MaxFieldBytes || len(m.Got) > phaseio.MaxFieldBytes {
			t.Errorf("shadow artifact leaks an uncapped value on %s (want=%d got=%d bytes)", m.Field, len(m.Want), len(m.Got))
		}
	}
	for _, e := range run.ledger.Entries {
		if e.Kind != "phaseio_shadow_mismatch" {
			continue
		}
		if strings.Contains(e.Message, "cycle_inputs.carryover") {
			t.Errorf("ledger phaseio_shadow_mismatch entry names cycle_inputs.carryover for a cap-equivalent value")
		}
		if strings.Contains(e.Message, raw) {
			t.Errorf("ledger phaseio_shadow_mismatch entry embeds the %d-byte raw carryover (the cycle-1593 H1 leak)", len(raw))
		}
	}
}

func TestC1632_005_GenuineDriftStillDetected_BindsCoreTest(t *testing.T) {
	const name = "TestCompareCycleInputsShadow_DetectsRealDrift"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^"+name+"$", corePkg)
	if code < 0 {
		t.Fatalf("go test failed to launch: code=%d err=%v\n%s", code, err, stderr)
	}
	if code != 0 || !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("binding test %s did NOT pass (exit %d)\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
	}
}

func TestC1632_006_NoRawCarryoverWriterBelowEnforce(t *testing.T) {
	backlog := []core.CarryoverTodo{
		{ID: "todo-big-1", Action: overCap('a', 100000), Priority: "high"},
		{ID: "todo-big-2", Action: overCap('b', 100000), Priority: "high"},
	}
	for _, tc := range []struct {
		name  string
		stage config.Stage
	}{{"off", config.StageOff}, {"shadow", config.StageShadow}, {"enforce", config.StageEnforce}} {
		t.Run(tc.name, func(t *testing.T) {
			run := runCycle(t, tc.stage, map[string]string{"goal": "g", "strategy": "s"}, backlog)
			for phase, r := range run.runners {
				for i, req := range r.(*fixtures.FakeRunner).Requests {
					if v, has := req.Context["carryover_summary"]; has {
						t.Errorf("phase %s request[%d]: Context[carryover_summary] minted (%d bytes) — a raw carryover writer exists", phase, i, len(v))
					}
					if tc.stage < config.StageEnforce && !reflect.DeepEqual(req.Input, phaseio.PhaseInput{}) {
						t.Errorf("phase %s request[%d]: PhaseInput not zero below enforce", phase, i)
					}
					if req.Input.Active() && req.Input.CycleInputs().Carryover() != "" {
						t.Errorf("phase %s request[%d]: typed Carryover() non-empty (%d bytes) with no carryover in the request", phase, i, len(req.Input.CycleInputs().Carryover()))
					}
				}
			}
			for i, req := range run.triage.Requests {
				prompt := triagePrompt(tc.stage, req)
				if strings.Contains(prompt, carryoverLine) {
					t.Errorf("request[%d]: real triage prompt renders a carryover line from state backlog — baseline is zero bytes", i)
				}
				for _, todo := range backlog {
					if strings.Contains(prompt, todo.ID) || strings.Contains(prompt, todo.Action[:64]) {
						t.Errorf("request[%d]: real triage prompt embeds backlog todo %s", i, todo.ID)
					}
				}
			}
		})
	}
}

const laneItemID = "tokenopt-handoff-digests"

const thisCycle = 1632

var (
	perEdgeRe   = regexp.MustCompile(`(?i)per[- ](phase[- ])?edge`)
	configRe    = regexp.MustCompile(`(?i)config`)
	insteadOfRe = regexp.MustCompile(`(?i)instead[- ]of`)
	phasesRe    = regexp.MustCompile(`(?i)(ComposePrompt|(every|each|all|remaining|other|six)\s+(phase|prompt))`)
	inboxIDRe   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func sourceRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv(ipcenv.WorktreeRootKey); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func cycleRunDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(stateRoot(t), ".evolve", "runs", fmt.Sprintf("cycle-%d", thisCycle))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("cycle workspace %s absent (runtime state not present): %v", dir, err)
	}
	return dir
}

func refersToLaneItem(it inboxbatch.Item) bool {
	for _, edge := range append(append([]string{}, it.ConnectsTo...), it.Deps...) {
		if strings.Contains(edge, laneItemID) {
			return true
		}
	}
	return false
}

func remainderItems(t *testing.T, src string) (matched []inboxbatch.Item, warnings []string) {
	t.Helper()
	items, warnings, err := inboxbatch.LoadDir(filepath.Join(src, ".evolve", "inbox"))
	if err != nil {
		t.Fatalf("inboxbatch.LoadDir(%s/.evolve/inbox): %v", src, err)
	}
	for _, it := range items {
		if it.ID == laneItemID || !refersToLaneItem(it) {
			continue
		}
		acc := strings.Join(it.Acceptance, "\n")
		var missing []string
		for _, want := range []struct {
			name string
			re   *regexp.Regexp
		}{{"per-edge", perEdgeRe}, {"config", configRe}, {"instead-of", insteadOfRe}, {"remaining ComposePrompt phases", phasesRe}} {
			if !want.re.MatchString(acc) {
				missing = append(missing, want.name)
			}
		}
		if len(missing) > 0 {
			t.Logf("inbox item %s links to %s but its acceptance does not name: %v", it.ID, laneItemID, missing)
			continue
		}
		matched = append(matched, it)
	}
	return matched, warnings
}

func TestC1632_007_BuildReportDoesNotClaimFullClosureOfPartialInboxItem(t *testing.T) {
	runDir := cycleRunDir(t)
	body, err := os.ReadFile(filepath.Join(runDir, "build-report.md"))
	if err != nil {
		t.Fatalf("build-report.md unreadable in %s: %v — the build must produce the report the landing reads", runDir, err)
	}
	closes := inboxmover.ClosesInboxIDs(body)
	for _, id := range closes {
		if id == laneItemID {
			t.Errorf("build-report.md declares `Closes-Inbox: %s` while inbox acceptance[1] (per-edge explicit artifact-flow config) is unimplemented and acceptance[0] is met for one phase, additively — a PASS landing would retire the item with the unmet remainder recorded nowhere (cycle-1604 debb0673…/d7f2448d…, third audit on this lane)", laneItemID)
		}
	}
	report := string(body)
	if !perEdgeRe.MatchString(report) {
		t.Errorf("build-report.md never names the unmet per-edge artifact-flow criterion — the Handoff Summary must state what this landing does NOT deliver")
	}
	matched, _ := remainderItems(t, sourceRoot(t))
	if len(matched) == 0 {
		t.Errorf("build-report.md cannot point at a queued remainder because none exists at the inbox root (see TestC1632_008) — the unmet criteria are recorded nowhere durable")
		return
	}
	for _, rem := range matched {
		if !strings.Contains(report, rem.ID) {
			t.Errorf("build-report.md never names the remainder inbox item %s — the unmet criteria are not traceable from the landing report", rem.ID)
		}
		for _, id := range closes {
			if id == rem.ID {
				t.Errorf("build-report.md declares `Closes-Inbox: %s` for the remainder it just queued", rem.ID)
			}
		}
	}
}

func TestC1632_008_UnmetRemainderIsQueuedAsTrackedInboxItem(t *testing.T) {
	src := sourceRoot(t)
	inboxDir := filepath.Join(src, ".evolve", "inbox")
	matched, warnings := remainderItems(t, src)
	if len(matched) != 1 {
		t.Fatalf("want exactly ONE inbox-root item that links to %s (connects_to/deps) and whose acceptance names per-edge config, instead-of (not additive) and the remaining ComposePrompt phases; got %d", laneItemID, len(matched))
	}
	rem := matched[0]
	if !inboxIDRe.MatchString(rem.ID) {
		t.Errorf("remainder id %q is not a valid inbox id (Closes-Inbox / filename shape)", rem.ID)
	}
	if rem.Title == "" {
		t.Errorf("remainder %s has no title — it renders as an empty Task Contract heading", rem.ID)
	}
	if rem.Weight <= 0 {
		t.Errorf("remainder %s has weight %v — an unweighted item sorts to the bottom of every triage menu and is lost in practice", rem.ID, rem.Weight)
	}
	if rem.Continuation != nil {
		t.Errorf("remainder %s carries a continuation binding — the remainder must start fresh, not resume the FAILed lane's snapshot", rem.ID)
	}
	path, err := inboxmover.FindFileByTaskID(inboxDir, rem.ID)
	if err != nil {
		t.Errorf("inboxmover.FindFileByTaskID(%s, %s): %v — the remainder is not claimable", inboxDir, rem.ID, err)
	} else if filepath.Base(path) != rem.Path {
		t.Errorf("FindFileByTaskID resolved %s to %s, LoadDir loaded it from %s — duplicate id at the inbox root", rem.ID, filepath.Base(path), rem.Path)
	}
	for _, w := range warnings {
		if strings.HasPrefix(w, rem.Path+":") {
			t.Errorf("the loader sanitised the remainder file: %s", w)
		}
	}
	rel := filepath.Join(".evolve", "inbox", rem.Path)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", src, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is on disk but not git-tracked — it would be dropped at ship (cycle-93)", rel)
	}
}

func digestBase() phaseio.HandoffsInit {
	return phaseio.HandoffsInit{
		Scout:    &phaseio.ScoutView{CycleSizeEstimate: "small", ItemCount: 4, CarryoverCount: 2, BacklogSize: 9},
		Triage:   &phaseio.TriageView{CycleSize: "small", PhaseSkip: []string{"tdd"}},
		Build:    &phaseio.BuildView{Verdict: "PASS", ACSGreen: 3, ACSTotal: 3, SeverityMax: "LOW"},
		Audit:    &phaseio.AuditView{Verdict: "FAIL", Confidence: 0.9, RedCount: 2},
		Degraded: []string{"build: handoff-build.json: permission denied"},
		Generic:  map[string]any{"scout.notes": "n"},
	}
}

func compactLines(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if r := []rune(ln); len(r) > width {
			lines[i] = string(r[:width]) + "…(" + fmt.Sprint(len(r)) + " runes)"
		}
	}
	return strings.Join(lines, "\n")
}

func TestC1632_009_UpstreamDigestOversizedScalarCannotEvictOtherSections(t *testing.T) {
	huge := strings.Repeat("x", 4096)
	placements := []struct {
		name string
		put  func(*phaseio.HandoffsInit)
	}{
		{"scout-scalar", func(i *phaseio.HandoffsInit) { i.Scout.CycleSizeEstimate = huge }},
		{"triage-scalar", func(i *phaseio.HandoffsInit) { i.Triage.CycleSize = huge }},
		{"generic-value", func(i *phaseio.HandoffsInit) { i.Generic["scout.notes"] = huge }},
		{"degraded-entry", func(i *phaseio.HandoffsInit) { i.Degraded = []string{"build: handoff-build.json: " + huge} }},
	}
	sections := []string{"scout:", "triage:", "build:", "audit:", "degraded:", "generic "}
	for _, cap := range []int{512, 0} {
		for _, pl := range placements {
			t.Run(fmt.Sprintf("cap%d/%s", cap, pl.name), func(t *testing.T) {
				init := digestBase()
				pl.put(&init)
				got := phaseio.NewHandoffs(init).UpstreamDigest(cap)
				if n := len([]rune(got)); cap > 0 && n > cap {
					t.Errorf("digest is %d runes, cap %d", n, cap)
				}
				if strings.Contains(got, huge) {
					t.Errorf("the %d-rune value is embedded verbatim — no section bound applied", len(huge))
				}
				lines := strings.Split(got, "\n")
				for _, sec := range sections {
					found := false
					for _, ln := range lines {
						if strings.HasPrefix(ln, sec) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("section %q evicted by the oversized %s:\n%s", sec, pl.name, compactLines(got, 72))
					}
				}
				for _, ln := range lines {
					if strings.HasPrefix(ln, "degraded:") && !strings.Contains(ln, "build") {
						t.Errorf("degraded row no longer names its edge (R5 read-miss made anonymous): %q", ln)
					}
				}
			})
		}
	}
}

func fencedDigest(prompt string) string {
	_, after, ok := strings.Cut(prompt, "## Upstream Handoff Digest")
	if !ok {
		return ""
	}
	_, after, ok = strings.Cut(after, "```text\n")
	if !ok {
		return ""
	}
	block, _, ok := strings.Cut(after, "\n```")
	if !ok {
		return ""
	}
	return block
}

func TestC1632_010_TDDPromptDigestCapIsThePackageDefault(t *testing.T) {
	generic := map[string]any{}
	for i := 0; i < 64; i++ {
		generic[fmt.Sprintf("scout.k%02d", i)] = strings.Repeat("v", 40)
	}
	req := core.PhaseRequest{Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
		Phase:    string(core.PhaseTDD),
		Upstream: phaseio.NewHandoffs(phaseio.HandoffsInit{Generic: generic}),
	})}
	if !req.Input.Active() {
		t.Fatalf("fixture PhaseInput inactive — the digest hook would not run")
	}
	prompt := tdd.New(tdd.Config{}).ComposePrompt("BODY", req)
	block := fencedDigest(prompt)
	if block == "" {
		t.Fatalf("tdd's real ComposePrompt rendered no fenced upstream digest block for an active PhaseInput")
	}
	want := req.Input.Upstream().UpstreamDigest(0)
	if block != want {
		t.Errorf("tdd's rendered digest (%d runes) != internal/phaseio's package-default rendering (%d runes) — the phase forks its own cap", len([]rune(block)), len([]rune(want)))
	}
	src, err := os.ReadFile(filepath.Join(sourceRoot(t), "go", "internal", "phases", "tdd", "tdd.go"))
	if err != nil {
		t.Fatalf("read tdd.go: %v", err)
	}
	if m := regexp.MustCompile(`UpstreamDigest\(\s*[1-9][0-9]*`).Find(src); m != nil {
		t.Errorf("tdd.go passes a bare numeric cap %q to UpstreamDigest — pass a non-positive cap so internal/phaseio's default is the single source (cycle-1604 dbd6c43a…/dd88cbdc…)", m)
	}
}

var suiteReceiptRe = regexp.MustCompile(`\[acs suite\] cycle=(\d+) verdict=(\w+) green=(\d+) red=(\d+) skip=(\d+) total=(\d+) \(cycle=(\d+) regression=(\d+) red-team=(\d+)\)`)

func declaredCyclePredicates(t *testing.T) []string {
	t.Helper()
	const self = "github.com/mickeyyaya/evolve-loop/go/acs/cycle1632"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-tags", "acs", "-list", "^TestC1632_", self)
	if code != 0 {
		t.Fatalf("go test -list %s: code=%d err=%v\n%s%s", self, code, err, stdout, stderr)
	}
	var names []string
	for _, line := range strings.Split(stdout, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "TestC1632_") {
			names = append(names, line)
		}
	}
	if len(names) == 0 {
		t.Fatalf("go test -list found no TestC1632_ predicate in %s:\n%s", self, stdout)
	}
	return names
}

func TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound(t *testing.T) {
	runDir := cycleRunDir(t)
	body, err := os.ReadFile(filepath.Join(runDir, "build-report.md"))
	if err != nil {
		t.Fatalf("build-report.md unreadable in %s: %v — the build must produce the report the landing reads", runDir, err)
	}
	declared := declaredCyclePredicates(t)
	receipts := suiteReceiptRe.FindAllStringSubmatch(string(body), -1)
	if len(receipts) == 0 {
		t.Fatalf("build-report.md carries no `[acs suite] cycle=… (cycle=N …)` receipt — paste the line `evolve acs suite --cycle %d --root <worktree> --evolve-dir <runtime>/.evolve` prints for THIS tree", thisCycle)
	}
	for _, m := range receipts {
		cycle, _ := strconv.Atoi(m[1])
		verdict := m[2]
		red, _ := strconv.Atoi(m[4])
		rows, _ := strconv.Atoi(m[7])
		if cycle != thisCycle {
			t.Errorf("receipt %q names cycle %d, not %d — a foreign cycle's suite run cannot back this build", m[0], cycle, thisCycle)
		}
		if verdict != "PASS" || red != 0 {
			t.Errorf("receipt %q reports verdict=%s red=%d — a `Status: PASS` cannot rest on a red suite; fix the reds and re-run", m[0], verdict, red)
		}
		if rows < len(declared) {
			t.Errorf("stale receipt %q: %d this-cycle row(s), but %d TestC1632_ predicates are declared on this tree (%s) — the suite was not re-run after they landed (round-2 claim-discrepancy); replace it with the fresh line", m[0], rows, len(declared), strings.Join(declared, ", "))
		}
	}
}
