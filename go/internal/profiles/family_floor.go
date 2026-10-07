package profiles

import "maps"

var claudeFamilyFloor = map[string]string{
	"auditor":            "adversarial grading of build content — cross-family anti-gaming core",
	"adversarial-review": "adversarial grading of build content — cross-family anti-gaming core",
	"tdd-engineer":       "test author — anti-cooperative-bias family split from the builder",
	"spec-verifier":      "audit-side verification of build output",
	"spec-verify":        "audit-side verification of build output",
	"code-reviewer":      "independent review of build content — cross-family with the builder (ADR-0124)",
}

func ClaudeFamilyFloor() map[string]string {
	return maps.Clone(claudeFamilyFloor)
}

func IsClaudeFamilyFloor(name string) bool {
	_, floored := claudeFamilyFloor[name]
	return floored
}
