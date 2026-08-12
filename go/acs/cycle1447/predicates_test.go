//go:build acs

// Package cycle1447 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane:
//
//	fill-verdict-correlation-report-land — the CONTINUATION of cycle-1402.
//
// Cycle-1402 authored the whole feature and then died at ship on the
// repo-contract scanner pack ("[REPO_CONTRACT_GATE/precondition @atomic-ship]
// … RED in the lane worktree"), off a base that is now four releases stale.
// Predicates 001–005 are cycle-1402's contract carried forward verbatim (the
// package was renamed 1402→1447 so `evolve acs suite --cycle 1447` binds it —
// a predicate package the current cycle does not name is a package the gate
// never runs). Predicate 006 is NEW and pins the thing that actually killed
// the prior ship: the scanner pack must be GREEN in THIS lane.
//
//	Feature context — part (3) of inbox item
//	`context-fill-telemetry-and-cap` (P1, weight 0.89). Parts (1) and (2) —
//	the per-phase fill-ratio derivation (internal/contextfill) and its durable
//	wiring onto phasetiming.Entry.ContextFillRatio — already landed in
//	cycle-1271. What does NOT exist is the join: nothing anywhere correlates
//	fill% at dispatch against the cycle's final verdict, which is the evidence
//	that promotes or demotes the whole tokenopt band.
//
// The contract Builder must satisfy, pinned by the predicates below:
//
//   - a new leaf package go/internal/contextfillcorrelate exposing a PURE
//     join/bucket function Correlate([]CycleFill) Report plus a Load() that
//     reads the real corpus (knowledge-base/cycles/cycle-*.json dossiers for
//     final_verdict, .evolve/runs/*/phase-timing.json for ContextFillRatio);
//   - a CLI subcommand `evolve context-fill correlate` reachable from the
//     top-level dispatch table (registry.go), emitting --json to stdout and a
//     markdown artifact via --out;
//   - explicit "no data" handling: a cycle with no usable fill ratio or no
//     verdict is reported in NoData, NEVER bucketed as a fabricated 0.0;
//   - ADR-0069 new-package graduation for the new package (enrolment in
//     go/.apicover-enforce plus an apicover_named_test.go that names and
//     exercises every exported symbol).
//
// Predicate strategy — every load-bearing assertion EXERCISES the system under
// test (the cycle-85 degenerate-predicate ban):
//
//   - 001 calls Correlate directly on a crafted fixture and asserts the
//     bucketing arithmetic and the high-fill/low-fill FAIL-rate ordering.
//   - 002 is the negative/edge predicate: missing fill data and a missing
//     verdict must land in NoData and be absent from every bucket, and an
//     all-PASS corpus must yield finite 0 rates (never NaN from a 0/0).
//   - 003 is the CLI WIRING PROOF: it builds the real `evolve` binary from this
//     worktree and invokes `evolve context-fill correlate` through top-level
//     dispatch against a synthetic project root, asserting the emitted JSON and
//     the emitted markdown artifact. A seam whose only caller is a unit test
//     would leave this predicate RED.
//   - 004 runs the same binary against the REAL repo corpus and asserts no
//     silent drops: joined + no-data must account for every dossier on disk.
//   - 005 is the ADR-0069 graduation predicate: it runs the new package's
//     apicover named test, so an unenrolled or unnamed export stays RED.
//   - 006 is this cycle's own criterion: it RUNS each of the four repo-contract
//     scanner suites the ship gate runs, one named package per invocation, in
//     this lane's module dir — the gate's own precondition, proved in-lane
//     before ship rather than discovered at ship for a second time.
//
// Bucket boundary note: the hot bucket's lower bound is asserted against
// contextfill.HotThreshold rather than a re-declared 0.85 literal, so the
// inclusive hot boundary keeps exactly one definition in the tree.
package cycle1447

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/contextfillcorrelate"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// Shared harness: one `evolve` build for the whole suite.
// ---------------------------------------------------------------------------

var (
	buildOnce sync.Once
	builtBin  string
	buildLog  string
)

