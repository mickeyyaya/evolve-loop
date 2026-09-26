package core

import "github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"

// AgentIdentity is the dispatch identity (CLI, model, profile, persona, label) the control-plane advisors share.
type AgentIdentity = advisor.Identity
