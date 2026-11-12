// Package gatecontract holds the types of a gate entry and the words of its contract; see docs/architecture/gate-contract.md.
package gatecontract

import (
	"context"
	"slices"
)

const (
	KindScanner    = "scanner"
	KindPhaseFloor = "phase-floor"
	KindShipCheck  = "ship-check"
	StageShadow    = "shadow"
	StageEnforce   = "enforce"
	ClassAuthority = "authority"
	ClassVerified  = "verified"
	ClassSubject   = "subject"
	ClassAdvisory  = "advisory"
	ClassOpenSeam  = "open-seam"
)

var (
	trustClasses = []string{ClassAuthority, ClassVerified, ClassSubject, ClassAdvisory, ClassOpenSeam}
	kinds        = []string{KindScanner, KindPhaseFloor, KindShipCheck}
	stages       = []string{StageShadow, StageEnforce}
)

type Input struct {
	Root string
	Base string
}

type Verdict struct {
	Reds  []string
	Notes []string
}

type Owner struct {
	Pattern string
	Phase   string
}

type Declared struct {
	Path  string
	Class string
}

type Entry struct {
	Name   string
	Kind   string
	Files  []string
	Covers []string
	Owners []Owner
	Inputs []Declared
	Stage  string
	Run    func(context.Context, Input) (Verdict, error)
}

func TrustClasses() []string { return slices.Clone(trustClasses) }

func Kinds() []string { return slices.Clone(kinds) }

func Stages() []string { return slices.Clone(stages) }
