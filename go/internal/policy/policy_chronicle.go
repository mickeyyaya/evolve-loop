package policy

// Chronicle S2 (chronicle-s2-digest-writer): the .evolve/policy.json
// "chronicle" block, mirroring the GatesPolicy default-then-override idiom.
// Lives in its own file because policy.go is already 1500+ lines.
//
// Stages are policy-driven (shadow/enforce/off), NOT feature flags: compiled
// defaults apply when the block is absent, and a present block overrides only
// the fields it sets. Bump step / cap / autofile weight / historian cap stay
// compiled constants in their owning packages, by design.

// ChroniclePolicy is the .evolve/policy.json "chronicle" block.
type ChroniclePolicy struct {
	// Digest is the recent-outcomes digest stage: "shadow" (default — write
	// the artifact, don't inject it), "enforce", or "off".
	Digest string `json:"digest,omitempty"`
	// DigestTokens caps the rendered digest (len/4 estimator). Default 1200.
	DigestTokens int `json:"digest_tokens,omitempty"`
	// DigestCycles caps the dossier window. Default 10.
	DigestCycles int `json:"digest_cycles,omitempty"`
	// Escalation is the recurrence-escalation stage. Default "shadow".
	Escalation string `json:"escalation,omitempty"`
	// Historian is the historian phase stage. Default "off".
	Historian string `json:"historian,omitempty"`
}

// ChronicleConfig is the resolved chronicle configuration with the compiled
// defaults applied.
type ChronicleConfig struct {
	Digest       string
	DigestTokens int
	DigestCycles int
	Escalation   string
	Historian    string
}

// ChronicleConfig resolves the chronicle block against the compiled defaults:
// digest=shadow, digest_tokens=1200, digest_cycles=10, escalation=shadow,
// historian=off. An absent (or empty) block resolves to exactly the defaults.
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
