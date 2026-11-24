package guards

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Denial struct {
	Guard  string
	Path   string
	Reason string
	CWD    string
}

var FamilyNeutral = map[string]bool{"ship": true}

func EmitDenial(c *signalcenter.Center, d Denial) {
	c.Emit(signalcenter.Event{
		Module: signalcenter.ModuleIsolation, Origin: "guards.EmitDenial",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: core.CodeGuardDenied,
		Reason: "guard " + d.Guard + " denied " + d.Path + ": " + d.Reason,
		Fields: map[string]string{
			core.DenialFieldGuard:   d.Guard,
			core.DenialFieldPath:    d.Path,
			core.DenialFieldCWD:     d.CWD,
			core.DenialFieldNeutral: strconv.FormatBool(FamilyNeutral[d.Guard]),
			core.DenialFieldReason:  d.Reason,
		},
	})
}
