//go:build acs

// Package cycle1703 materialises the acceptance criteria for
// multi-member-file-scope-advisory: once a multi-member lane's declared set
// reconciles with the committed set, the TDD scope gate must still judge the
// authored test files against the union of every member's scout targetFiles,
// advisory only, while single-member lanes keep today's advisory unchanged.
//
// Predicates 001-004 drive the shipping reviewer (topngate.NewReviewer at the
// enforce stage, the constructor cmd_cycle wires) over synthetic phase
// deliverables and assert on its verdict and its stderr logf seam. Predicate
// 005 runs the package's vet and race suite, and requires the named
// in-package regression tests to PASS.
package cycle1703

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	alpha       = "alpha-member"
	beta        = "beta-member"
	alphaTarget = "go/internal/alpha/alpha.go"
	betaTarget  = "go/internal/beta/beta.go"
	driftFile   = "go/internal/tokenresolver/resolver_test.go"
	topngatePkg = "./internal/topngate"
)

type scoutTask struct {
	slug        string
	targetFiles []string
}

// lane is one TDD-boundary workspace: what triage committed, what scout
// declared per task, and what the TDD handoff declared and authored.
type lane struct {
	committed []string
	scout     []scoutTask
	declared  []string
	authored  []string
}

func (l lane) write(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()

	ids := make([]map[string]string, 0, len(l.committed))
	var triage strings.Builder
	triage.WriteString("# Triage Decision\n\n## top_n (commit to THIS cycle)\n")
	for _, id := range l.committed {
		ids = append(ids, map[string]string{"id": id})
		triage.WriteString("- " + id + ": placeholder — priority=M, evidence=x, source=scout\n")
	}
	triage.WriteString("\n## deferred (carry to NEXT cycle's carryoverTodos)\n(none)\n")
	writeJSON(t, ws, "triage-decision.json", map[string]any{"top_n": ids})
	writeFile(t, ws, "triage-report.md", triage.String())

	if l.scout != nil {
		var scout strings.Builder
		scout.WriteString("# Scout Report\n\n## Selected Tasks\n\n")
		for i, task := range l.scout {
			scout.WriteString("### Task " + string(rune('1'+i)) + ": " + task.slug + "\n\n")
			quoted := make([]string, 0, len(task.targetFiles))
			for _, f := range task.targetFiles {
				quoted = append(quoted, "`"+f+"`")
			}
			scout.WriteString("- **targetFiles:** " + strings.Join(quoted, ", ") + "\n- **complexity:** S\n\n")
		}
		scout.WriteString("## Acceptance Criteria Summary\n\n- placeholder\n")
		writeFile(t, ws, "scout-report.md", scout.String())
	}

	handoff, err := json.Marshal(map[string]any{
		"slugs": l.declared, "testFiles": l.authored, "redRunConfirmed": true, "doNotModifyTests": true,
	})
	if err != nil {
		t.Fatalf("marshal handoff: %v", err)
	}
	fence := "```"
	writeFile(t, ws, "test-report.md", "# TDD Report\n\n## Task: "+strings.Join(l.declared, ", ")+
		"\n\n## RED Run Output\n\n"+fence+"\nFAIL\n"+fence+"\n\n## Handoff to Builder\n\n"+fence+"json\n"+string(handoff)+"\n"+fence+"\n")
	return ws
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	writeFile(t, dir, name, string(body))
}

// twoMembers commits alpha and beta, each declaring its own package.
func twoMembers(declared []string, authored ...string) lane {
	return lane{
		committed: []string{alpha, beta},
		scout:     []scoutTask{{alpha, []string{alphaTarget}}, {beta, []string{betaTarget}}},
		declared:  declared,
		authored:  authored,
	}
}

// review runs the shipping reviewer at enforce and returns its verdict plus
// everything it wrote to the stderr logf seam.
func review(t *testing.T, workspace string) (res core.ReviewResult, logged string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	res = topngate.NewReviewer(config.StageEnforce).Review(
		context.Background(),
		core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: workspace},
	)

	os.Stderr = orig
	_ = w.Close()
	logged = <-done
	_ = r.Close()
	return res, logged
}

