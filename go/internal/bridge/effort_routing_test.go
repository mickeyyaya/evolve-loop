package bridge

// effort_routing_test.go — cycle-566 RED tests for per-phase reasoning-EFFORT
// routing (inbox `per-phase-effort-routing`, weight 0.88). Triage committed ONLY
// the plumbing slice: an abstract `effort` (low|medium|high) dimension added to
// LaunchIntent, realized per-manifest to each CLI's native mechanism (claude
// effort flag, codex reasoning_effort; agy/ollama noop) — mirroring the existing
// model_tier params.channel pattern. Retry-escalation + telemetry/soak are
// explicitly OUT of scope this cycle (see triage-report.md Rationale).
//
// RED now: LaunchIntent carries no Effort field, so this file does not compile
// until Builder adds the field + the realizeScalar("effort", intent.Effort) call
// + the manifest `params.effort` entries. GREEN once effort is realized through an
// effective channel for the supporting CLIs and cleanly no-ops for the rest.

import (
	"reflect"
	"testing"
)

// effortSupportingCLIs are the embedded tmux manifests whose CLI exposes a native
// reasoning-effort dial the manifest MUST translate the abstract effort vocabulary
// through (inbox: claude effort param, codex reasoning_effort).
var effortSupportingCLIs = []string{"claude-tmux", "codex-tmux"}

// effortNoopCLIs are the embedded tmux manifests whose CLI has no effort dial; the
// abstract effort MUST cleanly no-op — never abort the launch, never emit a stray
// flag — the same parity contract model_tier holds for a positional/single-model CLI.
var effortNoopCLIs = []string{"agy-tmux", "ollama-tmux"}

// emitCount is the size of a realization's observable emission surface (launch
// flags + REPL input). Effort translating "through some effective channel" means
// this count grows when effort is supplied; a clean no-op leaves it unchanged.
func emitCount(r Realization) int { return len(r.LaunchFlags) + len(r.REPLInput) }

// TestEffortRealize_Matrix — AC-A: manifests map the abstract effort onto each
// CLI's native mechanism; unsupported CLIs no-op (the parity contract). The
// positive arm (claude/codex) is the anti-no-op guard: an all-noop implementation
// that never wires effort would fail here, so the predicate cannot pass vacuously.
func TestEffortRealize_Matrix(t *testing.T) {
	injectCatalogDir(t, t.TempDir()) // neutralize the live-catalog overlay (model_tier parity precedent)

	for _, cli := range effortSupportingCLIs {
		t.Run("supported/"+cli, func(t *testing.T) {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", cli, err)
			}
			base := Realize(m, LaunchIntent{ModelTier: "deep"})
			hi := Realize(m, LaunchIntent{ModelTier: "deep", Effort: "high"})
			if emitCount(hi) <= emitCount(base) {
				t.Errorf("%s does not translate abstract effort=high through any effective channel (flag/repl): base=%+v high=%+v", cli, base, hi)
			}
		})
	}

	for _, cli := range effortNoopCLIs {
		t.Run("noop/"+cli, func(t *testing.T) {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", cli, err)
			}
			base := Realize(m, LaunchIntent{ModelTier: "deep"})
			hi := Realize(m, LaunchIntent{ModelTier: "deep", Effort: "high"})
			if !reflect.DeepEqual(base, hi) {
				t.Errorf("%s must NO-OP an unsupported abstract effort (no stray flag, no abort): base=%+v high=%+v", cli, base, hi)
			}
		})
	}
}

// TestEffortRealize_AbsentByteIdentical — AC-C regression: with Effort unset the
// realization is byte-identical to the one produced by a manifest that predates
// the feature (its `effort` param stripped). Guarantees the effort dimension is
// purely additive — an unpinned launch behaves exactly as it does today. The
// explicit Effort:"" also forces the compile dependency so this stays RED until
// the field lands.
func TestEffortRealize_AbsentByteIdentical(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	allCLIs := append(append([]string{}, effortSupportingCLIs...), effortNoopCLIs...)
	for _, cli := range allCLIs {
		t.Run(cli, func(t *testing.T) {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", cli, err)
			}
			intent := LaunchIntent{ModelTier: "deep", Permission: "bypass", Effort: ""}
			withEffortParam := Realize(m, intent)

			// A manifest as it existed before the effort feature: same params
			// minus the effort entry.
			m2 := m
			m2.Params = make(map[string]ParamSpec, len(m.Params))
			for k, v := range m.Params {
				if k != "effort" {
					m2.Params[k] = v
				}
			}
			preEffort := Realize(m2, intent)

			if !reflect.DeepEqual(withEffortParam, preEffort) {
				t.Errorf("%s: unset Effort must be byte-identical to a pre-effort manifest: with=%+v pre-effort=%+v", cli, withEffortParam, preEffort)
			}
		})
	}
}
