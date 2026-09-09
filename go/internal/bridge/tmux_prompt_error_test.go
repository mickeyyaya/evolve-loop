package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type promptErrorTmux struct {
	*fakeTmux
	fail  string
	cause error
	calls []string
}

func (f *promptErrorTmux) LoadBuffer(context.Context, string, string) error {
	f.calls = append(f.calls, "load-buffer")
	if f.fail == "load-buffer" {
		return f.cause
	}
	return nil
}
func (f *promptErrorTmux) PasteBuffer(context.Context, string) error {
	f.calls = append(f.calls, "paste-buffer")
	if f.fail == "paste-buffer" {
		return f.cause
	}
	return nil
}
func (f *promptErrorTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if len(f.calls) > 0 && keys == "" && enter {
		f.calls = append(f.calls, "submit")
		if f.fail == "submit" {
			return f.cause
		}
	}
	return f.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestTmuxPromptTransportErrorStopsDelivery(t *testing.T) {
	for _, human := range []bool{false, true} {
		for i, stage := range []string{"load-buffer", "paste-buffer", "submit"} {
			t.Run(fmt.Sprintf("human=%t/%s", human, stage), func(t *testing.T) {
				cause := errors.New("fixture transport unavailable")
				tm := &promptErrorTmux{fakeTmux: &fakeTmux{paneSeq: []string{"❯"}}, fail: stage, cause: cause}
				cfg := fixtureConfig(t)
				cfg.HumanInput = human
				deps := fixtureDeps(tm)
				deps.LookupEnv = mapLookup(map[string]string{"BRIDGE_HUMAN_SIMULATION": "1", "EVOLVE_PHASE_RECOVERY": "off"})
				var log bytes.Buffer
				deps.Stderr = &log
				rc, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{name: "claude-tmux", session: "delivery-error", launchCmd: "claude", promptMarker: "❯", inputLineMarker: "❯", bootIntervalS: 1})
				if rc != ExitArtifactTimeout || !errors.Is(err, cause) {
					t.Errorf("delivery error lost: rc=%d err=%v", rc, err)
				}
				if want := []string{"load-buffer", "paste-buffer", "submit"}[:i+1]; !reflect.DeepEqual(tm.calls, want) {
					t.Errorf("continued after failed %s: calls=%v want=%v", stage, tm.calls, want)
				}
				if strings.Contains(log.String(), "prompt delivered") {
					t.Error("failed transport logged successful delivery")
				}
				if !strings.Contains(log.String(), artifactTimeoutMarker+"phase=build") || !strings.Contains(log.String(), stage) {
					t.Errorf("missing classified delivery cause: %s", log.String())
				}
				events, _ := os.ReadFile(filepath.Join(cfg.Workspace, "build-interactions.ndjson"))
				if strings.Contains(string(events), "submit_verified") {
					t.Errorf("failed transport recorded verified submission: %s", events)
				}
			})
		}
	}
}
