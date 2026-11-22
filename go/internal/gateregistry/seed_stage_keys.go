package gateregistry

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

var outsideSchemaStageSeeds = []stageSeed{
	{
		key:     "gates.scan_cache_gate",
		signal:  ship.CodeScanShadowEscape,
		site:    gatecontract.EmitSite{Path: "internal/phases/ship/scanshadow.go", Func: "escape", Ref: "CodeScanShadowEscape"},
		fields:  []string{"package", "key", "tags", "flags", "gate"},
		closure: []string{"internal/testkey", "internal/testverdict"},
	},
	{key: "handoff.build-audit.stage"},
	{key: "handoff.intent-scout.stage"},
	{key: "handoff.scout-triage.stage"},
	{key: "handoff.tdd-build.stage"},
	{key: "handoff.triage-tdd.stage"},
}