// evolveBin builds the evolve binary from THIS worktree's source and returns
// its path. Building (rather than reusing an installed binary) is what makes
// predicates 003–004 a wiring proof of the code under review instead of of
// whatever happens to be on PATH.
func evolveBin(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1447-bin-")
		if err != nil {
			buildLog = "mkdtemp: " + err.Error()
			return
		}
		out := filepath.Join(dir, "evolve")
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "-C", filepath.Join(root, "go"), "build", "-o", out, "./cmd/evolve")
		if err != nil || code != 0 {
			buildLog = "go build ./cmd/evolve exited " + itoa(code) + "\nstdout:\n" + stdout + "\nstderr:\n" + stderr
			return
		}
		builtBin = out
	})
	if builtBin == "" {
		t.Fatalf("could not build the evolve binary from this worktree: %s", buildLog)
	}
	return builtBin
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// writeFile writes content under root, creating parents. Fixture helper.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// dossierJSON is a minimal cycle dossier carrying the field the join needs.
func dossierJSON(cycle int, verdict string) string {
	return `{"cycle":` + itoa(cycle) + `,"run_id":"FIXTURE","goal":"fixture","final_verdict":"` + verdict + `","phases":[]}`
}

// timingJSON is a minimal phase-timing log carrying context_fill_ratio.
func timingJSON(phase string, ratio float64) string {
	b, _ := json.Marshal([]map[string]any{{
		"phase":              phase,
		"duration_ms":        1,
		"verdict":            "PASS",
		"attempt_count":      1,
		"resolved_model":     "deep",
		"context_fill_ratio": ratio,
	}})
	return string(b)
}

// bucketFor returns the report bucket whose [Min,Max) range contains ratio.
func bucketFor(t *testing.T, r contextfillcorrelate.Report, ratio float64) contextfillcorrelate.Bucket {
	t.Helper()
	for _, b := range r.Buckets {
		if ratio >= b.Min && (ratio < b.Max || math.IsInf(b.Max, 1)) {
			return b
		}
	}
	t.Fatalf("no bucket covers fill ratio %v; buckets=%+v", ratio, r.Buckets)
	return contextfillcorrelate.Bucket{}
}

// ---------------------------------------------------------------------------
// 001 — the join buckets by peak fill and high fill shows the higher FAIL rate.
// ---------------------------------------------------------------------------

