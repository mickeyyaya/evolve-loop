package phasecoherence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type ProvenanceFields struct {
	Phase        string
	Cycle        int
	TreeSHA      string
	InputsDigest string
}

var provenanceRegex = regexp.MustCompile(`<!--\s*evolve:provenance\s+([^>]+)\s*-->`)
var kvRegex = regexp.MustCompile(`(\w+)=(\S+)`)

func CheckProvenance(artifact string, expected ProvenanceFields) []Violation {
	var violations []Violation

	matches := provenanceRegex.FindStringSubmatch(artifact)
	if len(matches) < 2 {
		violations = append(violations, Violation{
			Severity: "WARN",
			Kind:     "missing-provenance",
			Message:  "missing evolve:provenance header",
		})
		return violations
	}

	inner := matches[1]
	kvs := kvRegex.FindAllStringSubmatch(inner, -1)
	parsed := make(map[string]string)
	for _, kv := range kvs {
		parsed[kv[1]] = kv[2]
	}

	phase := parsed["phase"]
	cycleStr := parsed["cycle"]
	treeSHA := parsed["tree_sha"]
	inputsDigest := parsed["inputs_digest"]

	if phase != expected.Phase {
		violations = append(violations, Violation{
			Severity: "error",
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("phase mismatch: got %q, want %q", phase, expected.Phase),
		})
	}

	cycle, err := strconv.Atoi(cycleStr)
	if err != nil || cycle != expected.Cycle {
		violations = append(violations, Violation{
			Severity: "error",
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("cycle mismatch: got %q, want %d", cycleStr, expected.Cycle),
		})
	}

	if expected.InputsDigest != "" && inputsDigest != expected.InputsDigest {
		violations = append(violations, Violation{
			Severity: "error",
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("inputs_digest mismatch: got %q, want %q", inputsDigest, expected.InputsDigest),
		})
	}

	hasDirectTreeSHAMismatch := false
	if expected.TreeSHA != "" && treeSHA != expected.TreeSHA {
		hasDirectTreeSHAMismatch = true
		violations = append(violations, Violation{
			Severity: "error",
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("tree_sha mismatch: got %q, want %q", treeSHA, expected.TreeSHA),
		})
	}

	// cross-check tree_sha against ledger when available
	layout := paths.Resolve(os.Getenv, "")
	ledgerFile := layout.LedgerFile
	if _, err := os.Stat(ledgerFile); err == nil {
		if f, err := os.Open(ledgerFile); err == nil {
			defer func() { _ = f.Close() }()
			scanner := bufio.NewScanner(f)
			foundEntry := false
			var ledgerTreeSHA string
			for scanner.Scan() {
				var entry struct {
					Cycle        int    `json:"cycle"`
					Role         string `json:"role"`
					TreeStateSHA string `json:"tree_state_sha"`
				}
				if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
					if entry.Cycle == expected.Cycle && canonicalRole(entry.Role) == canonicalRole(expected.Phase) {
						foundEntry = true
						if entry.TreeStateSHA != "" {
							ledgerTreeSHA = entry.TreeStateSHA
						}
					}
				}
			}
			if !hasDirectTreeSHAMismatch && foundEntry && ledgerTreeSHA != "" && treeSHA != ledgerTreeSHA {
				violations = append(violations, Violation{
					Severity: "error",
					Kind:     "provenance-mismatch",
					Message:  fmt.Sprintf("tree_sha mismatch against ledger: got %q, ledger has %q", treeSHA, ledgerTreeSHA),
				})
			}
		}
	}

	return violations
}

func canonicalRole(role string) string {
	switch role {
	case "scout":
		return "scout"
	case "builder", "build":
		return "builder"
	case "auditor", "audit":
		return "auditor"
	case "intent":
		return "intent"
	case "memo":
		return "memo"
	default:
		return strings.ToLower(role)
	}
}
