package core

// planStage selects which of the advisor's two whole-cycle planning calls is
// running (ADR-0052 WS1-S3). stageInitial is the cycle-start Plan (zero value =
// today's behavior); stagePostScout is the re-invokable RePlan. It is kept
// UNEXPORTED in core — ADR-0052 table 4.2 sketched it as a router.PlanStage, but
// the advisor's dispatch framing is core's concern and no other package consumes
// it, so exporting it would add cross-package coupling + an apicover surface with
// no caller. The router still owns the floor + canonical order it clamps against.
type planStage int

const (
	stageInitial planStage = iota
	stagePostScout
)

// artifactFile is the raw plan artifact name for the stage.
func (s planStage) artifactFile() string {
	if s == stagePostScout {
		return "routing-replan.json"
	}
	return "routing-plan.json"
}

// captureKind is the <kind> token embedded in the WS3 capture filenames
// (advisor-{prompt,response,span}-<kind>.*); isSafeArtifactKind confines it.
func (s planStage) captureKind() string {
	if s == stagePostScout {
		return "replan"
	}
	return "plan"
}

// replanDepth is the depth stamped on the decision span: a re-plan is one level
// deeper than the initial plan. WS2-S5 caps the live re-plan at this depth=1.
func (s planStage) replanDepth() int {
	if s == stagePostScout {
		return 1
	}
	return 0
}
