package bridge

type LaunchIntent struct {
	ModelTier        string
	Permission       string
	SettingsScope    string
	SessionMode      string
	Effort           string
	LadderEffort     string
	AllowedTools     []string
	SystemPromptFile string
	RawByCLI         map[string][]string
}

type Realization struct {
	LaunchFlags         []string
	REPLInput           []string
	Env                 map[string]string
	Ephemeral           bool
	SessionName         string
	ModelOmitted        string
	SystemPromptFile    string
	EffortCapped        string
	EffortVariant       string
	EffortUnapplied     string
	modelDispatchEffect modelDispatch
}
