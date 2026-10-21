package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	auditFailReasonFile = "audit-fail-reason.json"
	failureDigestFile   = "failure-digest.json"
)

const (
	PreClassInfraError  = "infra-error"
	PreClassGuardAbort  = "guard-abort"
	PreClassGateBlock   = "gate-block"
	PreClassVerdictFail = "verdict-fail"
	PreClassUnknown     = "unknown"
)

const (
	maxFailureReasons     = 5
	maxFailureReasonBytes = 200
)

type FailureRecord struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	PreClass    string   `json:"pre_class,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

func failureRecord(workspace string, cycle int) (*FailureRecord, bool) {
	rec := &FailureRecord{Reasons: failureReasons(workspace)}
	if raw, err := os.ReadFile(filepath.Join(workspace, failureDigestFile)); err == nil {
		var d struct {
			Cycle       int    `json:"cycle"`
			Fingerprint string `json:"fingerprint"`
			PreClass    string `json:"pre_class"`
		}
		if json.Unmarshal(raw, &d) == nil && d.Cycle == cycle {
			rec.Fingerprint = strings.TrimSpace(d.Fingerprint)
			rec.PreClass = strings.TrimSpace(d.PreClass)
		}
	}
	if rec.Fingerprint == "" && rec.PreClass == "" && len(rec.Reasons) == 0 {
		return nil, false
	}
	return rec, true
}

func failureReasons(workspace string) []string {
	raw, err := os.ReadFile(filepath.Join(workspace, auditFailReasonFile))
	if err != nil {
		return nil
	}
	var a struct {
		Reasons []string `json:"reasons"`
	}
	if json.Unmarshal(raw, &a) != nil {
		return nil
	}
	var out []string
	for _, r := range a.Reasons {
		r = strings.Join(strings.Fields(r), " ")
		if r == "" {
			continue
		}
		if len(r) > maxFailureReasonBytes {
			r = strings.ToValidUTF8(r[:maxFailureReasonBytes], "")
		}
		out = append(out, r)
		if len(out) == maxFailureReasons {
			break
		}
	}
	return out
}