// TestC1447_001_correlate_buckets_fill_against_verdict exercises the pure join
// on a crafted corpus: three low-fill cycles (1 of 3 FAIL) and three high-fill
// cycles (2 of 3 FAIL). It asserts the exact per-bucket arithmetic, the
// hot/cold split derived through contextfill.HotThreshold, and the ordering
// claim the whole report exists to answer — high fill correlates with FAIL.
func TestC1447_001_correlate_buckets_fill_against_verdict(t *testing.T) {
	in := []contextfillcorrelate.CycleFill{
		{Cycle: 101, Verdict: "PASS", Fills: []float64{0.10, 0.20}},
		{Cycle: 102, Verdict: "PASS", Fills: []float64{0.30}},
		{Cycle: 103, Verdict: "FAIL", Fills: []float64{0.05, 0.40}},
		{Cycle: 201, Verdict: "FAIL", Fills: []float64{0.90}},
		{Cycle: 202, Verdict: "FAIL", Fills: []float64{0.20, 0.95}},
		{Cycle: 203, Verdict: "PASS", Fills: []float64{0.88}},
	}

	got := contextfillcorrelate.Correlate(in)

	if got.CyclesJoined != 6 {
		t.Errorf("CyclesJoined = %d, want 6 (every fixture cycle has both fill data and a verdict)", got.CyclesJoined)
	}
	if len(got.NoData) != 0 {
		t.Errorf("NoData = %v, want empty — every fixture cycle is joinable", got.NoData)
	}

	// Buckets must be ascending and the top bucket must start at the ONE
	// canonical hot boundary, not a re-declared literal.
	if len(got.Buckets) < 2 {
		t.Fatalf("Buckets = %+v, want at least a low and a hot bucket", got.Buckets)
	}
	for i := 1; i < len(got.Buckets); i++ {
		if got.Buckets[i].Min <= got.Buckets[i-1].Min {
			t.Errorf("Buckets not ascending at %d: %+v", i, got.Buckets)
		}
	}
	top := got.Buckets[len(got.Buckets)-1]
	if math.Abs(top.Min-contextfill.HotThreshold) > 1e-9 {
		t.Errorf("top bucket Min = %v, want contextfill.HotThreshold (%v) so the hot boundary has one definition",
			top.Min, contextfill.HotThreshold)
	}

	// Cycles bucket by PEAK per-phase fill: 0.20/0.30/0.40 are cold, and
	// 0.90/0.95/0.88 all land in the hot bucket.
	if top.Cycles != 3 || top.Fail != 2 {
		t.Errorf("hot bucket = {Cycles:%d Fail:%d}, want {3 2}", top.Cycles, top.Fail)
	}
	if math.Abs(top.FailRate-2.0/3.0) > 1e-9 {
		t.Errorf("hot bucket FailRate = %v, want 2/3", top.FailRate)
	}

	low := bucketFor(t, got, 0.30)
	if low.Cycles != 3 || low.Fail != 1 {
		t.Errorf("low bucket = {Cycles:%d Fail:%d}, want {3 1}", low.Cycles, low.Fail)
	}
	if math.Abs(low.FailRate-1.0/3.0) > 1e-9 {
		t.Errorf("low bucket FailRate = %v, want 1/3", low.FailRate)
	}
	if !(top.FailRate > low.FailRate) {
		t.Errorf("high-fill FailRate (%v) must exceed low-fill FailRate (%v) on this fixture", top.FailRate, low.FailRate)
	}

	// Hot/cold split is the same evidence keyed off contextfill.IsHot.
	if got.Hot.Cycles != 3 || got.Hot.Fail != 2 {
		t.Errorf("Hot = %+v, want {Cycles:3 Fail:2}", got.Hot)
	}
	if got.Cold.Cycles != 3 || got.Cold.Fail != 1 {
		t.Errorf("Cold = %+v, want {Cycles:3 Fail:1}", got.Cold)
	}

	// Bucket counts must account for exactly the joined cycles.
	sum := 0
	for _, b := range got.Buckets {
		sum += b.Cycles
	}
	if sum != got.CyclesJoined {
		t.Errorf("bucket cycle counts sum to %d, want CyclesJoined=%d — a cycle was dropped or double-counted", sum, got.CyclesJoined)
	}
}

// ---------------------------------------------------------------------------
// 002 — negative/edge: absent data is reported, never fabricated as 0.0.
// ---------------------------------------------------------------------------

