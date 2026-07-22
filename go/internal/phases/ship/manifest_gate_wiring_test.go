package ship

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// noopRunner is an inert subprocess seam: the wiring assertions never shell out.
func noopRunner(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return 0, nil
}

// TestShipOptions_ThreadsManifestGate is the cycle-1064 wiring crux: the ship
// PhaseRunner's PhaseRequest→Options translation must carry the config-sourced
// manifest-gate mode. Today Options.ManifestGate is never assigned at the sole
// production construction site (ship.go runNative), so the dial is permanently
// "" (shadow) no matter what policy.json says — the gate is unreachable short of
// a code edit.
//
// The translation is asserted through the exported constructor + the
// shipOptions seam (the extracted Options literal runNative uses), so the test
// exercises the real production path rather than a source-grep.
func TestShipOptions_ThreadsManifestGate(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       1064,
		ProjectRoot: "/repo",
		Workspace:   "/repo/.evolve/runs/cycle-1064",
		RunID:       "run-abc",
		Env:         map[string]string{"EVOLVE_PLUGIN_ROOT": "/plugin"},
	}
	for _, tc := range []struct{ name, gate, want string }{
		{"enforce flows through", "enforce", "enforce"},
		{"shadow flows through", "shadow", "shadow"},
		{"unset stays empty (shadow)", "", ""},
	} {
		p := New(Config{Runner: noopRunner, PhaseIO: config.StageEnforce, ManifestGate: tc.gate})
		opts := p.shipOptions(req, "evolve-cycle 1064")
		if opts.ManifestGate != tc.want {
			t.Errorf("%s: Options.ManifestGate = %q, want %q", tc.name, opts.ManifestGate, tc.want)
		}
		// Regression axis: the other translated fields must not be disturbed.
		if opts.ProjectRoot != req.ProjectRoot || opts.WorkspacePath != req.Workspace ||
			opts.RunID != req.RunID || opts.PluginRoot != "/plugin" ||
			opts.CommitMessage != "evolve-cycle 1064" || opts.Class != ClassCycle ||
			opts.PhaseIO != config.StageEnforce {
			t.Errorf("%s: translation regressed: %+v", tc.name, opts)
		}
	}
}

// TestManifestGate_PolicyToBlockEndToEnd closes the two halves into one chain:
// a policy.json `gates.manifest_gate: "enforce"` resolves through
// GatesConfig(), threads into the ship Options, and actually BLOCKS an
// out-of-manifest path with the dedicated code. The shadow row is the negative
// axis — the same chain with the default value must NOT block.
func TestManifestGate_PolicyToBlockEndToEnd(t *testing.T) {
	for _, tc := range []struct {
		name      string
		policyRaw string
		wantBlock bool
	}{
		{"enforce blocks", `{"gates":{"manifest_gate":"enforce"}}`, true},
		{"default shadow does not block", `{}`, false},
	} {
		var pol policy.Policy
		if err := json.Unmarshal([]byte(tc.policyRaw), &pol); err != nil {
			t.Fatalf("%s: unmarshal policy: %v", tc.name, err)
		}
		resolved := pol.GatesConfig().ManifestGate

		opts := manifestLeakOpts(t)
		opts.ManifestGate = New(Config{Runner: noopRunner, ManifestGate: resolved}).
			shipOptions(core.PhaseRequest{ProjectRoot: opts.ProjectRoot, Workspace: opts.WorkspacePath}, "msg").ManifestGate

		err := reconcileManifest(context.Background(), opts, &RunResult{}, "wt", "main", "cycle")
		if tc.wantBlock {
			se, ok := core.AsShipError(err)
			if !ok || se.Code != core.CodeManifestGate {
				t.Errorf("%s: want a MANIFEST_GATE block from policy=%q, got err=%v", tc.name, resolved, err)
			}
		} else if err != nil {
			t.Errorf("%s: policy=%q must not block, got %v", tc.name, resolved, err)
		}
	}
}
