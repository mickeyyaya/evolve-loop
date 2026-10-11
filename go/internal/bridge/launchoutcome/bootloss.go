package launchoutcome

type BootLossCause string

const (
	BootLossPaneLost  BootLossCause = BootLossCause(TimeoutPaneLost)
	BootLossDeadShell BootLossCause = "dead_shell"
)

func (c BootLossCause) Lost() bool {
	return c == BootLossPaneLost || c == BootLossDeadShell
}

func WithBootLoss(cause string, code int, loss BootLossCause) string {
	if code == ExitREPLBootTimeout && loss.Lost() {
		return string(loss)
	}
	return cause
}
