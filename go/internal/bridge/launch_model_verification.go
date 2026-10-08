package bridge

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeDispatchModelMismatch signalcenter.Code = "BRIDGE_DISPATCH_MODEL_MISMATCH"

const (
	unreadableModelFamily = "unreadable"
	ambiguousModelFamily  = "ambiguous"
)

const modelLabelTailLines = 6

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeDispatchModelMismatch, "a target whose manifest declares launch_model_verification showed no footer label of its model_family within label_wait_s (the label on the footer-prefix line decides; any other label never counts; no prefix line means unreadable, which means 87; a footer label naming two families is no evidence), after the REPL booted and before the prompt was delivered; the session is killed and the launch exits 87, a walk trigger, so the attempt moves to the next CLI. The mismatch itself benches nothing; the usage-evidence decorator queries the family's usage as for any quota-explainable exit, and an exhausted verdict benches it (agy boots its Gemini default when the Claude group is exhausted); fields.expected (the target's model_family), observed (the last labelled footer's family, ambiguous, or unreadable), label (the last footer label read), cli, phase")
}

type LaunchModelVerification struct {
	LabelWaitS int `json:"label_wait_s"`
}

type launchModelCheck struct {
	family     string
	labelRegex string
	waitS      int
}

func (c launchModelCheck) enabled() bool { return c.family != "" }

type launchModelVerifier interface {
	launchModelCheck() (launchModelCheck, error)
}

func launchModelCheckFor(cli string) (launchModelCheck, error) {
	m, err := LoadManifest(cli)
	if err != nil {
		return launchModelCheck{}, err
	}
	if m.LaunchModelVerification == nil {
		return launchModelCheck{}, nil
	}
	return launchModelCheck{family: m.ModelFamily, labelRegex: m.ModelLabelRegex, waitS: m.LaunchModelVerification.LabelWaitS}, nil
}

func VerifiesLaunchModel(cli string) (bool, error) {
	d, ok := LookupDriver(cli)
	if !ok {
		return false, fmt.Errorf("bridge: no driver registered for %s", cli)
	}
	verifier, ok := d.(launchModelVerifier)
	if !ok {
		return false, nil
	}
	check, err := verifier.launchModelCheck()
	return check.enabled(), err
}

func validateLaunchModelVerification(cli string, m Manifest) error {
	rule := m.LaunchModelVerification
	switch {
	case rule == nil:
		return nil
	case m.ModelFamily == "":
		return fmt.Errorf("bridge:manifest: launch_model_verification for cli=%s needs a model_family to compare the booted model with", cli)
	case m.ModelLabelRegex == "":
		return fmt.Errorf("bridge:manifest: launch_model_verification for cli=%s needs a model_label_regex to read the booted model", cli)
	case rule.LabelWaitS <= 0:
		return fmt.Errorf("bridge:manifest: launch_model_verification.label_wait_s for cli=%s must be positive, got %d", cli, rule.LabelWaitS)
	}
	if err := (paneVocabularyRule{requiredGroups: []string{"footer"}}).check(m.ModelLabelRegex); err != nil {
		return fmt.Errorf("bridge:manifest: launch_model_verification for cli=%s reads only the footer-prefix line, so its model_label_regex needs the footer prefix as a named group: %w", cli, err)
	}
	return nil
}

func (lp tmuxLaunch) withModelCheck(check launchModelCheck) tmuxLaunch {
	lp.modelCheck = check
	return lp
}

type bootedModelRun struct {
	cfg       *Config
	deps      Deps
	responder *autoResponder
}

type footerLabel struct {
	label  string
	family string
}

type labelWait struct {
	footer    footerLabel
	verified  bool
	died      bool
	cancelled bool
	guardErr  error
}

