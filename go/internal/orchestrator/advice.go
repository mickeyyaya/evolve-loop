package orchestrator

const (
	AdviceBuild  = "console: build"
	AdviceFeed   = "console: feed"
	AdviceFinish = "console: finish"

	ConsoleBatchSize = 10
)

func ConsoleAdvice(admissible, window, openPRs int) string {
	switch {
	case openPRs > ConsoleBatchSize:
		return AdviceFinish
	case admissible < window:
		return AdviceFeed
	}
	return AdviceBuild
}