// TestC1447_002_missing_data_never_silently_zero is the anti-no-op predicate.
// An implementation that defaults a missing fill ratio or a missing verdict to
// zero would quietly poison the lowest bucket and invent a correlation. This
// asserts both are excluded into NoData, and that an all-PASS corpus produces
// finite zero rates rather than NaN from an empty-bucket 0/0.
func TestC1447_002_missing_data_never_silently_zero(t *testing.T) {
	in := []contextfillcorrelate.CycleFill{
		{Cycle: 301, Verdict: "PASS", Fills: nil},                  // no fill data at all
		{Cycle: 302, Verdict: "PASS", Fills: []float64{}},          // empty, same class
		{Cycle: 303, Verdict: "", Fills: []float64{0.42}},          // fill but no verdict
		{Cycle: 304, Verdict: "PASS", Fills: []float64{0.10, 0.2}}, // the one joinable cycle
	}

	got := contextfillcorrelate.Correlate(in)

	if got.CyclesJoined != 1 {
		t.Errorf("CyclesJoined = %d, want 1 — only cycle 304 has both fill data and a verdict", got.CyclesJoined)
	}
	wantNoData := map[int]bool{301: true, 302: true, 303: true}
	if len(got.NoData) != len(wantNoData) {
		t.Errorf("NoData = %v, want exactly the three unjoinable cycles %v", got.NoData, []int{301, 302, 303})
	}
	for _, c := range got.NoData {
		if !wantNoData[c] {
			t.Errorf("NoData contains %d, which IS joinable — it must not be excluded", c)
		}
		if c == 304 {
			t.Errorf("cycle 304 is joinable but was reported as no-data")
		}
	}

	sum := 0
	for _, b := range got.Buckets {
		sum += b.Cycles
		if math.IsNaN(b.FailRate) || math.IsInf(b.FailRate, 0) {
			t.Errorf("bucket %q FailRate = %v, want a finite number (empty bucket must be 0, not 0/0)", b.Label, b.FailRate)
		}
		if b.FailRate < 0 || b.FailRate > 1 {
			t.Errorf("bucket %q FailRate = %v, want within [0,1]", b.Label, b.FailRate)
		}
	}
	if sum != 1 {
		t.Errorf("bucket cycle counts sum to %d, want 1 — no-data cycles leaked into a bucket", sum)
	}

	// All-same-verdict edge: every rate is a finite 0, nothing divides by zero.
	allPass := contextfillcorrelate.Correlate([]contextfillcorrelate.CycleFill{
		{Cycle: 401, Verdict: "PASS", Fills: []float64{0.10}},
		{Cycle: 402, Verdict: "PASS", Fills: []float64{0.95}},
	})
	if allPass.CyclesJoined != 2 {
		t.Errorf("all-PASS CyclesJoined = %d, want 2", allPass.CyclesJoined)
	}
	for _, b := range allPass.Buckets {
		if b.FailRate != 0 {
			t.Errorf("all-PASS bucket %q FailRate = %v, want 0", b.Label, b.FailRate)
		}
	}
	if allPass.Hot.FailRate != 0 || allPass.Cold.FailRate != 0 {
		t.Errorf("all-PASS hot/cold FailRate = %v/%v, want 0/0", allPass.Hot.FailRate, allPass.Cold.FailRate)
	}
}

// ---------------------------------------------------------------------------
// 003 — CLI wiring proof through top-level dispatch.
// ---------------------------------------------------------------------------

