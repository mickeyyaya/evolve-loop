package triagecap

// The projection's wire shape, as the tests decode it; the writer is triagedecision.Project.
type projTopN struct {
	ID     string   `json:"id"`
	Action string   `json:"action,omitempty"`
	Files  []string `json:"files,omitempty"`
}

type projID struct {
	ID string `json:"id"`
}

type projDropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

type projectedDecision struct {
	Cycle      int           `json:"cycle"`
	TopN       []projTopN    `json:"top_n"`
	Deferred   []projID      `json:"deferred"`
	Dropped    []projDropped `json:"dropped"`
	Superseded []string      `json:"superseded"`
	Projected  bool          `json:"projected_by_orchestrator"`
}
