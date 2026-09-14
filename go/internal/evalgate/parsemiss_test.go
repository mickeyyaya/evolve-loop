// parsemiss_test.go — cycle-1685 regression pins for the parse-miss vs
// convergence distinction (inbox id `evalgate-selectedslugs-nil-blindness`).
//
// SelectedSlugs returns nil for two categorically different scout-report shapes,
// and until this cycle nothing downstream could tell them apart:
//
//  1. genuine convergence — no "## Selected Tasks" section at all; fail-open is
//     CORRECT, the cycle claimed no work;
//  2. format drift — the section IS present with real task prose whose slug the
//     parser cannot read, so it yields zero slugs and Gate A's fail-open path
//     checks NOTHING while believing it checked everything.
//
// Shape 2 is cycle-1570: scout selected `config-gate-default-policy-authority`,
// authored no eval, the section parsed to nil, Gate A approved, and the missing
// eval surfaced three phases later as an audit H1.

package evalgate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// cycle1570ReportShape is the REAL incident shape, not a synthetic stand-in: a
// "## Selected Tasks" section carrying genuine task prose for
// config-gate-default-policy-authority whose slug is stated as free prose
// ("Task slug: ...") instead of the "- **Slug:**" bullet slugLineRE requires.
// Neither empty nor absent — and it parses to zero slugs.
const cycle1570ReportShape = "# Scout Report\n\n## Selected Tasks\n\n" +
	"### Task 1: Config gate default policy authority\n" +
	"Task slug: config-gate-default-policy-authority\n" +
	"- **Type:** bug\n- **Complexity:** S\n\n" +
	"No eval file was authored for this task; the slug is only named in prose above.\n"

// TestCycle1570ReportShape pins the incident itself. The first assertion is the
// fixture PREMISE: this shape must still parse to zero slugs, or the predicate
// below is exercising nothing. The second is the fix.
func TestCycle1570ReportShape(t *testing.T) {
	if got := SelectedSlugs(cycle1570ReportShape); got != nil {
		t.Fatalf("fixture premise broken: the cycle-1570 shape must still parse to ZERO slugs for this test to pin the reported bug; SelectedSlugs()=%v", got)
	}
	if !SelectedTasksParseMiss(cycle1570ReportShape) {
		t.Errorf("SelectedTasksParseMiss(cycle-1570 shape)=false, want true — a section full of real, unreadable task prose is still indistinguishable from a converged cycle, which is the entire defect")
	}
	if !strings.Contains(cycle1570ReportShape, "config-gate-default-policy-authority") {
		t.Error("the fixture no longer names the real incident slug; a synthetic stand-in can drift away from the defect it is supposed to pin")
	}
}

// TestSelectedTasksParseMiss is the behavioural table: true ONLY for drift.
func TestSelectedTasksParseMiss(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   bool
	}{
		// Drift — the section claims work the parser could not read.
		{"cycle-1570 prose slug", cycle1570ReportShape, true},
		{
			name: "drifted section followed by another heading",
			report: "## Selected Tasks\n\n### Task 1: Drifted\nTask slug: drifted-task\n" +
				"- **Type:** bug\n\n## Deferred\n- **Slug:** later-one\n",
			want: true,
		},

		// Genuine convergence — nothing was claimed.
		{"no Selected Tasks section at all", "## Gap Analysis\nNothing to do.\n", false},
		{"empty report", "", false},
		{"prose report that never uses the heading", "# Scout Report\n\nThe backlog converged; no task was selected.\n", false},
		{"decision trace with zero selections", "## Decision Trace\n```json\n{\"decisionTrace\":[]}\n```\n", false},

		// Contentless section — a heading claiming nothing is still convergence.
		{"heading then EOF", "## Selected Tasks\n", false},
		{"heading with no trailing newline", "## Selected Tasks", false},
		{"heading then blank lines only", "## Selected Tasks\n\n\n   \n\t\n", false},
		{"heading then an html comment only", "## Selected Tasks\n\n<!-- none selected this cycle -->\n", false},
		{"heading then a multi-line html comment only", "## Selected Tasks\n\n<!--\nnone\nselected\n-->\n", false},
		{"heading then blanks then the next heading", "## Selected Tasks\n\n\n## Deferred\n- carried prose\n", false},

		// Section-bounded: content in a LATER section is not this section's drift.
		{
			name: "prose lives after the next heading",
			report: "## Selected Tasks\n\n## Deferred\n\n### Task 1: Elsewhere\n" +
				"Task slug: lives-in-another-section\nreal prose outside the bounded body\n",
			want: false,
		},

		// Readable sections are never drift.
		{"plain slug bullet", "## Selected Tasks\n- **Slug:** add-cache\n", false},
		{"backticked slug bullet (the persona's live form)", "## Selected Tasks\n- **Slug:** `add-cache`\n", false},
		{"bounded bullet with a later section", "## Selected Tasks\n- **Slug:** in-section\n\n## Deferred\n- **Slug:** not-counted\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SelectedTasksParseMiss(c.report); got != c.want {
				t.Errorf("SelectedTasksParseMiss()=%v, want %v", got, c.want)
			}
		})
	}
}

// TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged pins the scoping: the
// signal reads the Selected Tasks body and nothing else, so a separately
// malformed "## Decision Trace" can never leak into it.
func TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged(t *testing.T) {
	t.Run("readable section beside a malformed decision trace", func(t *testing.T) {
		report := "## Selected Tasks\n- **Slug:** ok-slug\n\n## Decision Trace\n```json\n{not valid json\n```\n"
		if got := SelectedSlugs(report); len(got) == 0 {
			t.Fatalf("fixture premise broken: the section is readable and must yield a slug; SelectedSlugs()=%v", got)
		}
		if SelectedTasksParseMiss(report) {
			t.Error("SelectedTasksParseMiss()=true, want false — the Selected Tasks section parsed fine; a malformed Decision Trace is a different fail-open path and must not leak into this signal")
		}
	})

	t.Run("trace-only report has no section to drift", func(t *testing.T) {
		report := "# Scout Report\n\n## Decision Trace\n```json\n" +
			"{\"decisionTrace\":[{\"slug\":\"trace-only\",\"finalDecision\":\"selected\"}]}\n```\n"
		if SelectedTasksParseMiss(report) {
			t.Error("SelectedTasksParseMiss()=true, want false — a report with no '## Selected Tasks' heading has no section to have misparsed")
		}
	})

	t.Run("malformed trace alone is not drift", func(t *testing.T) {
		if SelectedTasksParseMiss("## Decision Trace\n```json\n{not valid json\n```\n") {
			t.Error("SelectedTasksParseMiss()=true, want false — the signal is scoped to the Selected Tasks section only")
		}
	})
}

