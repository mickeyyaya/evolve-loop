package core

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const auditScopeFile = "audit-scope.json"

type ScopeFinding struct {
	HostID   string `json:"host_id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
}

type AuditRoundScope struct {
	Round    int                `json:"round"`
	Enforce  bool               `json:"enforce,omitempty"`
	Tree     string             `json:"tree"`
	Worktree string             `json:"worktree,omitempty"`
	KnownIDs []string           `json:"known_ids"`
	Previous []ScopeFinding     `json:"previous"`
	Hunks    []convergence.Hunk `json:"hunks"`
	FixHunks []convergence.Hunk `json:"fix_hunks"`
}

type scopeInputs struct {
	ledger      Ledger
	cycle       int
	round       int
	projectRoot string
	worktree    string
	workspace   string
}

func HostFindingID(f reportdoc.Finding) string {
	return defectledger.ID(findingTitleKey(f))
}

func findingTitleKey(f reportdoc.Finding) string {
	return cmp.Or(f.Title, f.ID, "finding")
}

func (s *AuditRoundScope) Late(f reportdoc.Finding) bool {
	if s == nil || f.Location == "" || len(s.Hunks) == 0 {
		return false
	}
	switch {
	case slices.Contains(s.KnownIDs, HostFindingID(f)):
		return false
	case convergence.InsideHunks(f.Location, s.Hunks):
		return false
	}
	return !slices.ContainsFunc(s.Previous, func(p ScopeFinding) bool { return p.Location == f.Location })
}

func (s *AuditRoundScope) CheckFiled(entries []reportdoc.Finding) []string {
	var problems []string
	for _, e := range entries {
		if why := s.filingProblem(e); why != "" {
			problems = append(problems, fmt.Sprintf("%s (%s) %s: %s", e.ID, e.Severity, findingTitleKey(e), why))
		}
	}
	return problems
}

func (s *AuditRoundScope) filingProblem(e reportdoc.Finding) string {
	switch {
	case slices.Contains(s.KnownIDs, HostFindingID(e)):
		return "its host id is known from an earlier round, so it is not a new finding"
	case e.Location == "":
		return "it names no path:line, so it is in scope and never late"
	case convergence.InsideHunks(e.Location, s.Hunks):
		return "its location " + e.Location + " is inside the repair hunks"
	case !s.Late(e):
		return "its location " + e.Location + " is the location of an open previous finding or no hunks exist"
	case !convergence.LateFiled(convergence.LoopAuditRepair, lateMark(e)):
		return "a late " + e.Severity + " blocks and is never filed"
	}
	return ""
}

func lateMark(f reportdoc.Finding) convergence.Finding {
	return convergence.Finding{ID: HostFindingID(f), Severity: convergence.Severity(f.Severity), Status: convergence.StatusOpen, Late: true}
}

func AuditWorktreeTree(ctx context.Context, req PhaseRequest) string {
	return worktreeContentSHA(ctx, req.ProjectRoot, req.Worktree, req.Workspace)
}

func computeAuditScope(ctx context.Context, in scopeInputs) (AuditRoundScope, string) {
	if in.round < 2 {
		return AuditRoundScope{}, "round 1 is a full audit"
	}
	known, locations, reason := archivedFindings(in)
	if reason != "" {
		return AuditRoundScope{}, reason
	}
	trees, reason := auditorTrees(ctx, in)
	if reason != "" {
		return AuditRoundScope{}, reason
	}
	current := worktreeContentSHA(ctx, in.projectRoot, in.worktree, in.workspace)
	if current == "" {
		return AuditRoundScope{}, "the round tree is unknown (in-place worktree or a failed ship tree)"
	}
	cumulative, err := diffPostImageHunks(ctx, in.worktree, trees[0], current)
	if err != nil {
		return AuditRoundScope{}, "git: " + err.Error()
	}
	repair, err := diffPostImageHunks(ctx, in.worktree, trees[in.round-2], current)
	if err != nil {
		return AuditRoundScope{}, "git: " + err.Error()
	}
	return AuditRoundScope{Round: in.round, Tree: current, Worktree: in.worktree, KnownIDs: known, Previous: openPreviousFindings(in, locations), Hunks: cumulative, FixHunks: repair}, ""
}

func archivedFindings(in scopeInputs) (known []string, locations map[string]string, reason string) {
	locations = map[string]string{}
	for round := 1; round < in.round; round++ {
		raw, err := os.ReadFile(filepath.Join(in.workspace, phasecontract.RoundArchiveFilename(auditReportName, round)))
		if err != nil {
			return nil, nil, fmt.Sprintf("the round-%d audit archive is lost: %v", round, err)
		}
		for _, f := range reportdoc.Findings(string(raw)) {
			known = append(known, HostFindingID(f))
			locations[findingTitleKey(f)] = f.Location
		}
	}
	return known, locations, ""
}

func openPreviousFindings(in scopeInputs, locations map[string]string) []ScopeFinding {
	judgments := judgmentsOfRounds(loadArchivedAuditRounds(in.workspace, in.round, io.Discard))
	var previous []ScopeFinding
	for _, f := range judgments[len(judgments)-1].Findings {
		if f.Status == convergence.StatusOpen {
			previous = append(previous, ScopeFinding{HostID: defectledger.ID(f.ID), Severity: string(f.Severity), Title: f.ID, Location: locations[f.ID]})
		}
	}
	return previous
}

func auditorTrees(ctx context.Context, in scopeInputs) ([]string, string) {
	if in.ledger == nil {
		return nil, "the ledger is not wired"
	}
	it, err := in.ledger.Iter(ctx)
	if err != nil {
		return nil, "the ledger cannot be read: " + err.Error()
	}
	defer func() { _ = it.Close() }()
	var trees []string
	for {
		e, ok, err := it.Next()
		if err != nil {
			return nil, "the ledger cannot be read: " + err.Error()
		}
		if !ok {
			break
		}
		if e.Cycle == in.cycle && auditledger.IsAuditorRow(auditledger.Entry{Role: e.Role, Kind: e.Kind}) {
			trees = append(trees, e.WorktreeTreeSHA)
		}
	}
	if len(trees) < in.round-1 {
		return nil, fmt.Sprintf("the ledger holds %d auditor bindings, round %d needs %d", len(trees), in.round, in.round-1)
	}
	if trees[0] == "" || trees[in.round-2] == "" {
		return nil, "an auditor binding holds no worktree tree SHA"
	}
	return trees, ""
}

func diffPostImageHunks(ctx context.Context, dir, from, to string) ([]convergence.Hunk, error) {
	out, code, err := gitCapture(ctx, dir, "diff-tree", "-r", "-p", "-U0", "--no-color", "--no-renames", "--no-ext-diff", from, to)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff-tree %s %s exited %d", from, to, code)
	}
	return parsePostImageHunks(out), nil
}

var postImageHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func parsePostImageHunks(diff string) []convergence.Hunk {
	var hunks []convergence.Hunk
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		if rest, ok := strings.CutPrefix(line, "+++ "); ok {
			file = postImagePath(rest)
			continue
		}
		m := postImageHeader.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		hunks = append(hunks, convergence.Hunk{File: file, From: start, To: start + max(count, 1) - 1})
	}
	return hunks
}

func postImagePath(header string) string {
	header = strings.TrimRight(header, "\t")
	if header == "/dev/null" {
		return ""
	}
	if unquoted, err := strconv.Unquote(header); err == nil {
		header = unquoted
	}
	return strings.TrimPrefix(header, "b/")
}

func writeAuditScope(workspace string, scope AuditRoundScope) error {
	return atomicwrite.JSON(filepath.Join(workspace, phasecontract.RoundArchiveFilename(auditScopeFile, scope.Round)), scope)
}

func readAuditScope(workspace string, round int) *AuditRoundScope {
	return readAuditScopeFile(filepath.Join(workspace, phasecontract.RoundArchiveFilename(auditScopeFile, round)))
}
