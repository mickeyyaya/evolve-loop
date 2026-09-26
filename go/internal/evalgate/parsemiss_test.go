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

// cycle1570ReportShape states its slug as prose ("Task slug:") rather than as a
// bullet, so it parses to no slug.
const cycle1570ReportShape = "# Scout Report\n\n## Selected Tasks\n\n" +
	"### Task 1: Config gate default policy authority\n" +
	"Task slug: config-gate-default-policy-authority\n" +
	"- **Type:** bug\n- **Complexity:** S\n\n" +
	"No eval file was authored for this task; the slug is only named in prose above.\n"

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

func TestSelectedTasksParseMiss(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   bool
	}{
		{"cycle-1570 prose slug", cycle1570ReportShape, true},
		{
			name: "drifted section followed by another heading",
			report: "## Selected Tasks\n\n### Task 1: Drifted\nTask slug: drifted-task\n" +
				"- **Type:** bug\n\n## Deferred\n- **Slug:** later-one\n",
			want: true,
		},

		{"no Selected Tasks section at all", "## Gap Analysis\nNothing to do.\n", false},
		{"empty report", "", false},
		{"prose report that never uses the heading", "# Scout Report\n\nThe backlog converged; no task was selected.\n", false},
		{"decision trace with zero selections", "## Decision Trace\n```json\n{\"decisionTrace\":[]}\n```\n", false},

		{"heading then EOF", "## Selected Tasks\n", false},
		{"heading with no trailing newline", "## Selected Tasks", false},
		{"heading then blank lines only", "## Selected Tasks\n\n\n   \n\t\n", false},
		{"heading then an html comment only", "## Selected Tasks\n\n<!-- none selected this cycle -->\n", false},
		{"heading then a multi-line html comment only", "## Selected Tasks\n\n<!--\nnone\nselected\n-->\n", false},
		{"heading then blanks then the next heading", "## Selected Tasks\n\n\n## Deferred\n- carried prose\n", false},

		{
			name: "prose lives after the next heading",
			report: "## Selected Tasks\n\n## Deferred\n\n### Task 1: Elsewhere\n" +
				"Task slug: lives-in-another-section\nreal prose outside the bounded body\n",
			want: false,
		},

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

// scoutWorkspaceWithReport writes body as the scout report and returns a project
// root holding no eval.
func scoutWorkspaceWithReport(t *testing.T, body string) (projectRoot, workspace string) {
	t.Helper()
	projectRoot, workspace = t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, scoutReportName), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", scoutReportName, err)
	}
	return projectRoot, workspace
}

// captureParseMissLog captures stderr around fn; NewReviewer's logger resolves
// os.Stderr at call time.
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

// Both shapes carry their slug only as a backticked bullet, and a Decision Trace in the
// "selected_tasks" array form that decisionTraceSelected does not read.
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

// preWideningSlugLineRE is slugLineRE without the backtick widening, the counterfactual arm.
var preWideningSlugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*([a-z0-9][a-z0-9-]*)`)

var slugBulletRE = regexp.MustCompile(`(?m)^- \*\*Slug:\*\*.*\n`)

func TestSlugLineWideningIsNotBlockingNeutral(t *testing.T) {
	for _, tc := range []struct{ cycle, slug, report string }{
		{"cycle-1664", "settle-wait-stability-shortcircuit", cycle1664ReportShape},
		{"cycle-1669", "verdict-tool-call-claudep", cycle1669ReportShape},
	} {
		t.Run(tc.cycle, func(t *testing.T) {
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

			got := SelectedSlugs(tc.report)
			if len(got) != 1 || got[0] != tc.slug {
				t.Fatalf("SelectedSlugs(%s)=%v, want exactly [%s] — the widening is the only thing that can parse this backticked bullet, and the union it produces is therefore NOT the pre-widening one", tc.cycle, got, tc.slug)
			}

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

func reviewScoutReport(t *testing.T, body string) core.ReviewResult {
	t.Helper()
	root, ws := scoutWorkspaceWithReport(t, body)
	return NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws,
	})
}
