package coherence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// InvariantStatus is one invariant's outcome: ok, violated, or indeterminate.
type InvariantStatus string

const (
	// InvariantOK means the invariant was evaluated and holds.
	InvariantOK InvariantStatus = "ok"
	// InvariantViolated means the invariant was evaluated and does NOT hold.
	InvariantViolated InvariantStatus = "violated"
	// InvariantIndeterminate means the artifacts were absent, malformed or silent: never a match, never a violation.
	InvariantIndeterminate InvariantStatus = "indeterminate"
)

const (
	// InvariantVerdictAgreement names the embedded-sentinel vs standalone-verdict check.
	InvariantVerdictAgreement = "verdict-agreement"
	// InvariantTestCounts names the claimed-counts vs independent-recount check.
	InvariantTestCounts = "test-count-agreement"
	// InvariantReferencedPaths names the cited-evidence-path existence check.
	InvariantReferencedPaths = "referenced-paths-exist"
	// InvariantPhaseOrder names the provenance / phase-order check.
	InvariantPhaseOrder = "provenance-phase-order"
)

// Invariant is one weak verifier's finding; a non-ok status always carries Evidence.
type Invariant struct {
	Name     string          `json:"name"`
	Status   InvariantStatus `json:"status"`
	Evidence string          `json:"evidence"`
}

// InvariantReport is the per-cycle aggregate: the four invariants in declared order, always advisory.
type InvariantReport struct {
	Advisory   bool        `json:"advisory"`
	Invariants []Invariant `json:"invariants"`
}

// Violations returns only the violated invariants, in report order; indeterminates never appear.
func (r InvariantReport) Violations() []Invariant {
	var out []Invariant
	for _, inv := range r.Invariants {
		if inv.Status == InvariantViolated {
			out = append(out, inv)
		}
	}
	return out
}

// CheckCrossArtifactInvariants evaluates the four invariants over a cycle workspace, resolving cited paths under it and then the lane worktree.
func CheckCrossArtifactInvariants(workspace, worktree string) InvariantReport {
	sentinel, sentinelOK := readAuditSentinel(workspace)
	acs, acsOK := readACSArtifact(workspace)
	return InvariantReport{
		Advisory: true,
		Invariants: []Invariant{
			checkVerdictAgreement(sentinel, sentinelOK, acs, acsOK),
			checkTestCounts(acs, acsOK),
			checkReferencedPaths(sentinel, sentinelOK, workspace, worktree),
			checkPhaseOrder(workspace),
		},
	}
}

func readAuditSentinel(workspace string) (phasecontract.VerdictSentinel, bool) {
	b, err := os.ReadFile(filepath.Join(workspace, phasecontract.ArtifactFilename("audit")))
	if err != nil {
		return phasecontract.VerdictSentinel{}, false
	}
	return phasecontract.ParseVerdictSentinelFull(string(b))
}

// acsArtifact is decoded locally: importing internal/acssuite would pull gitexec,
// sysexec, changedpkgs and verifylock into this package.
type acsArtifact struct {
	PredicateSuite struct {
		Total int `json:"total"`
	} `json:"predicate_suite"`
	Results []struct {
		Result string `json:"result"`
	} `json:"results"`
	GreenCount int    `json:"green_count"`
	RedCount   int    `json:"red_count"`
	SkipCount  int    `json:"skip_count"`
	Verdict    string `json:"verdict"`
}

func readACSArtifact(workspace string) (acsArtifact, bool) {
	b, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json"))
	if err != nil {
		return acsArtifact{}, false
	}
	var a acsArtifact
	if json.Unmarshal(b, &a) != nil {
		return acsArtifact{}, false
	}
	return a, true
}

func checkVerdictAgreement(s phasecontract.VerdictSentinel, sentinelOK bool, a acsArtifact, acsOK bool) Invariant {
	sv := strings.ToUpper(strings.TrimSpace(s.Verdict))
	av := strings.ToUpper(strings.TrimSpace(a.Verdict))
	switch {
	case !sentinelOK || sv == "":
		return indeterminate(InvariantVerdictAgreement,
			"audit-report.md carries no parseable evolve-verdict sentinel — nothing to compare (an arbitrary PASS in prose is not a verdict)")
	case !acsOK || av == "":
		return indeterminate(InvariantVerdictAgreement,
			"acs-verdict.json is absent, unparseable, or carries no verdict — nothing to compare")
	case sv != av:
		return violated(InvariantVerdictAgreement, fmt.Sprintf(
			"the audit-report.md sentinel records verdict=%s while the standalone acs-verdict.json records verdict=%s — the embedded and standalone verdicts disagree", sv, av))
	}
	return ok(InvariantVerdictAgreement, fmt.Sprintf("sentinel verdict=%s agrees with the standalone verdict=%s", sv, av))
}

func checkTestCounts(a acsArtifact, acsOK bool) Invariant {
	if !acsOK {
		return indeterminate(InvariantTestCounts, "acs-verdict.json is absent or unparseable — nothing to recount")
	}
	if len(a.Results) == 0 {
		return indeterminate(InvariantTestCounts, "acs-verdict.json carries no results[] entries — nothing to recount")
	}
	var green, red, skip int
	for _, r := range a.Results {
		switch strings.ToLower(strings.TrimSpace(r.Result)) {
		case "green":
			green++
		case "red":
			red++
		case "skip":
			skip++
		}
	}
	var disagreements []string
	if a.GreenCount != green {
		disagreements = append(disagreements, "green")
	}
	if a.RedCount != red {
		disagreements = append(disagreements, "red")
	}
	if a.SkipCount != skip {
		disagreements = append(disagreements, "skip")
	}
	if a.PredicateSuite.Total != len(a.Results) {
		disagreements = append(disagreements, "predicate_suite.total")
	}
	claim := fmt.Sprintf("acs-verdict.json claims green=%d red=%d skip=%d over predicate_suite.total=%d; an independent recount of its own %d result(s) gives green=%d red=%d skip=%d",
		a.GreenCount, a.RedCount, a.SkipCount, a.PredicateSuite.Total, len(a.Results), green, red, skip)
	if len(disagreements) > 0 {
		return violated(InvariantTestCounts, claim+" — disagreeing: "+strings.Join(disagreements, ", "))
	}
	return ok(InvariantTestCounts, claim)
}

