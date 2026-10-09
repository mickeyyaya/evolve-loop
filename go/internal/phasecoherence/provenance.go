package phasecoherence

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// ProvenanceFields holds the expected header values; an empty TreeSHA or InputsDigest is not checked.
type ProvenanceFields struct {
	Phase        string
	Cycle        int
	TreeSHA      string
	InputsDigest string
}

var provenanceRegex = regexp.MustCompile(`<!--\s*evolve:provenance\s+([^>]+)\s*-->`)
var kvRegex = regexp.MustCompile(`(\w+)=(\S+)`)

// CheckProvenance reports each evolve:provenance header field that disagrees with expected,
// and cross-checks tree_sha against the ledger that the process environment resolves.
func CheckProvenance(artifact string, expected ProvenanceFields) ([]Violation, error) {
	matches := provenanceRegex.FindStringSubmatch(artifact)
	if len(matches) < 2 {
		return []Violation{{
			Severity: SeverityWarn,
			Kind:     "missing-provenance",
			Message:  "missing evolve:provenance header",
		}}, nil
	}

	parsed := parseProvenanceKV(matches[1])
	violations, hasDirectTreeSHAMismatch := checkProvenanceFields(parsed, expected)
	ledgerViolations, err := checkLedgerTreeSHA(expected, parsed["tree_sha"], hasDirectTreeSHAMismatch)
	if err != nil {
		return nil, err
	}
	return append(violations, ledgerViolations...), nil
}

func parseProvenanceKV(inner string) map[string]string {
	kvs := kvRegex.FindAllStringSubmatch(inner, -1)
	parsed := make(map[string]string)
	for _, kv := range kvs {
		parsed[kv[1]] = kv[2]
	}
	return parsed
}

func checkProvenanceFields(parsed map[string]string, expected ProvenanceFields) ([]Violation, bool) {
	var violations []Violation

	if phase := parsed["phase"]; phase != expected.Phase {
		violations = append(violations, Violation{
			Severity: SeverityError,
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("phase mismatch: got %q, want %q", phase, expected.Phase),
		})
	}

	cycleStr := parsed["cycle"]
	if cycle, err := strconv.Atoi(cycleStr); err != nil || cycle != expected.Cycle {
		violations = append(violations, Violation{
			Severity: SeverityError,
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("cycle mismatch: got %q, want %d", cycleStr, expected.Cycle),
		})
	}

	if inputsDigest := parsed["inputs_digest"]; expected.InputsDigest != "" && inputsDigest != expected.InputsDigest {
		violations = append(violations, Violation{
			Severity: SeverityError,
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("inputs_digest mismatch: got %q, want %q", inputsDigest, expected.InputsDigest),
		})
	}

	treeSHA := parsed["tree_sha"]
	hasDirectTreeSHAMismatch := expected.TreeSHA != "" && treeSHA != expected.TreeSHA
	if hasDirectTreeSHAMismatch {
		violations = append(violations, Violation{
			Severity: SeverityError,
			Kind:     "provenance-mismatch",
			Message:  fmt.Sprintf("tree_sha mismatch: got %q, want %q", treeSHA, expected.TreeSHA),
		})
	}

	return violations, hasDirectTreeSHAMismatch
}

const maxLedgerLineBytes = 64 << 20

type ledgerScan struct {
	foundEntry bool
	treeSHA    string
	malformed  int
}

func checkLedgerTreeSHA(expected ProvenanceFields, treeSHA string, hasDirectTreeSHAMismatch bool) ([]Violation, error) {
	ledgerPath := paths.Resolve(os.Getenv, "").LedgerFile
	scan, err := scanLedger(ledgerPath, expected)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var violations []Violation
	if scan.malformed > 0 {
		violations = append(violations, Violation{
			Severity: SeverityWarn,
			Kind:     "malformed-ledger",
			Message:  fmt.Sprintf("ledger %s: %d malformed line(s) skipped by the tree_sha cross-check", ledgerPath, scan.malformed),
		})
	}
	if hasDirectTreeSHAMismatch || !scan.foundEntry || scan.treeSHA == "" || treeSHA == scan.treeSHA {
		return violations, nil
	}
	return append(violations, Violation{
		Severity: SeverityError,
		Kind:     "provenance-mismatch",
		Message:  fmt.Sprintf("tree_sha mismatch against ledger: got %q, ledger has %q", treeSHA, scan.treeSHA),
	}), nil
}

func scanLedger(ledgerPath string, expected ProvenanceFields) (ledgerScan, error) {
	var scan ledgerScan
	f, err := os.Open(ledgerPath)
	if errors.Is(err, os.ErrNotExist) {
		return scan, err
	}
	if err != nil {
		return scan, fmt.Errorf("open ledger %s: %w", ledgerPath, err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLedgerLineBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Cycle        int    `json:"cycle"`
			Role         string `json:"role"`
			TreeStateSHA string `json:"tree_state_sha"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			scan.malformed++
			continue
		}
		if entry.Cycle == expected.Cycle && canonicalRole(entry.Role) == canonicalRole(expected.Phase) {
			scan.foundEntry = true
			if entry.TreeStateSHA != "" {
				scan.treeSHA = entry.TreeStateSHA
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return scan, fmt.Errorf("scan ledger %s: %w", ledgerPath, err)
	}
	return scan, nil
}

func canonicalRole(role string) string {
	switch lower := strings.ToLower(role); lower {
	case "builder", "build":
		return "builder"
	case "auditor", "audit":
		return "auditor"
	default:
		return lower
	}
}
