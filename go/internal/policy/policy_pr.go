package policy

import (
	"fmt"
	"time"
)

type PRPolicy struct {
	MergeMethod string             `json:"merge_method,omitempty"`
	MergeWindow *MergeWindowPolicy `json:"merge_window,omitempty"`
}

const (
	MergeWindowOff = "off"
	MergeWindowOn  = "on"
)

const DefaultEarlyBoundaryAfterS = 3600

type MergeWindowPolicy struct {
	Stage               string `json:"stage,omitempty"`
	EarlyBoundaryAfterS *int   `json:"early_boundary_after_s,omitempty"`
}

func (p Policy) PREarlyBoundaryAfter() (time.Duration, error) {
	if p.PR == nil || p.PR.MergeWindow == nil || p.PR.MergeWindow.EarlyBoundaryAfterS == nil {
		return DefaultEarlyBoundaryAfterS * time.Second, nil
	}
	if v := *p.PR.MergeWindow.EarlyBoundaryAfterS; v > 0 {
		return time.Duration(v) * time.Second, nil
	}
	return 0, fmt.Errorf("policy: pr.merge_window.early_boundary_after_s must be > 0, got %d", *p.PR.MergeWindow.EarlyBoundaryAfterS)
}

func (p Policy) PRMergeMethod() (string, error) {
	if p.PR == nil || p.PR.MergeMethod == "" {
		return "merge", nil
	}
	switch v := p.PR.MergeMethod; v {
	case "merge", "squash", "rebase":
		return v, nil
	default:
		return "", fmt.Errorf("policy: pr.merge_method must be merge, squash or rebase, got %q", v)
	}
}

func (p Policy) PRMergeWindowStage() (string, error) {
	if p.PR == nil || p.PR.MergeWindow == nil || p.PR.MergeWindow.Stage == "" {
		return MergeWindowOff, nil
	}
	switch v := p.PR.MergeWindow.Stage; v {
	case MergeWindowOff, MergeWindowOn:
		return v, nil
	default:
		return "", fmt.Errorf("policy: pr.merge_window.stage must be off or on, got %q", v)
	}
}
