package policy

type ChroniclePolicy struct {
	Digest       string `json:"digest,omitempty"`
	DigestTokens int    `json:"digest_tokens,omitempty"`
	DigestCycles int    `json:"digest_cycles,omitempty"`
	Escalation   string `json:"escalation,omitempty"`
	Historian    string `json:"historian,omitempty"`
}

type ChronicleConfig struct {
	Digest       string
	DigestTokens int
	DigestCycles int
	Escalation   string
	Historian    string
}

func (p Policy) ChronicleConfig() ChronicleConfig {
	c := ChronicleConfig{
		Digest:       "shadow",
		DigestTokens: 1200,
		DigestCycles: 10,
		Escalation:   "shadow",
		Historian:    "off",
	}
	if p.Chronicle == nil {
		return c
	}
	if p.Chronicle.Digest != "" {
		c.Digest = p.Chronicle.Digest
	}
	if p.Chronicle.DigestTokens > 0 {
		c.DigestTokens = p.Chronicle.DigestTokens
	}
	if p.Chronicle.DigestCycles > 0 {
		c.DigestCycles = p.Chronicle.DigestCycles
	}
	if p.Chronicle.Escalation != "" {
		c.Escalation = p.Chronicle.Escalation
	}
	if p.Chronicle.Historian != "" {
		c.Historian = p.Chronicle.Historian
	}
	return c
}
