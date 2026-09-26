package policy

// ChroniclePolicy is the "chronicle" block of recurrence-chronicle stages and digest budgets.
type ChroniclePolicy struct {
	// Digest is "shadow" (default: write the digest, do not inject it), "enforce" or "off".
	Digest string `json:"digest,omitempty"`
	// DigestTokens caps the rendered digest, estimated as len/4; default 1200.
	DigestTokens int `json:"digest_tokens,omitempty"`
	// DigestCycles caps the dossier window; default 10.
	DigestCycles int `json:"digest_cycles,omitempty"`
	// Escalation is the recurrence-escalation stage; default "shadow".
	Escalation string `json:"escalation,omitempty"`
	// Historian is the historian phase stage; default "off".
	Historian string `json:"historian,omitempty"`
}

// ChronicleConfig is the resolved chronicle configuration with the compiled defaults applied.
type ChronicleConfig struct {
	Digest       string
	DigestTokens int
	DigestCycles int
	Escalation   string
	Historian    string
}

// ChronicleConfig resolves the chronicle block; an absent or empty block yields exactly the defaults.
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
	if p.Chronicle.DigestTokens != 0 {
		c.DigestTokens = p.Chronicle.DigestTokens
	}
	if p.Chronicle.DigestCycles != 0 {
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
