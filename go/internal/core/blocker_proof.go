package core

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	BlockerKindProtectedPath     = "protected-path"
	BlockerKindPredicateConflict = "predicate-conflict"
	blockerDefectWord            = "BLOCKER"
	DiagCodeBlockerRoute         = cyclestate.DiagCodeBlockerRoute
)

type BlockerClaim struct {
	Kind    string
	Path    string
	Failing string
	Guard   string
	Defect  string
}

type BlockerCase struct {
	Phase               string
	Worktree            string
	PredicatesAtRequest map[string]string
	PredicatesNow       map[string]string
	Changed             []string
	ShrunkTests         []string
	Protected           func(string) bool
	Failures            []string
	Defects             []string
	Explainable         func(failure string) bool
	EvidenceFault       string
}

type BlockerProof struct {
	Proven   []BlockerClaim
	Refusals []string
}

func (p BlockerProof) Blocked() bool { return len(p.Proven) > 0 && len(p.Refusals) == 0 }

type BlockerProver interface {
	Prove(c BlockerCase) BlockerProof
}

type claimVerifier func(BlockerClaim, BlockerCase) string

type deterministicBlockerProver struct {
	verifiers map[string]claimVerifier
}

func NewBlockerProver() BlockerProver {
	return deterministicBlockerProver{verifiers: map[string]claimVerifier{
		BlockerKindProtectedPath:     verifyProtectedPathClaim,
		BlockerKindPredicateConflict: verifyPredicateConflictClaim,
	}}
}

func (p deterministicBlockerProver) Prove(c BlockerCase) BlockerProof {
	claims := parseBlockerClaims(c.Defects)
	if len(claims) == 0 || len(c.Failures) == 0 {
		return BlockerProof{}
	}
	if why := caseFault(c); why != "" {
		return BlockerProof{Refusals: []string{why}}
	}
	var proof BlockerProof
	for _, claim := range claims {
		if why := p.verify(claim, c); why != "" {
			proof.Refusals = append(proof.Refusals, fmt.Sprintf("%q: %s", claim.Defect, why))
			continue
		}
		proof.Proven = append(proof.Proven, claim)
	}
	if len(proof.Proven) == 0 {
		return proof
	}
	for _, f := range c.Failures {
		if !explainedFailure(f, proof.Proven, c.Explainable) {
			proof.Refusals = append(proof.Refusals, "no proven blocker explains the floor failure "+firstLine(f))
		}
	}
	return proof
}

func caseFault(c BlockerCase) string {
	switch {
	case c.Explainable == nil:
		return "the caller gave no failure kinds that a blocker can explain"
	case c.EvidenceFault != "":
		return "the evidence is not complete: " + c.EvidenceFault
	case !maps.Equal(c.PredicatesAtRequest, c.PredicatesNow):
		return "the lane changed the predicates of the cycle after the phase request, so no failure of theirs can prove a blocker"
	}
	return ""
}

func (p deterministicBlockerProver) verify(claim BlockerClaim, c BlockerCase) string {
	if claim.Kind == "" {
		return "malformed: write `BLOCKER protected-path <path>[:line] <why>` or `BLOCKER predicate-conflict <failing test> <guard test> <path>[:line] <why>`"
	}
	verify, ok := p.verifiers[claim.Kind]
	if !ok {
		return "unknown blocker kind " + claim.Kind
	}
	return verify(claim, c)
}

var blockerLineSuffix = regexp.MustCompile(`:[0-9][0-9,.-]*$`)

func parseBlockerClaims(defects []string) []BlockerClaim {
	var claims []BlockerClaim
	for _, d := range defects {
		fields := strings.Fields(d)
		if len(fields) == 0 || fields[0] != blockerDefectWord {
			continue
		}
		claims = append(claims, blockerClaimFrom(fields[1:], d))
	}
	return claims
}

func blockerClaimFrom(fields []string, defect string) BlockerClaim {
	malformed := BlockerClaim{Defect: defect}
	if len(fields) == 0 {
		return malformed
	}
	kind := fields[0]
	switch {
	case kind == BlockerKindProtectedPath && len(fields) >= 2:
		return BlockerClaim{Kind: kind, Path: claimPath(fields[1]), Defect: defect}
	case kind == BlockerKindPredicateConflict && len(fields) >= 4:
		return BlockerClaim{Kind: kind, Failing: fields[1], Guard: fields[2], Path: claimPath(fields[3]), Defect: defect}
	case kind == BlockerKindProtectedPath || kind == BlockerKindPredicateConflict:
		return malformed
	}
	return BlockerClaim{Kind: kind, Defect: defect}
}

func claimPath(token string) string {
	return blockerLineSuffix.ReplaceAllString(strings.Trim(token, "`"), "")
}

func verifyProtectedPathClaim(claim BlockerClaim, c BlockerCase) string {
	if c.Protected == nil {
		return "no protected-surface manifest is wired, so the path cannot be proven protected"
	}
	if why := claimTargetProblem(claim.Path, c); why != "" {
		return why
	}
	if !c.Protected(claim.Path) {
		return claim.Path + " is not on the protected-surface manifest, so the lane can edit it"
	}
	return ""
}

