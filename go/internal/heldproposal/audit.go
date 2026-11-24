package heldproposal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	AuditPass = "PASS"
	AuditFail = "FAIL"
	AuditWait = "WAIT"

	WaitFloorExcludesOtherFamilies = "floor_excludes_other_families"
	WaitNoIndependentRoute         = "no_independent_route"
	WaitBuilderUnattributed        = "builder_unattributed"
	WaitSeatFailed                 = "seat_failed"
	WaitPortNotWired               = "port_not_wired"
	WaitShipWindowHeld             = "ship_window_held"
	WaitDiffMissing                = "diff_missing"
	WaitDiffMismatch               = "diff_mismatch"
)

var (
	ErrAuditMissing       = errors.New("heldproposal: the held cycle has no adversarial audit report")
	ErrAuditNotPass       = errors.New("heldproposal: the adversarial audit of the held cycle did not pass")
	ErrAuditBuilderFamily = errors.New("heldproposal: the audit ran on a model family that built the change")
	ErrAuditTreeDiffers   = errors.New("heldproposal: the audit report names another tree than the hold")
	ErrAuditDiffDiffers   = errors.New("heldproposal: the audit report names another diff than the hold")
	ErrAuditNotDeep       = errors.New("heldproposal: the audit did not run at the deep tier")
	ErrAuditUnattributed  = errors.New("heldproposal: the hold names no builder model family, so no audit can be independent")
	ErrAuditBodyDiffers   = errors.New("heldproposal: the audit report body differs from its recorded digest")
)

type AuditReport struct {
	Cycle           int       `json:"cycle"`
	Tree            string    `json:"tree"`
	DiffSHA256      string    `json:"diff_sha256"`
	BuilderFamilies []string  `json:"builder_families"`
	Family          string    `json:"family"`
	CLI             string    `json:"cli"`
	Tier            string    `json:"tier"`
	Verdict         string    `json:"verdict"`
	Reason          string    `json:"reason,omitempty"`
	RunLabel        string    `json:"run_label,omitempty"`
	LaunchRecord    string    `json:"launch_record,omitempty"`
	BodySHA256      string    `json:"body_sha256,omitempty"`
	At              time.Time `json:"at"`
}

func DigestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func AuditReportPath(evolveDir string, cycle int) string {
	return filepath.Join(HeldDir(evolveDir), "cycle-"+strconv.Itoa(cycle)+".audit.json")
}

func AuditBodyPath(evolveDir string, cycle int) string {
	return filepath.Join(HeldDir(evolveDir), "cycle-"+strconv.Itoa(cycle)+".audit.md")
}

func AuditDiffPath(evolveDir string, cycle int) string {
	return filepath.Join(HeldDir(evolveDir), "cycle-"+strconv.Itoa(cycle)+".audit.diff")
}

func WriteAuditReport(evolveDir string, r AuditReport, body []byte) error {
	if r.Cycle < 1 || r.Tree == "" || r.Verdict == "" {
		return ErrBadRecord
	}
	if len(body) > 0 {
		r.BodySHA256 = DigestOf(body)
		if err := writeBytes(AuditBodyPath(evolveDir, r.Cycle), body); err != nil {
			return err
		}
	}
	return write(AuditReportPath(evolveDir, r.Cycle), r)
}

func ReadAuditReport(evolveDir string, cycle int) (AuditReport, []byte, bool, error) {
	var r AuditReport
	found, err := read(AuditReportPath(evolveDir, cycle), &r)
	if err != nil || !found {
		return r, nil, found, err
	}
	body, bodyFound, err := readBytes(AuditBodyPath(evolveDir, cycle))
	if err != nil {
		return r, nil, true, err
	}
	if !bodyFound {
		body = nil
	}
	return r, body, true, nil
}

func Independent(family, tier string, builders []string) error {
	if family == "" || slices.Contains(builders, family) {
		return fmt.Errorf("%w: audit family %q, builder families %v", ErrAuditBuilderFamily, family, builders)
	}
	if policy.TierRank(tier) < policy.TierRank(policy.DeepTier) {
		return fmt.Errorf("%w: tier %q", ErrAuditNotDeep, tier)
	}
	return nil
}

func WriteAuditDiff(evolveDir string, cycle int, diff []byte) (string, error) {
	if err := writeBytes(AuditDiffPath(evolveDir, cycle), diff); err != nil {
		return "", err
	}
	return DigestOf(diff), nil
}

func VerifyAudit(rec Record, report AuditReport, found bool, body []byte) error {
	if !found {
		return ErrAuditMissing
	}
	if len(rec.BuilderFamilies) == 0 || slices.Contains(rec.BuilderFamilies, "") {
		return ErrAuditUnattributed
	}
	if report.Verdict != AuditPass {
		return fmt.Errorf("%w: verdict %q %s", ErrAuditNotPass, report.Verdict, report.Reason)
	}
	if report.Tree != rec.Tree {
		return fmt.Errorf("%w: held %s, report %s", ErrAuditTreeDiffers, rec.Tree, report.Tree)
	}
	if report.DiffSHA256 != rec.DiffSHA256 {
		return ErrAuditDiffDiffers
	}
	if err := Independent(report.Family, report.Tier, rec.BuilderFamilies); err != nil {
		return err
	}
	if report.BodySHA256 == "" || report.BodySHA256 != DigestOf(body) {
		return ErrAuditBodyDiffers
	}
	return nil
}
