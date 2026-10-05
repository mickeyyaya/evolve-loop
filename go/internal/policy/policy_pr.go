package policy

import "fmt"

type PRPolicy struct {
	MergeMethod string `json:"merge_method,omitempty"`
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