// TestMaterializationGate_ParseMissAdvisoryIsNonBlocking is the caller proof:
// the detector must actually reach Gate A, and must reach it as an ADVISORY.
// Both halves matter — an unreported signal is dead code, and a blocking one
// would false-block every converged cycle.
func TestMaterializationGate_ParseMissAdvisoryIsNonBlocking(t *testing.T) {
	g := materializationGate{}

	t.Run("check reports the drift without blocking", func(t *testing.T) {
		root, ws := scoutWorkspaceWithReport(t, cycle1570ReportShape)
		reason, block := g.check(core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws})
		if block {
			t.Errorf("check() blocked on a parse-miss (reason=%q) — the signal must stay advisory; hard-blocking every zero-slug report false-blocks every genuine convergence cycle", reason)
		}
		low := strings.ToLower(reason)
		if !strings.Contains(low, "selected tasks") || !strings.Contains(low, "parse-miss") {
			t.Errorf("check()'s reason does not name the drifted section and the parse-miss condition, so it is not actionable; reason=%q", reason)
		}
	})

	t.Run("the advisory reaches Gate A through the production reviewer", func(t *testing.T) {
		root, ws := scoutWorkspaceWithReport(t, cycle1570ReportShape)
		var res core.ReviewResult
		logged := captureParseMissLog(t, func() {
			res = NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
				Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws,
			})
		})
		if !strings.Contains(logged, "evals-materialized") {
			t.Fatalf("Gate A emitted NO line for a parse-miss report — the advisory never reaches the seam an operator sees; stderr=%q", logged)
		}
		if !res.Approve {
			t.Errorf("the production reviewer REJECTED a parse-miss report (reason=%q); the advisory must not block", res.Reason)
		}
	})

	t.Run("silent on genuine convergence", func(t *testing.T) {
		root, ws := scoutWorkspaceWithReport(t, "## Gap Analysis\nNothing to do.\n")
		if reason, block := g.check(core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws}); reason != "" || block {
			t.Errorf("check()=(%q, %v) on a converged report; an advisory that fires every cycle is noise, not signal", reason, block)
		}
	})

	t.Run("the pre-existing hard block still fires", func(t *testing.T) {
		root, ws := scoutWorkspaceSelecting(t, "no-eval-authored")
		reason, block := g.check(core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws})
		if !block {
			t.Fatalf("a selected slug with no eval file no longer blocks — the pre-existing contract regressed; reason=%q", reason)
		}
		if !strings.Contains(reason, "no-eval-authored") {
			t.Errorf("the block does not name the missing slug; reason=%q", reason)
		}
	})

	// The advisory must not claim more than it knows. Gate A checked NOTHING only
	// when the union is genuinely empty; when the "## Decision Trace" still
	// supplied slugs the gate DID check those, and on 3 of the 59 real cycle-16*
	// fires it had (audit L1, cycle 1685).
	t.Run("claims nothing-was-checked only when the union is empty", func(t *testing.T) {
		root, ws := scoutWorkspaceWithReport(t, cycle1570ReportShape)
		if got := SelectedSlugs(cycle1570ReportShape); got != nil {
			t.Fatalf("fixture premise broken: this shape must yield an EMPTY union; SelectedSlugs()=%v", got)
		}
		reason, _ := g.check(core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws})
		if !strings.Contains(reason, "checked NOTHING") {
			t.Errorf("the advisory drops the nothing-was-checked claim on an empty union, which is exactly where it is true; reason=%q", reason)
		}
	})

	t.Run("narrows the claim when the Decision Trace supplied slugs", func(t *testing.T) {
		report := cycle1570ReportShape + "\n## Decision Trace\n```json\n" +
			`{"decisionTrace":[{"slug":"traced-task","finalDecision":"selected"}]}` + "\n```\n"
		if got := SelectedSlugs(report); len(got) != 1 || got[0] != "traced-task" {
			t.Fatalf("fixture premise broken: the trace must supply exactly traced-task; SelectedSlugs()=%v", got)
		}
		if !SelectedTasksParseMiss(report) {
			t.Fatal("fixture premise broken: the Selected Tasks section must still be a parse-miss")
		}
		root, ws := scoutWorkspaceWithReport(t, report)
		reason, _ := g.check(core.ReviewInput{Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws})
		if strings.Contains(reason, "checked NOTHING") {
			t.Errorf("the advisory still asserts Gate A checked NOTHING while the trace supplied traced-task, which it DID check; reason=%q", reason)
		}
		if !strings.Contains(reason, "traced-task") {
			t.Errorf("the narrowed advisory does not name what Gate A actually checked, so the operator cannot tell what is still unchecked; reason=%q", reason)
		}
	})
}

// scoutWorkspaceWithReport writes body as the scout deliverable in a fresh
// workspace, under the artifact name the scout phase really produces
// (scoutReportName, resolved from the phasecontract registry Gate A reads
// through), and returns an empty project root so no eval file exists anywhere.
func scoutWorkspaceWithReport(t *testing.T, body string) (projectRoot, workspace string) {
	t.Helper()
	projectRoot, workspace = t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, scoutReportName), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", scoutReportName, err)
	}
	return projectRoot, workspace
}

