package channel

// Policy decides whether to ask, from one feed envelope (decoded as a map). A
// smarter (LLM-driven) policy implements the same interface later (ADR-0037
// non-goal for v1).
type Policy interface {
	OnEvent(env map[string]any) *AskAction
}

// AskAction is a policy's decision to inject a question.
type AskAction struct{ Question string }

// StallPolicy is the minimal default: ask for a progress summary when the
// producer emits a stall envelope.
type StallPolicy struct{ Question string }

// OnEvent returns an AskAction when env is a stall, else nil.
func (p StallPolicy) OnEvent(env map[string]any) *AskAction {
	if env["kind"] == "stall" {
		return &AskAction{Question: p.Question}
	}
	return nil
}
