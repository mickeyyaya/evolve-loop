package bridge

import (
	"reflect"
	"testing"
)

var effortSupportingCLIs = []string{"claude-tmux", "codex-tmux"}

var effortNoopCLIs = []string{"agy-tmux", "ollama-tmux"}

func emitCount(r Realization) int { return len(r.LaunchFlags) + len(r.REPLInput) }

// The positive arm (claude/codex) guards against a vacuous pass: an all-noop
// wiring would fail here too.
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
			lo := Realize(m, LaunchIntent{ModelTier: "deep", Effort: "low"})
			if reflect.DeepEqual(hi, lo) {
				t.Errorf("%s effort dial ineffective: high and low realize identically: %+v", cli, hi)
			}
			if def := m.Params["effort"].Default; def != "" {
				want := Realize(m, LaunchIntent{ModelTier: "deep", Effort: def})
				if !reflect.DeepEqual(base, want) {
					t.Errorf("%s declares effort default=%q but an unset intent does not realize it: base=%+v want=%+v", cli, def, base, want)
				}
			} else if emitCount(hi) <= emitCount(base) {
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

			m2 := m
			m2.Params = make(map[string]ParamSpec, len(m.Params))
			for k, v := range m.Params {
				if k != "effort" {
					m2.Params[k] = v
				}
			}
			preEffort := Realize(m2, intent)

			if def := m.Params["effort"].Default; def != "" {
				explicit := Realize(m, LaunchIntent{ModelTier: "deep", Permission: "bypass", Effort: def})
				if !reflect.DeepEqual(withEffortParam, explicit) {
					t.Errorf("%s: unset Effort must realize the manifest default %q: got=%+v want=%+v", cli, def, withEffortParam, explicit)
				}
			} else if !reflect.DeepEqual(withEffortParam, preEffort) {
				t.Errorf("%s: unset Effort must be byte-identical to a pre-effort manifest: with=%+v pre-effort=%+v", cli, withEffortParam, preEffort)
			}
		})
	}
}
