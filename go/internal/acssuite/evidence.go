package acssuite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// EvidenceIdentity comes from the host dispatch, never the predicate artifact.
// TreeSHA is the full Git tree tested by this audit round.
type EvidenceIdentity struct {
	Cycle   int    `json:"cycle"`
	RunID   string `json:"run_id"`
	Round   int    `json:"round"`
	TreeSHA string `json:"tree_sha"`
}

type evidenceSeal struct {
	Version int `json:"version"`
	EvidenceIdentity
	VerdictSHA256   string `json:"verdict_sha256"`
	InventorySHA256 string `json:"inventory_sha256"`
}

const evidencePrefix = "<!-- evolve-acs-evidence: "

var evidencePattern = regexp.MustCompile(`<!-- evolve-acs-evidence: ([^\r\n]*?) -->`)

// ReadVerdict validates the complete execution result rather than interpreting
// missing fields as successful zero values. Historical records remain readable
// elsewhere; an incomplete historical record cannot authorize a new ship.
func ReadVerdict(raw []byte) (Verdict, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Verdict{}, fmt.Errorf("predicate verdict JSON: %w", err)
	}
	for _, name := range []string{"schema_version", "cycle", "predicate_suite", "results", "green_count", "red_count", "skip_count", "red_ids", "verdict", "ship_eligible"} {
		if value, ok := fields[name]; !ok || string(value) == "null" && name != "red_ids" {
			return Verdict{}, fmt.Errorf("predicate verdict missing required %s", name)
		}
	}
	var v Verdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return Verdict{}, fmt.Errorf("predicate verdict fields: %w", err)
	}
	if v.SchemaVersion != "1.0" || v.Cycle <= 0 || len(v.Results) == 0 {
		return Verdict{}, fmt.Errorf("predicate verdict requires schema 1.0, a positive cycle, and a nonempty execution result set")
	}
	counted := Verdict{}
	seen := map[string]bool{}
	for _, r := range v.Results {
		key := r.Predicate + "\x00" + r.ACID
		if r.ACID == "" || r.Predicate == "" || seen[key] {
			return Verdict{}, fmt.Errorf("predicate verdict has missing or duplicate result identity %q", r.ACID)
		}
		seen[key] = true
		if r.ResultStr != "green" && r.ResultStr != "red" && r.ResultStr != "skip" ||
			r.ResultStr == "green" && r.ExitCode != 0 ||
			r.ResultStr == "red" && (r.ExitCode == 0 || r.ExitCode == SkipExitCode) ||
			r.ResultStr == "skip" && r.ExitCode != SkipExitCode {
			return Verdict{}, fmt.Errorf("predicate %s has inconsistent outcome and exit code", r.ACID)
		}
		counted.record(r)
	}
	counted.PredicateSuite.Total = len(v.Results)
	counted.PredicateSuite.SkippedCount = counted.SkipCount
	if v.GreenCount != counted.GreenCount || v.RedCount != counted.RedCount || v.SkipCount != counted.SkipCount ||
		v.PredicateSuite != counted.PredicateSuite || !slices.Equal(v.RedIDs, counted.RedIDs) || !slices.Equal(v.SkipIDs, counted.SkipIDs) {
		return Verdict{}, fmt.Errorf("predicate verdict counts, IDs, or inventory disagree with execution results")
	}
	want := "PASS"
	if v.RedCount > 0 {
		want = "FAIL"
	}
	if v.Verdict != want || v.ShipEligible != (v.RedCount == 0) {
		return Verdict{}, fmt.Errorf("predicate verdict or ship_eligible contradicts execution results")
	}
	return v, nil
}

func inventorySHA(v Verdict) string {
	ids := make([]string, 0, len(v.Results))
	for _, r := range v.Results {
		ids = append(ids, r.Predicate+"\x00"+r.ACID)
	}
	slices.Sort(ids)
	raw, _ := json.Marshal(ids)
	return digest(raw)
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func validEvidenceIdentity(id EvidenceIdentity) error {
	decoded, err := hex.DecodeString(id.TreeSHA)
	if id.Cycle <= 0 || strings.TrimSpace(id.RunID) == "" || id.Round <= 0 || err != nil || len(decoded) != 20 {
		return fmt.Errorf("predicate evidence requires host cycle, run, audit round, and Git tree identity; re-run Audit")
	}
	return nil
}

// InvalidateEvidence retires every candidate receipt before host verification.
// Even malformed agent input loses the reserved marker. Failure must prevent
// the orchestrator from recording a ship-eligible audit result.
func InvalidateEvidence(reportPath string) error {
	body, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("retire candidate predicate receipt: %w", err)
	}
	retired := strings.ReplaceAll(string(body), "<!-- evolve-acs-evidence:", "<!-- candidate-acs-evidence:")
	if retired == string(body) {
		return nil
	}
	return atomicwrite.Bytes(reportPath, []byte(retired))
}

// SealEvidence replaces any auditor-authored receipt after host execution.
// The orchestrator subsequently hashes this exact audit report into its ledger,
// binding the verdict without creating another independently mutable authority.
func SealEvidence(reportPath string, raw []byte, id EvidenceIdentity) error {
	if err := validEvidenceIdentity(id); err != nil {
		return err
	}
	v, err := ReadVerdict(raw)
	if err != nil {
		return err
	}
	if v.Cycle != id.Cycle {
		return fmt.Errorf("predicate verdict cycle %d differs from host cycle %d", v.Cycle, id.Cycle)
	}
	seal := evidenceSeal{Version: 1, EvidenceIdentity: id, VerdictSHA256: digest(raw), InventorySHA256: inventorySHA(v)}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("read audit report for predicate binding: %w", err)
	}
	body = evidencePattern.ReplaceAll(body, nil)
	if strings.Contains(string(body), "<!-- evolve-acs-evidence:") {
		return fmt.Errorf("malformed auditor-authored predicate receipt")
	}
	encoded, err := json.Marshal(seal)
	if err != nil {
		return err
	}
	body = append(body, []byte("\n"+evidencePrefix+string(encoded)+" -->\n")...)
	return atomicwrite.Bytes(reportPath, body)
}

// VerifyEvidence consumes a report whose SHA has already been verified against
// the host ledger. Matching a receipt supplied by an agent without that outer
// binding is never proof of execution.
func VerifyEvidence(report string, raw []byte, id EvidenceIdentity) (Verdict, error) {
	if err := validEvidenceIdentity(id); err != nil {
		return Verdict{}, err
	}
	matches := evidencePattern.FindAllStringSubmatch(report, -1)
	if len(matches) != 1 || strings.Count(report, "<!-- evolve-acs-evidence:") != 1 {
		return Verdict{}, fmt.Errorf("audit report requires exactly one host predicate receipt; re-run Audit")
	}
	var seal evidenceSeal
	if err := json.Unmarshal([]byte(matches[0][1]), &seal); err != nil {
		return Verdict{}, fmt.Errorf("predicate receipt: %w", err)
	}
	if seal.Version != 1 || seal.EvidenceIdentity != id {
		return Verdict{}, fmt.Errorf("predicate receipt differs from host cycle/run/round/tree identity")
	}
	v, err := ReadVerdict(raw)
	if err != nil {
		return Verdict{}, err
	}
	if v.Cycle != id.Cycle || seal.VerdictSHA256 != digest(raw) || seal.InventorySHA256 != inventorySHA(v) {
		return Verdict{}, fmt.Errorf("predicate verdict or inventory changed after host audit")
	}
	return v, nil
}
