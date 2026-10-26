package heldproposal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const (
	StatusWouldHold = "would-hold"
	StatusHeld      = "held-for-operator"
	StatusApproved  = "approved"
	StatusRejected  = "rejected"

	DecisionApprove = "approve"
	DecisionReject  = "reject"

	RequirementOperator         = "operator-approval"
	RequirementAdversarialAudit = "adversarial-audit: a family other than the builder, at the deep tier"

	pipelineKindPrefix = "pipeline-"
	recordPrefix       = "cycle-"
)

var (
	ErrBadApproval = errors.New("heldproposal: an approval needs a cycle above 0, a tree digest and the decision approve or reject")
	ErrBadRecord   = errors.New("heldproposal: a hold record needs a cycle above 0, a tree digest and a status")
)

type Inputs struct {
	Kinds     []string
	Changed   []string
	Protected func(path string) bool
}

type Decision struct {
	Hold    bool
	Reasons []string
}

func Decide(in Inputs) Decision {
	var reasons []string
	for _, kind := range in.Kinds {
		if k := strings.ToLower(strings.TrimSpace(kind)); strings.HasPrefix(k, pipelineKindPrefix) {
			reasons = append(reasons, "kind:"+k)
		}
	}
	for _, path := range in.Changed {
		if in.Protected != nil && in.Protected(path) {
			reasons = append(reasons, "protected:"+path)
		}
	}
	slices.Sort(reasons)
	reasons = slices.Compact(reasons)
	return Decision{Hold: len(reasons) > 0, Reasons: reasons}
}

type Record struct {
	Cycle    int       `json:"cycle"`
	Status   string    `json:"status"`
	Commit   string    `json:"commit"`
	Tree     string    `json:"tree"`
	Reasons  []string  `json:"reasons"`
	Requires []string  `json:"requires"`
	At       time.Time `json:"at"`
}

type Approval struct {
	Cycle    int       `json:"cycle"`
	Tree     string    `json:"tree"`
	Decision string    `json:"decision"`
	Audit    string    `json:"audit_report,omitempty"`
	At       time.Time `json:"at"`
}

type Verdict int

const (
	Pending Verdict = iota
	Approved
	Rejected
)

func Judge(a Approval, found bool, tree string) Verdict {
	if !found || a.Tree != tree {
		return Pending
	}
	switch a.Decision {
	case DecisionApprove:
		return Approved
	case DecisionReject:
		return Rejected
	}
	return Pending
}

func StatusAfter(decision string) string {
	if decision == DecisionApprove {
		return StatusApproved
	}
	return StatusRejected
}

func HeldDir(evolveDir string) string { return filepath.Join(evolveDir, "landing", "held") }

func ApprovalsDir(evolveDir string) string {
	return filepath.Join(evolveDir, "landing", "approvals")
}

func fileName(cycle int) string { return recordPrefix + strconv.Itoa(cycle) + ".json" }

func WriteRecord(evolveDir string, r Record) error {
	if r.Cycle < 1 || r.Tree == "" || r.Status == "" {
		return ErrBadRecord
	}
	return write(filepath.Join(HeldDir(evolveDir), fileName(r.Cycle)), r)
}

func WriteApproval(evolveDir string, a Approval) error {
	if a.Cycle < 1 || a.Tree == "" || (a.Decision != DecisionApprove && a.Decision != DecisionReject) {
		return ErrBadApproval
	}
	return write(filepath.Join(ApprovalsDir(evolveDir), fileName(a.Cycle)), a)
}

func ReadApproval(evolveDir string, cycle int) (Approval, bool, error) {
	var a Approval
	found, err := read(filepath.Join(ApprovalsDir(evolveDir), fileName(cycle)), &a)
	return a, found, err
}

func ReadRecord(evolveDir string, cycle int) (Record, bool, error) {
	var r Record
	found, err := read(filepath.Join(HeldDir(evolveDir), fileName(cycle)), &r)
	return r, found, err
}

func ReadRecords(evolveDir string) ([]Record, error) {
	entries, err := os.ReadDir(HeldDir(evolveDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("heldproposal: list the hold records: %w", err)
	}
	var out []Record
	for _, e := range entries {
		cycle, ok := cycleOf(e.Name())
		if !ok {
			continue
		}
		r, found, err := ReadRecord(evolveDir, cycle)
		if err != nil {
			return nil, err
		}
		if found {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b Record) int { return a.Cycle - b.Cycle })
	return out, nil
}

func cycleOf(name string) (int, bool) {
	rest, hasPrefix := strings.CutPrefix(name, recordPrefix)
	digits, hasSuffix := strings.CutSuffix(rest, ".json")
	if !hasPrefix || !hasSuffix {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil && n > 0
}

func write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("heldproposal: create %s: %w", filepath.Dir(path), err)
	}
	if err := atomicwrite.JSON(path, v); err != nil {
		return fmt.Errorf("heldproposal: write %s: %w", path, err)
	}
	return nil
}

func read(path string, v any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("heldproposal: read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, fmt.Errorf("heldproposal: parse %s: %w", path, err)
	}
	return true, nil
}
