package reviewcheck

const (
	CheckInboxIntegrity     = "inbox-integrity"
	CheckPersonaStrip       = "persona-strip"
	CheckPermissionWidening = "permission-widening"
)

type Refusal struct {
	Check   string `json:"check"`
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
}

type Range struct {
	Base string
	Head string
}