// captureParseMissLog runs fn with os.Stderr redirected to a pipe and returns
// what was written. reviewer.logf resolves os.Stderr at call time, so this is
// how the production gate line is observed without re-implementing the reviewer.
func captureParseMissLog(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	orig := os.Stderr
	os.Stderr = w
	func() {
		defer func() { os.Stderr = orig }()
		fn()
	}()
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// --- the slugLineRE widening's measured effect ---------------------------------
//
// Round 1 of this cycle shipped a NEUTRALITY CLAIM about the backtick widening —
// "the union SelectedSlugs returns is unchanged and no report becomes newly
// blockable" — asserted from a count of "## Decision Trace" HEADINGS. Heading
// presence is not union equality, and the claim was false: re-measured
// 2026-09-15 over .evolve/runs/cycle-16*/scout-report.md, the widening changes
// the union on 6 of 84 reports and makes 2 of them newly blockable at Gate A.
//
// The corpus is gitignored and absent in CI, so the correction cannot live only
// in prose citing it. These fixtures reproduce the two falsifying reports so the
// claim is RE-RUN on every `go test ./internal/evalgate`, which is the half
// round 1 was missing.

// cycle1664ReportShape and cycle1669ReportShape are the two real reports whose
// Gate A verdict the widening changes. Each keeps the two properties that
// matter: the slug appears ONLY as a backticked "- **Slug:**" bullet, and the
// "## Decision Trace" is present but states its selection as a "selected_tasks"
// string array — a shape decisionTraceSelected does not read, so the trace
// supplies nothing and the bullet is the sole source.
const cycle1664ReportShape = "# Scout Report — Cycle 1664\n\n" +
	"## Selected Tasks\n\n" +
	"### Task 1: settle-wait stability short-circuit\n" +
	"- **Deliverable kind:** code\n" +
	"- **Slug:** `settle-wait-stability-shortcircuit`\n" +
	"- **Complexity:** S\n\n" +
	"## Decision Trace\n\n```json\n{\n" +
	"  \"fleet_scope\": [\"nonconforming-deliverable-settle-wait-latency\"],\n" +
	"  \"selected_tasks\": [\"settle-wait-stability-shortcircuit\"]\n" +
	"}\n```\n"

const cycle1669ReportShape = "# Scout Report — Cycle 1669\n\n" +
	"## Selected Tasks\n\n" +
	"### Task 1: verdict-as-tool-call for the headless claude -p driver\n" +
	"- **Deliverable kind:** code\n" +
	"- **Slug:** `verdict-tool-call-claudep`\n" +
	"- **Complexity:** M\n\n" +
	"## Decision Trace\n\n```json\n{\n" +
	"  \"cycle\": 1669,\n" +
	"  \"selected_tasks\": [\"verdict-tool-call-claudep\"],\n" +
	"  \"deferred\": [\"extend-structured-verdict-to-codex-agy\"]\n" +
	"}\n```\n"

// preWideningSlugLineRE is slugLineRE exactly as it stood before this cycle
// (git show HEAD:go/internal/evalgate/slugs.go). The production parser cannot be
// swapped back at run time, so the counterfactual arm needs a local copy of what
// the old pattern matched.
var preWideningSlugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*([a-z0-9][a-z0-9-]*)`)

// slugBulletRE matches a whole "- **Slug:**" bullet line, for building the B arm
// of the A/B pair by deleting it.
var slugBulletRE = regexp.MustCompile(`(?m)^- \*\*Slug:\*\*.*\n`)

// TestSlugLineWideningIsNotBlockingNeutral executes the corrected claim.
//
// Union half: on both reports the "## Decision Trace" is present but yields
// nothing and the pre-widening pattern matches nothing, while the shipped
// SelectedSlugs returns the slug — so the union the widening produces differs.
//
// Blocking half: driving the production reviewer at StageEnforce over an A/B
// pair that differs ONLY by that bullet, Gate A BLOCKS with it and fail-opens
// without it. That delta is what "newly blockable" means, and it is the sentence
// round 1 got wrong.
func TestSlugLineWideningIsNotBlockingNeutral(t *testing.T) {
	for _, tc := range []struct{ cycle, slug, report string }{
		{"cycle-1664", "settle-wait-stability-shortcircuit", cycle1664ReportShape},
		{"cycle-1669", "verdict-tool-call-claudep", cycle1669ReportShape},
	} {
		t.Run(tc.cycle, func(t *testing.T) {
			// Premise: these are reports that DO carry the heading round 1
			// counted, and whose trace nonetheless supplies no slug.
			if !strings.Contains(tc.report, "## Decision Trace") {
				t.Fatalf("fixture premise broken: %s no longer carries a \"## Decision Trace\" — the falsified claim was about reports that carry one", tc.cycle)
			}
			if traced := decisionTraceSelected(tc.report); len(traced) != 0 {
				t.Fatalf("fixture premise broken: the trace supplies %v in %s, so the bullet is no longer the sole source of the slug", traced, tc.cycle)
			}
			body, ok := selectedTasksBody(tc.report)
			if !ok {
				t.Fatalf("fixture premise broken: %s has no \"## Selected Tasks\" section", tc.cycle)
			}
			if before := preWideningSlugLineRE.FindAllStringSubmatch(body, -1); len(before) != 0 {
				t.Fatalf("fixture premise broken: the pre-widening pattern already matched %v in %s, so this report cannot show a widening delta", before, tc.cycle)
			}

			// Union half.
			got := SelectedSlugs(tc.report)
			if len(got) != 1 || got[0] != tc.slug {
				t.Fatalf("SelectedSlugs(%s)=%v, want exactly [%s] — the widening is the only thing that can parse this backticked bullet, and the union it produces is therefore NOT the pre-widening one", tc.cycle, got, tc.slug)
			}

			// Blocking half, B arm: the same report without the bullet.
			without := slugBulletRE.ReplaceAllString(tc.report, "")
			if strings.Contains(without, "**Slug:**") {
				t.Fatalf("counterfactual arm still carries a Slug bullet — the A/B pair is not isolated to the widening")
			}
			if leftover := SelectedSlugs(without); len(leftover) != 0 {
				t.Fatalf("counterfactual premise broken: SelectedSlugs=%v with the bullet removed, so something other than the bullet supplies the slug", leftover)
			}
			if res := reviewScoutReport(t, without); !res.Approve {
				t.Fatalf("Gate A BLOCKED the counterfactual arm (reason=%q) — with no parsed slug it must fail open, or this pair cannot show a NEWLY blockable report", res.Reason)
			}

			// A arm: the real report. No eval file exists at either resolution
			// evalFilePath checks, so the newly parsed slug blocks.
			res := reviewScoutReport(t, tc.report)
			if res.Approve {
				t.Fatalf("Gate A APPROVED %s, whose backticked slug %q has no eval file — then the widening really would be blocking-neutral and the comment on slugLineRE is wrong; re-measure before editing it", tc.cycle, tc.slug)
			}
			if !strings.Contains(res.Reason, tc.slug) {
				t.Errorf("Gate A's rejection for %s does not name %q, so the new block is not actionable; reason=%q", tc.cycle, tc.slug, res.Reason)
			}
		})
	}
}

// reviewScoutReport drives the PRODUCTION reviewer — the core.WithReviewer seam
// the orchestrator mounts — over body in a fresh workspace with an empty project
// root, so no eval file resolves at either path evalFilePath checks.
func reviewScoutReport(t *testing.T, body string) core.ReviewResult {
	t.Helper()
	root, ws := scoutWorkspaceWithReport(t, body)
	return NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws,
	})
}