func checkReferencedPaths(s phasecontract.VerdictSentinel, sentinelOK bool, workspace, worktree string) Invariant {
	if !sentinelOK || s.Failure == nil {
		return indeterminate(InvariantReferencedPaths,
			"audit-report.md carries no parseable sentinel failure block — no evidence paths are cited")
	}
	var cited, missing []string
	for _, p := range s.Failure.EvidencePaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cited = append(cited, p)
		if !resolvesUnder(workspace, p) && !resolvesUnder(worktree, p) {
			missing = append(missing, p)
		}
	}
	if len(cited) == 0 {
		return indeterminate(InvariantReferencedPaths, "the sentinel failure block cites no evidence paths — nothing to resolve")
	}
	if len(missing) > 0 {
		return violated(InvariantReferencedPaths, fmt.Sprintf(
			"%d of %d cited evidence path(s) resolve under neither the cycle workspace nor the lane worktree: %s",
			len(missing), len(cited), strings.Join(missing, ", ")))
	}
	return ok(InvariantReferencedPaths, fmt.Sprintf("all %d cited evidence path(s) resolve under the cycle workspace or the lane worktree", len(cited)))
}

// resolvesUnder checks containment before the stat: filepath.Join cleans "../"
// segments, so an escaping citation could otherwise resolve to a real file outside root.
func resolvesUnder(root, rel string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	joined := filepath.Join(root, rel)
	inside, err := filepath.Rel(root, joined)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return false
	}
	_, err = os.Stat(joined)
	return err == nil
}

type timedPhase struct {
	phase   string
	started time.Time
	ended   time.Time
}

// checkPhaseOrder compares only the first build and first audit, so re-dispatched build/audit rounds stay ok.
func checkPhaseOrder(workspace string) Invariant {
	entries, err := phasetiming.Read(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return indeterminate(InvariantPhaseOrder, "phase-timing.json is absent — no recorded chain to validate")
		}
		return indeterminate(InvariantPhaseOrder, "phase-timing.json is not a parseable array of phase entries — no recorded chain to validate")
	}
	var chain []timedPhase
	for i, e := range entries {
		if e.StartedAt == "" || e.EndedAt == "" {
			continue // timestamps are optional in the wire shape; an untimed entry orders nothing
		}
		started, serr := time.Parse(time.RFC3339, e.StartedAt)
		ended, eerr := time.Parse(time.RFC3339, e.EndedAt)
		if serr != nil || eerr != nil {
			return indeterminate(InvariantPhaseOrder, fmt.Sprintf(
				"phase-timing.json entry %d (%s) carries timestamps that do not parse as RFC3339 (started_at=%q, ended_at=%q) — the chain cannot be ordered",
				i, e.Phase, e.StartedAt, e.EndedAt))
		}
		chain = append(chain, timedPhase{phase: e.Phase, started: started, ended: ended})
	}
	if len(chain) < 2 {
		return indeterminate(InvariantPhaseOrder, "phase-timing.json records fewer than two timestamped phases — nothing to order")
	}
	for i, e := range chain {
		if e.ended.Before(e.started) {
			return violated(InvariantPhaseOrder, fmt.Sprintf(
				"phase-timing.json entry %d (%s) ends at %s, before it starts at %s",
				i, e.phase, e.ended.Format(time.RFC3339), e.started.Format(time.RFC3339)))
		}
		if i > 0 && e.started.Before(chain[i-1].started) {
			return violated(InvariantPhaseOrder, fmt.Sprintf(
				"phase-timing.json runs backwards: entry %d (%s) starts at %s, before entry %d (%s) at %s",
				i, e.phase, e.started.Format(time.RFC3339), i-1, chain[i-1].phase, chain[i-1].started.Format(time.RFC3339)))
		}
	}
	build, hasBuild := firstPhase(chain, "build")
	audit, hasAudit := firstPhase(chain, "audit")
	if hasBuild && hasAudit && audit.started.Before(build.started) {
		return violated(InvariantPhaseOrder, fmt.Sprintf(
			"phase-timing.json records the first audit at %s, before the first build at %s — an audit cannot precede the build it audits",
			audit.started.Format(time.RFC3339), build.started.Format(time.RFC3339)))
	}
	return ok(InvariantPhaseOrder, fmt.Sprintf("the recorded %d-phase chain runs forward and respects the build→audit floor", len(chain)))
}

func firstPhase(chain []timedPhase, name string) (timedPhase, bool) {
	for _, e := range chain {
		if e.phase == name {
			return e, true
		}
	}
	return timedPhase{}, false
}

func ok(name, evidence string) Invariant {
	return Invariant{Name: name, Status: InvariantOK, Evidence: evidence}
}

func violated(name, evidence string) Invariant {
	return Invariant{Name: name, Status: InvariantViolated, Evidence: evidence}
}

func indeterminate(name, evidence string) Invariant {
	return Invariant{Name: name, Status: InvariantIndeterminate, Evidence: evidence}
}
