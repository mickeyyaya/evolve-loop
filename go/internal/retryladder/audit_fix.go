package retryladder

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const RungAuditFix Rung = "audit_fix"

const (
	AuditFixRuleSmallPrescribed = "small_prescribed"
	AuditFixRuleUsed            = "used_this_cycle"
	AuditFixRuleNoFindings      = "no_findings"
	AuditFixRuleUnprescribed    = "unprescribed"
	AuditFixRuleFiles           = "files_over_cap"
	AuditFixRuleLines           = "lines_over_cap"
	AuditFixRuleCodeLines       = "production_logic"
	AuditFixRuleTokens          = "tokens_over_cap"
	AuditFixRuleUncited         = "uncited"

	AuditFixLinesPerUnspannedFile = 10
	AuditFixTokensPerLine         = 12
)

type AuditFixBounds struct {
	MaxFiles             int
	MaxLines             int
	MaxCodeLines         int
	MaxTokens            int
	NoNewProductionLogic bool
}

type AuditFixEstimate struct {
	Files     int
	Lines     int
	CodeLines int
	Tokens    int
	Uncited   int
}

type AuditFixInput struct {
	Findings      int
	Prescriptions int
	AllPrescribed bool
	Estimate      AuditFixEstimate
	Bounds        AuditFixBounds
	Used          bool
}

type AuditFixChoice struct {
	Choose bool
	Rule   string
	Input  AuditFixInput
}

func ChooseAuditFix(in AuditFixInput) AuditFixChoice {
	rule := auditFixRule(in)
	return AuditFixChoice{Choose: rule == AuditFixRuleSmallPrescribed, Rule: rule, Input: in}
}

func auditFixRule(in AuditFixInput) string {
	switch {
	case in.Used:
		return AuditFixRuleUsed
	case in.Findings == 0:
		return AuditFixRuleNoFindings
	case !in.AllPrescribed:
		return AuditFixRuleUnprescribed
	case in.Estimate.Uncited > 0:
		return AuditFixRuleUncited
	}
	return cmp.Or(AuditFixOverBounds(in.Estimate, in.Bounds), AuditFixRuleSmallPrescribed)
}

func AuditFixOverBounds(est AuditFixEstimate, b AuditFixBounds) string {
	switch {
	case est.Files > b.MaxFiles:
		return AuditFixRuleFiles
	case est.Lines > b.MaxLines:
		return AuditFixRuleLines
	case b.NoNewProductionLogic && est.CodeLines > b.MaxCodeLines:
		return AuditFixRuleCodeLines
	case est.Tokens > b.MaxTokens:
		return AuditFixRuleTokens
	}
	return ""
}

func AllPrescribed(findings, prescriptions []string) bool {
	if len(findings) == 0 || len(prescriptions) < len(findings) {
		return false
	}
	for _, p := range prescriptions {
		if strings.TrimSpace(p) == "" {
			return false
		}
	}
	return true
}

var citedPath = regexp.MustCompile(`([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*\.(?:go|golden|md|json|yaml|yml|txt)\b)(?::(\d+)(?:-(\d+))?)?`)

func EstimateAuditFix(findings, prescriptions []string) AuditFixEstimate {
	spans := map[string]int{}
	for _, text := range append(append([]string(nil), findings...), prescriptions...) {
		for _, m := range citedPath.FindAllStringSubmatch(text, -1) {
			spans[m[1]] = max(spans[m[1]], spanLines(m[2], m[3]))
		}
	}
	for file, lines := range spans {
		if lines == 0 {
			spans[file] = AuditFixLinesPerUnspannedFile
		}
	}
	est := EstimatePatch(spans)
	for _, finding := range findings {
		if len(CitedPaths(finding)) == 0 {
			est.Uncited++
		}
	}
	return est
}

func EstimatePatch(changed map[string]int) AuditFixEstimate {
	var est AuditFixEstimate
	for file, lines := range changed {
		est.Files++
		est.Lines += lines
		if isProductionCode(file) {
			est.CodeLines += lines
		}
	}
	est.Tokens = est.Lines * AuditFixTokensPerLine
	return est
}

func CitedPaths(texts ...string) []string {
	var paths []string
	for _, text := range texts {
		for _, m := range citedPath.FindAllStringSubmatch(text, -1) {
			paths = append(paths, m[1])
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func spanLines(from, to string) int {
	start, err := strconv.Atoi(from)
	if err != nil {
		return 0
	}
	end, err := strconv.Atoi(to)
	if err != nil {
		return 1
	}
	return max(1, end-start+1)
}

func isProductionCode(file string) bool {
	return path.Ext(file) == ".go" && !strings.HasSuffix(file, "_test.go")
}
