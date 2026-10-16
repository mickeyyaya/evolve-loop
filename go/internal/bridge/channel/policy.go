package channel

type Policy interface {
	OnEvent(env map[string]any) *AskAction
}

type AskAction struct{ Question string }

type StallPolicy struct{ Question string }

func (p StallPolicy) OnEvent(env map[string]any) *AskAction {
	if env["kind"] == "stall" {
		return &AskAction{Question: p.Question}
	}
	return nil
}
