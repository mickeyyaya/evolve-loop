package policy

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

const (
	stageWordOff    = "off"
	stageWordShadow = "shadow"
)

var gateStageKeys = []string{"gates.scan_cache_gate"}

type StageField struct {
	Path     string
	TypeName string
}

func StageFields() []StageField {
	var out []StageField
	walkStageFields(reflect.TypeOf(Policy{}), "", map[reflect.Type]bool{}, &out)
	return out
}

func walkStageFields(t reflect.Type, prefix string, active map[reflect.Type]bool, out *[]StageField) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || active[t] {
		return
	}
	active[t] = true
	defer delete(active, t)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := prefix + name
		if strings.HasSuffix(name, "stage") {
			*out = append(*out, StageField{Path: path, TypeName: field.Type.String()})
		}
		walkStageFields(field.Type, path+".", active, out)
	}
}

func StageKeys() []string {
	fields := StageFields()
	keys := make([]string, 0, len(fields)+len(gateStageKeys)+len(knownHandoffPairs))
	for _, field := range fields {
		keys = append(keys, field.Path)
	}
	keys = append(keys, gateStageKeys...)
	for _, pair := range knownHandoffPairs {
		keys = append(keys, "handoff."+pair+".stage")
	}
	return keys
}

var errNoStageReader = errors.New("no reader")

type stageKeyError struct {
	Key string
	Err error
}

func (e *stageKeyError) Error() string { return fmt.Sprintf("policy: stage key %s: %v", e.Key, e.Err) }

func (e *stageKeyError) Unwrap() error { return e.Err }

type stageReader func(Policy) (string, error)

func (p Policy) EffectiveStages() (map[string]string, error) {
	return resolveStages(StageKeys(), stageReaders, p)
}

func resolveStages(keys []string, readers map[string]stageReader, p Policy) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		read, ok := readers[key]
		if !ok {
			return nil, &stageKeyError{Key: key, Err: errNoStageReader}
		}
		stage, err := read(p)
		if err != nil {
			return nil, &stageKeyError{Key: key, Err: err}
		}
		out[key] = stage
	}
	return out, nil
}

func plainStage(read func(Policy) string) stageReader {
	return func(p Policy) (string, error) { return read(p), nil }
}

func orStage(fallback string, read func(Policy) string) stageReader {
	return func(p Policy) (string, error) {
		if stage := read(p); stage != "" {
			return stage, nil
		}
		return fallback, nil
	}
}

func isolationStage(read func(Policy) string) stageReader {
	return plainStage(func(p Policy) string { return ResolveIsolationStage(read(p)) })
}

func flowStage(read func(Policy) string) stageReader {
	return plainStage(func(p Policy) string {
		stage, _ := ResolveFlowStage(read(p))
		return stage
	})
}

func handoffStage(pair string) stageReader {
	return plainStage(func(p Policy) string { return p.HandoffConfig()[pair].Stage })
}

func gcRemoteStage(p Policy) string {
	if p.GC == nil {
		return gcpolicy.Policy{}.RemoteStage()
	}
	return p.GC.RemoteStage()
}

func fleetBudgetStage(p Policy) string {
	if budget := p.FleetConfig().Budget; budget != nil {
		return budget.Stage
	}
	return stageWordOff
}

func fleetAdaptiveStage(p Policy) string {
	if adaptive := p.FleetConfig().Orchestrator.Adaptive; adaptive.Enabled {
		return adaptive.Stage
	}
	return stageWordOff
}

func integrityStage(p Policy) string {
	_, stage, _ := p.IntegrityMode()
	return stage
}

