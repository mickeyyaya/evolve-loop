package guards

import "github.com/mickeyyaya/evolveloop/go/internal/core"

// core_GuardInput is a local alias so helpers.go can take a thin
// dependency on the core type without each file re-importing core
// in lockstep. It's not exported.
type core_GuardInput = core.GuardInput