// TestC1447_003_cli_reachable_from_dispatch_emits_report drives the REAL
// production caller: the built `evolve` binary's top-level dispatch table. It
// runs `evolve context-fill correlate` against a synthetic project root holding
// one dossier plus one phase-timing log, and asserts both output surfaces — the
// --json projection on stdout and the --out markdown artifact on disk. A
// Correlate() that exists but is wired to nothing leaves this RED.
func TestC1447_003_cli_reachable_from_dispatch_emits_report(t *testing.T) {
	bin := evolveBin(t)
	root := t.TempDir()

	writeFile(t, root, "knowledge-base/cycles/cycle-7.json", dossierJSON(7, "FAIL"))
	writeFile(t, root, ".evolve/runs/cycle-7/phase-timing.json", timingJSON("build", 0.93))
	writeFile(t, root, "knowledge-base/cycles/cycle-8.json", dossierJSON(8, "PASS"))
	writeFile(t, root, ".evolve/runs/cycle-8/phase-timing.json", timingJSON("build", 0.11))

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin,
		"context-fill", "correlate", "--project-root", root, "--json")
	if err != nil || code != 0 {
		t.Fatalf("`evolve context-fill correlate --json` exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}

	var rep contextfillcorrelate.Report
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("--json stdout is not a Report: %v\nstdout:\n%s", err, stdout)
	}
	if rep.CyclesJoined != 2 {
		t.Errorf("CyclesJoined = %d, want 2 — the command must join both fixture cycles", rep.CyclesJoined)
	}
	if len(rep.NoData) != 0 {
		t.Errorf("NoData = %v, want empty on this fixture", rep.NoData)
	}
	if rep.Hot.Cycles != 1 || rep.Hot.Fail != 1 {
		t.Errorf("Hot = %+v, want {Cycles:1 Fail:1} (cycle 7 ran at 0.93 and FAILed)", rep.Hot)
	}
	if rep.Cold.Cycles != 1 || rep.Cold.Fail != 0 {
		t.Errorf("Cold = %+v, want {Cycles:1 Fail:0} (cycle 8 ran at 0.11 and PASSed)", rep.Cold)
	}

	// The markdown artifact is the deliverable the inbox item asked for.
	out := filepath.Join(root, "fill-verdict-correlation.md")
	stdout, stderr, code, err = acsassert.SubprocessOutput(bin,
		"context-fill", "correlate", "--project-root", root, "--out", out)
	if err != nil || code != 0 {
		t.Fatalf("`evolve context-fill correlate --out` exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	raw, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("--out did not write the report artifact %s: %v", out, readErr)
	}
	body := string(raw)
	for _, want := range []string{"|", "fail"} {
		if !strings.Contains(strings.ToLower(body), want) {
			t.Errorf("report artifact lacks %q — expected a bucketed markdown table\n%s", want, body)
		}
	}

	// Negative: a project root with no corpus must fail loudly, not emit an
	// empty report as if the correlation were genuinely zero.
	empty := t.TempDir()
	stdout, stderr, code, err = acsassert.SubprocessOutput(bin,
		"context-fill", "correlate", "--project-root", empty, "--json")
	if err == nil && code == 0 {
		t.Errorf("correlate over a corpus-less root exited 0, want non-zero — absent evidence must not be reported as data\nstdout:\n%s\nstderr:\n%s",
			stdout, stderr)
	}
}

// ---------------------------------------------------------------------------
// 004 — real corpus: every dossier is accounted for, nothing silently dropped.
// ---------------------------------------------------------------------------

// TestC1447_004_real_corpus_accounts_for_every_dossier runs the command against
// this repo's actual knowledge-base/cycles corpus. The load-bearing assertion is
// conservation: joined + no-data must equal the number of dossiers on disk, so a
// cycle whose fill data is missing shows up as no-data rather than vanishing or
// being fabricated into the zero bucket.
func TestC1447_004_real_corpus_accounts_for_every_dossier(t *testing.T) {
	bin := evolveBin(t)
	root := acsassert.RepoRoot(t)

	dossiers, err := filepath.Glob(filepath.Join(root, "knowledge-base", "cycles", "cycle-*.json"))
	if err != nil {
		t.Fatalf("glob dossiers: %v", err)
	}
	if len(dossiers) == 0 {
		t.Skipf("no dossier corpus under %s", filepath.Join(root, "knowledge-base", "cycles"))
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin,
		"context-fill", "correlate", "--project-root", root, "--json")
	if err != nil || code != 0 {
		t.Fatalf("`evolve context-fill correlate` over the real corpus exited %d (err=%v)\nstderr:\n%s", code, err, stderr)
	}

	var rep contextfillcorrelate.Report
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("real-corpus --json is not a Report: %v\nstdout head:\n%.400s", err, stdout)
	}
	if rep.CyclesJoined+len(rep.NoData) != len(dossiers) {
		t.Errorf("joined(%d) + no-data(%d) = %d, want %d dossiers on disk — cycles were silently dropped",
			rep.CyclesJoined, len(rep.NoData), rep.CyclesJoined+len(rep.NoData), len(dossiers))
	}
	for _, b := range rep.Buckets {
		if math.IsNaN(b.FailRate) || b.FailRate < 0 || b.FailRate > 1 {
			t.Errorf("real-corpus bucket %q FailRate = %v, want a finite rate within [0,1]", b.Label, b.FailRate)
		}
		if b.Fail > b.Cycles {
			t.Errorf("real-corpus bucket %q has Fail=%d > Cycles=%d", b.Label, b.Fail, b.Cycles)
		}
	}
}

// ---------------------------------------------------------------------------
// 005 — ADR-0069 new-package graduation for internal/contextfillcorrelate.
// ---------------------------------------------------------------------------