var stageReaders = map[string]stageReader{
	"inbox.curator.stage":                        plainStage(func(p Policy) string { return p.InboxCuratorConfig().Stage }),
	"inbox.curator.trailer.stage":                plainStage(Policy.InboxTrailerStage),
	"gc.remote_sweep.stage":                      plainStage(gcRemoteStage),
	"pr.merge_window.stage":                      Policy.PRMergeWindowStage,
	"workflow.findings_repair.code-review.stage": plainStage(func(p Policy) string { return p.WorkflowConfig().CodeReviewRepair.Stage }),
	"workflow.convergence.stage":                 plainStage(func(p Policy) string { return p.ConvergenceConfig().Stage }),
	"workflow.retry_ladder.stage":                plainStage(func(p Policy) string { return p.WorkflowConfig().RetryLadder.Stage }),
	"workflow.retry_ladder.audit_fix.stage":      plainStage(func(p Policy) string { return p.WorkflowConfig().RetryLadder.AuditFix.Stage }),
	"workflow.repair.fixability.stage":           plainStage(func(p Policy) string { return p.WorkflowConfig().Repair.Fixability.Stage }),
	"workflow.cycle_spend.stage":                 plainStage(func(p Policy) string { return p.WorkflowConfig().CycleSpend.Stage.String() }),
	"workflow.builder_selfcheck.stage":           plainStage(func(p Policy) string { return p.WorkflowConfig().BuilderSelfCheck.Stage.String() }),
	"workflow.supersede.stage":                   plainStage(func(p Policy) string { return p.WorkflowConfig().Supersede.Stage.String() }),
	"swarm.stage":                                plainStage(func(p Policy) string { return p.SwarmConfig().Stage }),
	"failure_disposition.stage":                  plainStage(func(p Policy) string { return p.FailureDispositionConfig().Stage }),
	"merge_gate.stage":                           plainStage(func(p Policy) string { return p.MergeGateConfig().Stage }),
	"parallel_evaluate.stage":                    plainStage(func(p Policy) string { return p.ParallelEvaluateConfig().Stage }),
	"catalog.refresh_stage":                      plainStage(func(p Policy) string { return p.CatalogConfig().RefreshStage }),
	"docs_floor.stage":                           plainStage(func(p Policy) string { return p.DocsFloorConfig().Stage }),
	"docs_floor.ste_stage":                       plainStage(func(p Policy) string { return p.DocsFloorConfig().SteStage }),
	"comment_floor.stage":                        plainStage(func(p Policy) string { return p.CommentFloorConfig().Stage }),
	"held_proposals.stage":                       plainStage(func(p Policy) string { return p.HeldProposalsStage().String() }),
	"cover_changed.stage":                        plainStage(func(p Policy) string { return p.CoverChangedConfig().Stage }),
	"retrospectives.stage":                       plainStage(func(p Policy) string { return p.RetrospectivesConfig().Stage }),
	"integrity.stage":                            plainStage(integrityStage),
	"fleet.budget.stage":                         plainStage(fleetBudgetStage),
	"fleet.orchestrator.stage":                   plainStage(func(p Policy) string { return p.FleetConfig().Orchestrator.Stage }),
	"fleet.integration.stage":                    Policy.FleetIntegrationStage,
	"fleet.orchestrator.window.adaptive.stage":   plainStage(fleetAdaptiveStage),
	"audit_cascade.stage":                        plainStage(func(p Policy) string { return p.AuditCascadeConfig().Stage }),
	"isolation.i2.stage":                         isolationStage(Policy.IsolationEnvStage),
	"isolation.i3.stage":                         isolationStage(Policy.IsolationExecStage),
	"isolation.i5.stage":                         isolationStage(Policy.IsolationReadStage),
	"isolation.i7.stage":                         isolationStage(Policy.IsolationGuaranteeStage),
	"isolation.i8.stage":                         flowStage(Policy.IsolationFlowStage),
	"isolation.i15.stage":                        plainStage(func(p Policy) string { return ResolveQueueStage(p.IsolationQueueStage()) }),
	"isolation.context.stage":                    orStage(stageWordShadow, func(p Policy) string { return p.IsolationContext().Stage }),
	"isolation.dispatch_requests.stage":          plainStage(func(p Policy) string { return string(p.IsolationDispatchRequests().Stage) }),
	"isolation.inbox_requests.stage":             plainStage(func(p Policy) string { return string(p.IsolationInboxRequests().Stage) }),
	"healing.detector.stage":                     plainStage(func(p Policy) string { return p.HealingDetector().Stage }),
	"healing.orchestrator.stage":                 plainStage(func(p Policy) string { return p.HealingOrchestrator().Stage }),
	"healing.validator.stage":                    plainStage(func(p Policy) string { return p.HealingValidator().Stage }),
	"healing.live_reaction.stage":                plainStage(func(p Policy) string { return p.HealingLiveReaction().Stage }),
	"gates.scan_cache_gate":                      plainStage(func(p Policy) string { return p.GatesConfig().ScanCacheGate }),
	"handoff.build-audit.stage":                  handoffStage("build-audit"),
	"handoff.intent-scout.stage":                 handoffStage("intent-scout"),
	"handoff.scout-triage.stage":                 handoffStage("scout-triage"),
	"handoff.tdd-build.stage":                    handoffStage("tdd-build"),
	"handoff.triage-tdd.stage":                   handoffStage("triage-tdd"),
}