func (lp tmuxLaunch) verifyBootedModel(ctx context.Context, run bootedModelRun) int {
	if !lp.modelCheck.enabled() {
		return ExitOK
	}
	w := lp.awaitDispatchedFamily(ctx, run)
	switch {
	case w.verified:
		fmt.Fprintf(run.deps.Stderr, "[%s] launch model verified: %q is %s\n", lp.name, w.footer.label, lp.modelCheck.family)
		return ExitOK
	case w.cancelled:
		return lp.endCancelledWait(ctx, run)
	case w.died:
		return lp.handOffDeadPane(run)
	case w.guardErr != nil:
		fmt.Fprintf(run.deps.Stderr, "[%s] FAIL: %v\n", lp.name, w.guardErr)
		return ExitREPLBootTimeout
	}
	observed := w.footer.observed()
	fmt.Fprintf(run.deps.Stderr, "[%s] FAIL: launch model mismatch: the target dispatches the %s family, the footer shows %q (%s) after %ds; the session is killed and the walk moves on\n", lp.name, lp.modelCheck.family, w.footer.label, observed, lp.modelCheck.waitS)
	if lp.named {
		_ = killSessionSwept(ctx, run.deps, lp.session)
	}
	run.deps.Signals.Emit(lp.mismatchEvent(run.cfg, w.footer.label, observed))
	return ExitModelMismatch
}

func (lp tmuxLaunch) awaitDispatchedFamily(ctx context.Context, run bootedModelRun) labelWait {
	var last footerLabel
	for waited := 0; ctx.Err() == nil; waited++ {
		if lp.paneDied(ctx, run.deps) {
			return labelWait{footer: last, died: true}
		}
		if pane, err := run.deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback); err == nil {
			repoll, guardErr := run.responder.bootTick(ctx, lp.session, pane)
			if guardErr != nil {
				return labelWait{footer: last, guardErr: guardErr}
			}
			footer := footerLabelOf(panestream.FooterModelLabel(pane, lp.modelCheck.labelRegex, modelLabelTailLines))
			if footer.family == lp.modelCheck.family && !repoll {
				return labelWait{footer: footer, verified: true}
			}
			last = cmp.Or(footer, last)
		}
		if waited >= lp.modelCheck.waitS {
			return labelWait{footer: last}
		}
		run.deps.Sleep(time.Second)
	}
	return labelWait{footer: last, cancelled: true}
}

func footerLabelOf(label string) footerLabel {
	switch families := modelquery.FamiliesIn(label); len(families) {
	case 0:
		return footerLabel{}
	case 1:
		return footerLabel{label: label, family: families[0]}
	default:
		return footerLabel{label: label, family: ambiguousModelFamily}
	}
}

func (f footerLabel) observed() string {
	return cmp.Or(f.family, unreadableModelFamily)
}

func (lp tmuxLaunch) endCancelledWait(ctx context.Context, run bootedModelRun) int {
	fmt.Fprintf(run.deps.Stderr, "[bridge] %scause=%s phase=%s waited=0s reason=%q\n",
		artifactTimeoutMarker, artifactTimeoutContextCancelled, orDefault(run.cfg.Agent, lp.name), "the launch was cancelled during the launch model check: "+context.Cause(ctx).Error())
	return ExitArtifactTimeout
}

func (lp tmuxLaunch) paneDied(ctx context.Context, deps Deps) bool {
	if !deps.Tmux.HasSession(ctx, lp.session) {
		return true
	}
	if !lp.guardDeadShell {
		return false
	}
	_, isShell := paneShellProcess(ctx, deps.Tmux, lp.session)
	return isShell
}

func (lp tmuxLaunch) handOffDeadPane(run bootedModelRun) int {
	fmt.Fprintf(run.deps.Stderr, "[bridge] %sphase=%s waited=0s transient=true reason=%q\n",
		artifactTimeoutMarker, orDefault(run.cfg.Agent, lp.name), "the REPL died during the launch model check (dead shell): one fresh session of the same CLI")
	observeFatalPane(run.deps, fatalPaneObservation{cause: recovery.CauseDeadShell, named: lp.named, intervalS: 1})
	return ExitArtifactTimeout
}

func (lp tmuxLaunch) mismatchEvent(cfg *Config, label, observed string) signalcenter.Event {
	id := configIdentity(cfg)
	return signalcenter.Event{
		Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleBridge, Origin: "tmuxLaunch.verifyBootedModel",
		Kind: signalcenter.KindBridgeWarning, Severity: signalcenter.SeverityWarn, Code: CodeDispatchModelMismatch,
		Reason: fmt.Sprintf("%s booted %s, not the %s family it dispatches: killed before the prompt, the walk moves to the next CLI", lp.name, observed, lp.modelCheck.family),
		Fields: map[string]string{"expected": lp.modelCheck.family, "observed": observed, "label": label, "cli": lp.name, "phase": id.phase},
	}
}
