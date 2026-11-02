package main

import (
	"context"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var smokeArtifactPath = regexp.MustCompile(`OK to (\S+) and`)

type answeringTmux struct {
	*bridge.FakeTmuxController
}

func (a answeringTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := a.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	prompt, err := os.ReadFile(a.LoadedBuffers[len(a.LoadedBuffers)-1])
	if err != nil {
		return err
	}
	if m := smokeArtifactPath.FindSubmatch(prompt); m != nil {
		return os.WriteFile(string(m[1]), []byte("OK\n"), 0o644)
	}
	return nil
}

func probeDeps(tmux bridge.TmuxController) bridge.Deps {
	return bridge.Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: func(string) (string, bool) { return "", false }, Stderr: io.Discard}
}

func credentialBenched(t *testing.T, family string) (string, *clihealth.Store) {
	t.Helper()
	root := t.TempDir()
	store := clihealth.NewStore(root, nil)
	if _, err := store.BenchWall(family, clihealth.CredentialPattern, "Please log in"); err != nil {
		t.Fatal(err)
	}
	return root, store
}

func TestCLIUpdateWiring_AfterASuccessfulLiveProbeTheCredentialWallNoLongerSkipsTheFamily(t *testing.T) {
	t.Parallel()
	root, store := credentialBenched(t, "codex")
	frames := []string{"›", "working ›", "working ›", "done ›", "cleanup", "cleanup", "cleanup", "cleanup"}
	w := cliUpdateWiringWith(root, probeDeps(answeringTmux{&bridge.FakeTmuxController{CaptureFrames: slices.Repeat(frames, 8)}}), io.Discard)
	if ok, why := w.seams.Eligible("codex"); ok || !strings.Contains(why, "the bench clears itself once a probe succeeds") {
		t.Fatalf("ok=%v why=%q; a credential wall skips the family until a probe succeeds", ok, why)
	}

	if err := w.seams.Probe(context.Background(), "codex"); err != nil {
		t.Fatalf("the live probe failed: %v", err)
	}

	if ok, why := w.seams.Eligible("codex"); !ok || why != "" {
		t.Fatalf("ok=%v why=%q; the successful probe must clear the wall", ok, why)
	}
	if e, benched := store.Active()["codex"]; benched {
		t.Fatalf("the credential wall stayed after the probe succeeded: %+v", e)
	}
}

func TestCLIUpdateWiring_AFailedLiveProbeKeepsTheCredentialWall(t *testing.T) {
	t.Parallel()
	root, store := credentialBenched(t, "codex")
	w := cliUpdateWiringWith(root, probeDeps(&bridge.FakeTmuxController{CaptureFrames: slices.Repeat([]string{"starting"}, 256)}), io.Discard)

	if err := w.seams.Probe(context.Background(), "codex"); err == nil {
		t.Fatal("a pane that never boots passed the live probe")
	}

	if ok, _ := w.seams.Eligible("codex"); ok {
		t.Fatal("a failed probe cleared the credential wall")
	}
	if _, benched := store.Active()["codex"]; !benched {
		t.Fatal("a failed probe cleared the credential wall")
	}
}

func TestNewUsageProber_TheLoopCenterReceivesTheAuthBenchClear(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	p := newUsageProber(usageProbeDirs{projectRoot: t.TempDir(), evolveDir: t.TempDir()}, []string{"claude"}, signals, io.Discard)
	if p.Signals != signals {
		t.Fatal("the usage prober does not carry the loop's signal center")
	}
}
