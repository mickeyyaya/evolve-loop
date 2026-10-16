package recovery

type StallAction string

const (
	StallExtend    StallAction = "extend"
	StallKillRetry StallAction = "kill_retry"
	StallEscalate  StallAction = "escalate"
)

type StallEvent struct {
	Kind        string
	Phase       string
	IdleS       int
	ThresholdS  int
	ToolCalls   int
	ToolResults int
}

type StallPolicy interface {
	Decide(ev StallEvent) (StallAction, string)
}