func verifyPredicateConflictClaim(claim BlockerClaim, c BlockerCase) string {
	if claim.Failing == claim.Guard {
		return "the claim names one predicate twice"
	}
	if why := claimTargetProblem(claim.Path, c); why != "" {
		return why
	}
	guard, ok := predicateBody(c.PredicatesAtRequest, claim.Guard)
	if _, failingOK := predicateBody(c.PredicatesAtRequest, claim.Failing); !failingOK || !ok {
		return "a named test is not a predicate of this cycle"
	}
	if !anyFailureNamesTest(c.Failures, claim.Failing) {
		return claim.Failing + " does not fail at the floor"
	}
	if anyFailureNamesTest(c.Failures, claim.Guard) {
		return claim.Guard + " fails at the floor too, so it is not a guard that holds"
	}
	dir := path.Dir(claim.Path)
	if !strings.Contains(guard, dir) && !strings.Contains(guard, strings.TrimPrefix(dir, "go/")) {
		return claim.Guard + " does not guard " + dir
	}
	return ""
}

func claimTargetProblem(p string, c BlockerCase) string {
	if !fs.ValidPath(p) || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
		return p + " is not a non-test Go source file in the tree"
	}
	if info, err := os.Lstat(filepath.Join(c.Worktree, filepath.FromSlash(p))); err != nil || !info.Mode().IsRegular() {
		return p + " does not exist in the worktree as a regular file"
	}
	if slices.Contains(c.Changed, p) {
		return p + " is in the diff: the lane changed it, so restore it first"
	}
	if slices.ContainsFunc(c.ShrunkTests, func(t string) bool { return path.Dir(t) == path.Dir(p) }) {
		return "the lane removed test lines in " + path.Dir(p) + ", so the gap can be the work of the lane"
	}
	return ""
}

func predicateBody(sources map[string]string, name string) (string, bool) {
	files := slices.Sorted(maps.Keys(sources))
	for _, f := range files {
		src := sources[f]
		start := strings.Index("\n"+src, "\nfunc "+name+"(")
		if start < 0 {
			continue
		}
		body := src[start:]
		if end := strings.Index(body[1:], "\nfunc "); end >= 0 {
			body = body[:end+1]
		}
		return body, true
	}
	return "", false
}

func anyFailureNamesTest(failures []string, name string) bool {
	for _, f := range failures {
		if strings.Contains(f, "--- FAIL: "+name+" ") || strings.Contains(f, "--- FAIL: "+name+"\n") || strings.Contains(f, "--- FAIL: "+name+"/") {
			return true
		}
	}
	return false
}

var goSourceToken = regexp.MustCompile(`[A-Za-z0-9_./-]+\.go\b`)

var failureItemStart = regexp.MustCompile(`(?m)^\s*(--- FAIL: |cover strict: )`)

func explainedFailure(f string, proven []BlockerClaim, explainable func(string) bool) bool {
	if strings.Contains(f, "…") || !explainable(f) {
		return false
	}
	starts := failureItemStart.FindAllStringIndex(f, -1)
	if len(starts) == 0 {
		named, ok := namesOnlyProvenFiles(f, proven)
		return ok && named > 0
	}
	if _, ok := namesOnlyProvenFiles(f[:starts[0][0]], proven); !ok {
		return false
	}
	for i, s := range starts {
		end := len(f)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		if named, ok := namesOnlyProvenFiles(f[s[0]:end], proven); !ok || named == 0 {
			return false
		}
	}
	return true
}

func namesOnlyProvenFiles(text string, proven []BlockerClaim) (int, bool) {
	named := 0
	for _, token := range goSourceToken.FindAllString(text, -1) {
		if strings.HasSuffix(token, "_test.go") {
			continue
		}
		named++
		if !slices.ContainsFunc(proven, func(c BlockerClaim) bool { return samePathTail(token, c.Path) }) {
			return named, false
		}
	}
	return named, true
}

func samePathTail(token, p string) bool {
	return token == p || strings.HasSuffix(token, "/"+p) || strings.HasSuffix(p, "/"+token)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func BlockerRouteDiagnostic(phase, item string, proof BlockerProof) Diagnostic {
	parts := make([]string, 0, len(proof.Proven))
	for _, c := range proof.Proven {
		parts = append(parts, c.summary())
	}
	return blockerRouteDiagnostic(item, "blocked: needs console — phase "+phase+" stopped on a proven blocker the lane cannot remove: "+strings.Join(parts, "; "))
}

func blockerRouteDiagnostic(item, message string) Diagnostic {
	return Diagnostic{Severity: cyclestate.SeverityError, Code: DiagCodeBlockerRoute, Subject: item, Message: message}
}

func (c BlockerClaim) summary() string {
	if c.Kind == BlockerKindPredicateConflict {
		return fmt.Sprintf("%s: %s fails at %s and %s forbids the edit that fixes it (the lane said: %s)", c.Kind, c.Failing, c.Path, c.Guard, c.Defect)
	}
	return fmt.Sprintf("%s: the fix needs %s, a protected control-plane path (the lane said: %s)", c.Kind, c.Path, c.Defect)
}
