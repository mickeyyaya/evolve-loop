// Package retryladder classifies a failed phase try from host facts alone (ADR-0142, component L1).
package retryladder

import (
	"path"
	"slices"
	"strings"
)

type Class string

const (
	Infra      Class = "infra"
	Quota      Class = "quota"
	Halt       Class = "halt"
	Spec       Class = "spec"
	Capability Class = "capability"
	Unknown    Class = "unknown"
)

const (
	exitSafetyGate       = 2
	exitCostLeak         = 3
	exitBadFlags         = 10
	exitREPLBootTimeout  = 80
	exitArtifactTimeout  = 81
	exitUnknownPrompt    = 85
	exitRespondLoopGuard = 86
	exitModelMismatch    = 87
	exitRequireFullUnmet = 99
	exitCmdTimeout       = 124
	exitMissingBinary    = 127
)

type FloorFailure struct {
	Path      string
	Protected bool
}

type Facts struct {
	ExitCode           int
	QuotaBench         bool
	TeardownError      bool
	AuditClass         string
	ProvenBlocker      bool
	LaneScope          []string
	Floors             []FloorFailure
	DeliverableMissing bool
	Reason             string
}

type Verdict struct {
	Class       Class
	Fingerprint string
}

func Classify(f Facts) Verdict {
	return Verdict{Class: classOf(f), Fingerprint: Fingerprint(f.Reason)}
}

func classOf(f Facts) Class {
	switch {
	case isHaltExit(f.ExitCode):
		return Halt
	case f.ExitCode == exitUnknownPrompt && f.QuotaBench:
		return Quota
	case isInfraExit(f.ExitCode) || f.TeardownError:
		return Infra
	case isSpecFailure(f):
		return Spec
	case isCapabilityFailure(f):
		return Capability
	}
	return Unknown
}

func isHaltExit(code int) bool {
	switch code {
	case exitSafetyGate, exitCostLeak, exitBadFlags, exitRespondLoopGuard, exitRequireFullUnmet:
		return true
	}
	return false
}

func isInfraExit(code int) bool {
	switch code {
	case exitREPLBootTimeout, exitArtifactTimeout, exitModelMismatch, exitUnknownPrompt, exitCmdTimeout, exitMissingBinary:
		return true
	}
	return false
}

func isSpecFailure(f Facts) bool {
	if f.ProvenBlocker || f.AuditClass == "intent-malformed" || f.AuditClass == "intent-rejected" {
		return true
	}
	return slices.ContainsFunc(f.Floors, func(fl FloorFailure) bool {
		return fl.Protected || !inScope(fl.Path, f.LaneScope)
	})
}

func isCapabilityFailure(f Facts) bool {
	return len(f.Floors) > 0 || f.DeliverableMissing || f.AuditClass == "code-build-fail" || f.AuditClass == "code-audit-fail"
}

func inScope(file string, scope []string) bool {
	file = path.Clean(file)
	return slices.ContainsFunc(scope, func(entry string) bool {
		entry = path.Clean(entry)
		return file == entry || strings.HasPrefix(file, entry+"/")
	})
}