// TestC1447_005_new_package_apicover_graduation enforces BOTH halves of the
// repo-wide apicover gate for the new package: enrolment in go/.apicover-enforce
// (an inherent config-presence check) AND an apicover_named_test.go that names
// and exercises every exported symbol. The second half is proved by RUNNING the
// named test for that one package — enrolled-but-unnamed and unenrolled both
// abort the build phase, so a plan that ships either half alone stays RED here.
func TestC1447_005_new_package_apicover_graduation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgDir := filepath.Join(root, "go", "internal", "contextfillcorrelate")

	if !acsassert.FileExists(t, filepath.Join(pkgDir, "apicover_named_test.go")) {
		t.Errorf("missing %s — ADR-0069 requires a named-exports test for every new package", filepath.Join(pkgDir, "apicover_named_test.go"))
	}
	// acs-predicate: config-check — enrolment is inherently a config-presence
	// fact; the behavioural half is the named test executed below.
	acsassert.FileContains(t, filepath.Join(root, "go", ".apicover-enforce"), "./internal/contextfillcorrelate")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"),
		"test", "-run", "TestAPICoverNamedExports", "-count=1", "./internal/contextfillcorrelate")
	if err != nil || code != 0 {
		t.Fatalf("named-exports test for internal/contextfillcorrelate exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	if strings.Contains(stdout, "no test files") {
		t.Errorf("internal/contextfillcorrelate reports \"no test files\" — the named-exports test must actually run\n%s", stdout)
	}
}

// ---------------------------------------------------------------------------
// 006 — the repo-contract scanner pack is GREEN in THIS lane (cycle-1447).
// ---------------------------------------------------------------------------

// repoContractSuites mirrors ship.repoContractPackages — the four guard suites
// the ship-time repo-contract gate runs in the lane worktree. Kept as separate
// entries (rather than one multi-package `go test`) so each suite is its own
// named-package invocation and a red one is named, not buried in a pack-wide
// exit code.
var repoContractSuites = []string{
	"./internal/phasespec",
	"./internal/profiles",
	"./internal/phasecoherence",
	"./internal/routingtest",
}

// TestC1447_006_repo_contract_scanner_pack_green_in_lane is this continuation's
// own acceptance criterion. Cycle-1402's ship died here:
//
//	[REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract scanner pack
//	RED in the lane worktree (exit status 1) — pushing would red main
//
// and the reason was never diagnosed, so the whole feature has sat stranded.
// The predicate RUNS each suite in this lane's module dir and requires exit 0,
// which is exactly the gate's precondition — a landing that reintroduces the
// prior RED (a stray/invalid `.evolve/phases/<name>/phase.json` overlay, an
// unpaired phase↔agent pair, a routing-table drift) fails here rather than at
// ship. Each suite is invoked on ONE named package with an explicit -C module
// dir, never a `./...` sweep and never cwd-relative `go` — the lane worktree is
// not the process cwd.
//
// The auxiliary check afterwards pins the suite list against the gate's own
// declaration so this predicate cannot silently drift into testing a subset of
// what the gate enforces.
func TestC1447_006_repo_contract_scanner_pack_green_in_lane(t *testing.T) {
	root := acsassert.RepoRoot(t)
	moduleDir := filepath.Join(root, "go")

	for _, pkg := range repoContractSuites {
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "-C", moduleDir, "test", "-count=1", pkg)
		if err != nil || code != 0 {
			t.Errorf("repo-contract suite %s is RED in this lane (exit %d, err=%v) — the ship gate will block on it, as it did in cycle-1402\nstdout:\n%s\nstderr:\n%s",
				pkg, code, err, stdout, stderr)
		}
	}

	// acs-predicate: config-check — auxiliary drift guard only; the behavioural
	// weight of this predicate is the four suite runs above.
	gateSrc := filepath.Join(root, "go", "internal", "phases", "ship", "repocontract.go")
	for _, pkg := range repoContractSuites {
		if !acsassert.FileContains(t, gateSrc, `"`+pkg+`/..."`) {
			t.Errorf("%s runs %q but this predicate's suite list does not match the gate's — the list drifted", gateSrc, pkg)
		}
	}
}
