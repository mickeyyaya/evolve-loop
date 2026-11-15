package inboxbatch

import "slices"

const (
	ProbeTestExists = "test-exists"
	ProbePathExists = "path-exists"
	ProbePathAbsent = "path-absent"
)

type Probe struct {
	Kind string `json:"kind"`
	Arg  string `json:"arg"`
}

var probeKinds = []string{ProbeTestExists, ProbePathExists, ProbePathAbsent}

func ProbeKinds() []string { return slices.Clone(probeKinds) }
