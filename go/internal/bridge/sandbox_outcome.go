package bridge

import "fmt"

type RefusalCause int

const (
	CauseUnspecified RefusalCause = iota
	CauseProbe
	CauseProfile
)

type WrapOutcome struct {
	Applied bool
	Cause   RefusalCause
	Reason  string
}

func (deps Deps) refuseWrap(cause RefusalCause, reason string) ([]string, WrapOutcome) {
	if deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] %s\n", reason)
	}
	return nil, WrapOutcome{Cause: cause, Reason: reason}
}

func (out WrapOutcome) describe(confinement string) string {
	switch {
	case out.Reason == "":
		return confinement
	case out.Cause == CauseProbe:
		return confinement + "; probe: " + out.Reason
	default:
		return out.Reason
	}
}