func TestC1703_001_TwoMemberDriftOutsideBothScopesIsAdvised(t *testing.T) {
	ws := twoMembers([]string{alpha, beta}, driftFile).write(t)

	res, logged := review(t, ws)
	if !res.Approve {
		t.Fatalf("file-scope drift must stay advisory (block=false) for a multi-member lane; got %+v", res)
	}
	if !strings.Contains(logged, "file scope drift") {
		t.Fatalf("a complete two-member declaration authoring outside both members' scopes must emit the file-scope advisory on the logf seam; logged=%q", logged)
	}
	for _, want := range []string{alpha, beta, driftFile, alphaTarget, betaTarget} {
		if !strings.Contains(logged, want) {
			t.Errorf("advisory must name both members, the authored file and every member's declared targetFiles; missing %q in %q", want, logged)
		}
	}
}

func TestC1703_002_IncompleteDeclarationStillBlocksOnScopeMismatch(t *testing.T) {
	ws := twoMembers([]string{alpha}, driftFile).write(t)

	res, _ := review(t, ws)
	if res.Approve || !strings.Contains(res.Reason, "scope-mismatch") {
		t.Fatalf("a two-member commitment with a one-member declaration must still block on scope-mismatch; got %+v", res)
	}
	if !strings.Contains(res.Reason, beta) {
		t.Errorf("the block must name the undeclared member %q; reason=%q", beta, res.Reason)
	}
	if strings.Contains(res.Reason, "file scope") {
		t.Errorf("file-scope drift is judged only after a complete reconciliation; reason=%q", res.Reason)
	}
}

func TestC1703_003_TwoMemberAuthoredInsideEitherScopeIsSilent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		authored []string
	}{
		{"inside alpha's scope", []string{"go/internal/alpha/alpha_test.go"}},
		{"inside beta's scope", []string{"go/internal/beta/beta_test.go"}},
		{"one overlapping file among several", []string{"go/acs/cycle1703/predicates_test.go", "go/internal/beta/beta_test.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := twoMembers([]string{alpha, beta}, tc.authored...).write(t)

			res, logged := review(t, ws)
			if !res.Approve {
				t.Fatalf("a complete in-scope declaration must proceed; got %+v", res)
			}
			if strings.TrimSpace(logged) != "" {
				t.Errorf("authored files inside either member's declared scope must yield no advisory; logged=%q", logged)
			}
		})
	}
}

func TestC1703_004_SingleMemberAdvisoryTextUnchanged(t *testing.T) {
	const slug = "committed-slug"
	ws := lane{
		committed: []string{slug},
		scout:     []scoutTask{{slug, []string{"go/internal/topngate/gate.go", "go/internal/topngate/gate_test.go"}}},
		declared:  []string{slug},
		authored:  []string{driftFile},
	}.write(t)

	want := "file scope drift (advisory): TDD authored test file(s) {" + driftFile + "}" +
		" but the committed item '" + slug + "' declares targetFiles {go/internal/topngate/gate.go, go/internal/topngate/gate_test.go}" +
		" — zero path overlap"
	res, logged := review(t, ws)
	if !res.Approve {
		t.Fatalf("the single-member file-scope advisory must approve at enforce; got %+v", res)
	}
	if !strings.Contains(logged, want) {
		t.Errorf("single-member lanes must keep today's advisory text byte for byte;\nwant substring %q\nlogged %q", want, logged)
	}
}

func TestC1703_005_TopngateVetAndRaceSuiteGreen(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")

	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", goDir, topngatePkg)
	if code != 0 {
		t.Fatalf("go vet %s exited %d (err=%v)\n%s%s", topngatePkg, code, err, stdout, stderr)
	}

	stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-C", goDir, "-race", "-count=1", "-v", topngatePkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("go test -race %s exited %d\n%s", topngatePkg, code, out)
	}
	for _, name := range []string{
		"TestTDDScopeGate_TwoMemberFileScopeDriftIsAdvised",
		"TestTDDScopeGate_TwoMemberInEitherScopeStaysSilent",
		"TestTDDScopeGate_TwoMemberWithoutDeclaredScopeStaysSilent",
		"TestTDDScopeGate_IncompleteMemberDeclarationBlocksBeforeScopeCheck",
		"TestTDDScopeGate_SingleMemberFileScopeAdvisoryTextUnchanged",
		"TestTDDScopeGate_FileScopeDriftIsAdvisory",
		"TestTDDScopeGate_FileScopeBinding",
	} {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("no PASS line for %s (renamed, skipped, or never ran?)", name)
		}
	}
}
