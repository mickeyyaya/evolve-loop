package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// replWaiter owns the completion wait state machine after prompt dispatch. Its
// dependencies are fixed for one launch, which keeps the state transitions
// testable without widening the package API.
type replWaiter struct {
	ctx            context.Context
	cfg            *Config
	deps           Deps
	launch         tmuxLaunch
	prefix         string
	phaseName      string
	resolvedPrompt string
	artifactBase   artifactBaseline
	responder      *autoResponder
	recorder       *interaction.Recorder
	cursor         *inbox.Cursor
	channel        *replLiveChannel
}

type replWaitResult struct {
	lastGoodPane string
	peakTokens   int
}

func (r *replWaitResult) recordTokens(pane string) {
	if tokens := panestream.ExtractResponseTokens(pane); tokens > r.peakTokens {
		r.peakTokens = tokens
	}
}

func newReplInboxCursor(cfg *Config) *inbox.Cursor {
	cursor := inbox.NewCursor(cfg.Workspace, cfg.Agent)
	if fi, err := os.Stat(inbox.Path(cfg.Workspace, cfg.Agent)); err == nil {
		cursor.SetOffset(fi.Size())
	}
	return cursor
}

func (w replWaiter) wait() (replWaitResult, int) {
	ctx := w.ctx
	cfg := w.cfg
	deps := w.deps
	lp := w.launch
	pfx := w.prefix
	phaseName := w.phaseName
	resolvedPrompt := w.resolvedPrompt
	artifactBase := w.artifactBase
	ar := w.responder
	irec := w.recorder
	cursor := w.cursor
	channel := w.channel
	paneProfile := channel.profile

	// --- Wait for the artifact in review intervals. A hard wall-clock deadline
	// kills a slow-but-productive agent (it cannot tell "stuck" from "still
	// thinking"). Instead, when a review interval elapses without the artifact,
	// a StopReviewer adjudicates the evidence — did the agent emit new output? —
	// into extend (still working, wait another interval) or pause (stalled,
	// surface for investigation, do not silently kill). See stopreview.go +
	// ADR-0026. The interval defaults to tmuxArtifactTimeoutS,
	// overridable per-launch (cfg.ArtifactTimeoutS) or via Deps.ArtifactTimeoutS (BridgePolicy).
	interval := cfg.ArtifactTimeoutS
	if interval <= 0 {
		interval = defaultIfZero(deps.ArtifactTimeoutS, tmuxArtifactTimeoutS)
	}
	// The configured extend backstop, resolved once: it both builds the default
	// reviewer and is REPORTED in the timeout summary (an operator reading
	// "extends_used=6 max_extends=6" knows the budget was the wall, not a stall).
	// The <=0 clamp mirrors newDeterministicReviewer's, so the number reported is
	// the number actually enforced.
	maxExtends := defaultIfZero(deps.ArtifactMaxExtends, defaultArtifactMaxExtends)
	if maxExtends <= 0 {
		maxExtends = defaultArtifactMaxExtends
	}
	// Defensive default: the Engine path sets deps.Reviewer via withDefaults,
	// but direct runTmuxREPL callers (tests, future Stage-1 wiring) may not —
	// avoid a nil-deref at the review checkpoint.
	reviewer := deps.Reviewer
	if reviewer == nil {
		reviewer = newDeterministicReviewer(maxExtends)
	}
	// SignalCenter (ADR-0068, S3): the authoritative liveness source for the
	// checkpoint below — Observe+Aggregate replace the bare per-run detectorFor(lp)
	// probe. deps.LivenessCenter injects a shared/test instance; nil (production)
	// builds a private center whose empty registry makes Observe fall back to
	// panestream.DetectorFor(profile) — the SAME probe detectorFor(lp) built, so
	// the migration is verdict-identical (H1) unless a caller has registered a
	// handler for this profile's name.
	livenessCenter := deps.LivenessCenter
	if livenessCenter == nil {
		livenessCenter = panestream.NewSignalCenter()
	}
	// ADR-0044 C2: the fatal-pane registry consulted before each review
	// checkpoint (fatalpane.go). Stage off ⇒ fatalPaneVerdict short-circuits
	// before touching the detector; nil detector is unreachable on that path.
	recoveryStage := recoveryStageFromEnv(deps)
	var fatalDet *recovery.FatalPaneDetector
	if recoveryStage != "off" {
		// Seeds + durable advisor promotions (ADR-0044 Slice 5): a novel
		// signature classified once is caught deterministically on every
		// later boot. Empty ProjectRoot degrades to seeds only.
		fatalDet = recovery.SeedDetectorWithPromotions(
			filepath.Join(cfg.ProjectRoot, ".evolve", "instincts", "fatal-signatures"))
	}
	// ADR-0027: the completion contract is a Strategy. Default ("" / "artifact")
	// is the legacy artifact-file poll, byte-identical to the pre-Strategy code;
	// "stdout" completes on REPL-idle for agents that print their answer and
	// write no file (the router/advisor). The detector ONLY decides readiness —
	// the stop-review/extend liveness adjudication below is unchanged.
	var lastEv StopEvent
	var lastVerdict ReviewVerdict
	detector := newCompletionDetector(cfg.Completion, cfg, deps, lp, artifactBase)
	completed := false
	transientShortcircuit := false
	nudgeSent := false
	// ADR-0045 I1: the one-shot nudge's outcome window — resolved when the
	// run concludes, against the only evidence that matters: did the
	// artifact appear within the bounded wait? `nudgeSent=true` with no
	// outcome record is the cycles-263–269 defect this closes.
	var nudgeEv *interaction.Event
	var nudgeAt time.Time
	defer func() {
		if nudgeEv == nil {
			return
		}
		res := interaction.ResultNoEffect
		if completed {
			res = interaction.ResultArtifactAppeared
		}
		irec.Record(interaction.Outcome{
			Event:     *nudgeEv,
			Result:    res,
			LatencyMS: deps.Now().Sub(nudgeAt).Milliseconds(),
		})
	}()
	detectErrLogged := false
	peakTokens := 0
	recordTokens := func(pane string) {
		if n := panestream.ExtractResponseTokens(pane); n > peakTokens {
			peakTokens = n
		}
	}
	attempt := 0
	intervalStart := 0
	// waitedS mirrors the wait loop's `elapsed` into function scope so the
	// timeout summary below can report how long this launch actually waited.
	// `elapsed` is scoped to the for statement, and lastEv.ElapsedS is only
	// populated once a review CHECKPOINT is reached — a wait that ends before the
	// first checkpoint (ctx cancel) would report 0 and read as "died instantly".
	waitedS := 0
	intervalBaselinePane, baselineErr := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
	if baselineErr != nil {
		// This pane is submit-verify's ONLY observation on the clean path. A
		// failed capture makes the guard entirely blind, and an empty pane reads
		// as "input line clear" — indistinguishable from a verified submission
		// unless we say so. Silence here reproduced the pre-fix cycle-1505 log.
		fmt.Fprintf(deps.Stderr, "%s submit-verify: prompt NOT verified — baseline capture failed, input-line state unknown: %v\n", pfx, baselineErr)
	}
	recordTokens(intervalBaselinePane)
	// CB.6: the freshest non-empty pane seen — escalation evidence that
	// survives a mid-phase server death (cycle-286 masked-evidence class).
	lastGoodPane := intervalBaselinePane
	writeArtifactTimeoutMarker := func(transient bool) {
		fmt.Fprintf(deps.Stderr,
			"[bridge] %sphase=%s waited=%ds interval=%ds extends_used=%d max_extends=%d last_review=%s liveness=%s progressed=%v busy=%v transient=%v reason=%q\n",
			artifactTimeoutMarker, phaseName, waitedS, interval, attempt, maxExtends,
			reviewActionOrNone(lastVerdict.Action), livenessOrUnknown(lastEv.State),
			lastEv.Progressed, lastEv.Busy, transient, lastVerdict.Reason)
	}
	// Persistence guard for the checkpoint exhaustion fast-fail (exhaustion_persistence.go):
	// the wall must be present across consecutive checkpoints before failing over,
	// so wall-shaped text a working agent momentarily rendered does not kill it.
	// Its OWN gate (not shared with the fast-poll's) — the two loops observe at
	// different cadences and must each confirm on their own consecutive frames.
	checkpointExhaustGate := newExhaustionGate()
	checkpointWall := &checkpointWallState{}
	// Persistence guard for the ADR-0044 C2 fatal-pane fast-fail
	// (fatalpane_persistence.go) — same shape, same reason: the detector matches
	// substrings against the RAW pane, so a working agent quoting a fatal
	// signature must not be killed for one frame. Loop-scoped like the gate
	// above: a per-checkpoint instance could never accumulate a streak.
	checkpointFatalGate := newFatalPaneGate()
	// Cycle-274 post-paste spill check (R3.2), on the ALREADY-captured
	// baseline (no extra capture, no fixture-frame drift): the prompt was
	// just pasted; if it spilled into a shell continuation (quote>/bquote>)
	// AND the pane's foreground process IS a shell (authoritative — a pane
	// merely quoting spill text under a live CLI must not trip this), the
	// CLI is gone. Fail fast as a transient so the fallback chain takes
	// over, instead of the 25-min wedge cycles 274/277 burned. Mid-run
	// process death past this boundary is the observer's job (plan R3.4).
	if lp.guardDeadShell && paneLooksLikeShellSpill(intervalBaselinePane) {
		if shellCmd, isShell := paneShellProcess(ctx, deps.Tmux, lp.session); isShell {
			fmt.Fprintf(deps.Stderr, "%s FAIL: prompt spilled into a dead shell (%s) after paste — CLI process gone (cycle-274 class)\n", pfx, shellCmd)
			return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitREPLBootTimeout
		}
	}
	// Submit-verify the prompt delivery (driver_tmux_submitverify.go). Read off
	// the SAME already-captured baseline the spill check just used — no extra
	// capture, no fixture-frame drift — and ordered after it so a pane that
	// spilled into a dead shell fails fast rather than collecting Enters. The
	// paste is submitted with a blind Enter (or the human-cadence review path);
	// if the prompt is still sitting at the input line, re-send it, bounded.
	if !lp.bootOnly {
		// Codex 0.153 renders bracketed multiline input with this chip. Keep
		// the additional echo local to Codex and to this prompt submission.
		codexPasteEcho := ""
		if lp.name == "codex-tmux" {
			codexPasteEcho = "[Pasted Content "
		}
		outcome := verifySubmitted(ctx, deps, lp, pfx, "prompt", intervalBaselinePane,
			promptSubmitEcho(resolvedPrompt), firstNonEmptyLine(resolvedPrompt), tmuxPastePlaceholderEcho, codexPasteEcho)
		recordSubmitVerify(irec, phaseName, cfg.Cycle, "prompt", outcome)
		if outcome.Result == interaction.ResultSubmitWedged {
			// GROUND TRUTH outranks the pane heuristic (v22.20.0 release red):
			// a REPL that consumes its input and answers by SIDE EFFECT alone —
			// never redrawing its input line — is indistinguishable from
			// "parked" by the echo match, and the instant 81 here aborted
			// sessions whose deliverable was already on disk. One read-only
			// probe: an artifact present that is NOT the pre-dispatch baseline
			// proves the submission landed; fall through to the normal wait
			// (the detector's stability window still gates completion). A
			// genuinely wedged pane has no post-dispatch artifact and still
			// fast-fails exactly as the retro-stall fix intended.
			// NOTE: a submit_wedged outcome was already recorded to the
			// interactions ledger above — with this belt, that record can
			// co-occur with a SUCCESSFUL phase (the stall was recovered by
			// ground truth). Consumers must gate on the actual phase error,
			// never on the ledger token alone (failure_learning.go does).
			delivered := false
			if path, found := artifactLocate(cfg); found {
				// Lstat, matching regularFileNonEmpty's never-follow-symlinks
				// invariant (cycle-1256 D3) — the belt must not re-resolve a
				// path the locator deliberately refused to follow.
				if fi, serr := os.Lstat(path); serr == nil && fi.Mode().IsRegular() && !artifactBase.matches(path, fi) {
					delivered = true
				}
			}
			if delivered {
				fmt.Fprintf(deps.Stderr, "%s submit-verify: pane looks parked but a post-dispatch deliverable is already on disk — submission evidently landed; continuing the normal wait\n", pfx)
			} else {
				lastVerdict = ReviewVerdict{
					Action: ReviewPause,
					Reason: fmt.Sprintf("prompt %s (resends=%d)", outcome.Result, outcome.Resends),
				}
				writeArtifactTimeoutMarker(false)
				return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitArtifactTimeout
			}
		}
	}
	ar.transientDwellEnabled = true
	for elapsed := 0; ; elapsed += 2 {
		deps.Sleep(2 * time.Second)
		waitedS = elapsed
		if err := ctx.Err(); err != nil {
			// Context cancelled (orchestrator timeout / SIGTERM / the next phase
			// tearing down this session): before abandoning, ONE final completion
			// poll — a deliverable already on disk means the session COMPLETED and
			// the cancel is benign teardown, not a phase timeout. Pre-fix this
			// break skipped straight to the !completed → ExitArtifactTimeout exit,
			// laundering a finished session into a timeout (session-lifecycle
			// residual; the runner's settle-retry was the only thing standing
			// between that mislabel and a false FAIL). A genuinely unfinished
			// session still exits ExitArtifactTimeout.
			//
			// The poll gets a context DETACHED from the cancellation (cycle-1236):
			// this line dispatches to all three completionDetector strategies, and
			// only artifactDetector is a pure file stat. stdoutDetector shells
			// CapturePane and gitEvidenceDetector shells git — exec.CommandContext
			// refuses to fork on a dead ctx, both swallow the transport error as
			// "not ready", and a DELIVERED stdout/git phase exited 81. withFinalPoll
			// hands them a live, finalPollGrace-bounded context carrying the
			// explicit finality marker artifactDetector's short-circuit now keys on
			// (a live ctx alone would have disarmed it — completion.go).
			finalCtx, finalCancel := withFinalPoll(ctx)
			ready, _, note, _ := detector.poll(finalCtx)
			finalCancel()
			if ready {
				completed = true
				if note != "" {
					fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
				}
				fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) AFTER completion — benign teardown of a finished session\n", pfx, err)
				break
			}
			// Load-bearing once a Stage-1 LLM reviewer can extend at length:
			// stop waiting promptly rather than running out the extend budget.
			fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) — abandoning completion wait\n", pfx, err)
			break
		}
		// Live channel: stream newly-stabilized rendered content to pane.live.
		// The first Next() primes the baseline (echoed prompt + boot chrome are
		// counted, not emitted); later ticks emit only the assistant output that
		// appeared above the volatile input box. Gated so off adds no capture.
		var waitPane string
		channelCaptureOK := true
		if channel.on {
			// This is the canonical pane observation for channel-enabled waits;
			// taking another in auto-response can skip alternating dwell frames.
			var captureErr error
			waitPane, captureErr = deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			channelCaptureOK = captureErr == nil
			if captureErr == nil {
				recordTokens(waitPane)
				channel.streamPane(waitPane)
			}
		}
		ready, _, note, derr := detector.poll(ctx)
		if ready {
			completed = true
			if note != "" {
				fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
			}
			break
		}
		if derr != nil && !detectErrLogged {
			// The detector surfaced a fault (e.g. an artifact present at a
			// non-canonical path that could not be relocated — read-only
			// workspace). Surface it once, immediately, instead of spinning the
			// full wait window with no signal.
			fmt.Fprintf(deps.Stderr, "%s WARN: completion detector: %v\n", pfx, derr)
			detectErrLogged = true
		}
		waitCaptureOK := true
		if !channel.on {
			// Preserve the legacy ordering: completed artifact waits break above
			// without consuming a pane frame solely for auto-response.
			var werr error
			waitPane, werr = deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			waitCaptureOK = werr == nil
		}
		// Drain live-injection envelopes BEFORE the auto-respond tick so an
		// operator interrupt pre-empts a pending auto-reply on this tick.
		if envs, _ := cursor.Drain(); len(envs) > 0 {
			for _, env := range envs {
				// injectEnvelope returns a non-empty CorrID only when an
				// idle-gated correlated ask was actually pasted (not re-queued,
				// dropped, or a keystroke/interrupt). The breadcrumb is emitted
				// HERE — at the moment delivery is confirmed — so the channel sink
				// (and the open-span tracking) lives entirely in this loop. Gated:
				// channel off → no breadcrumb, no span tracking.
				cid := injectEnvelope(ctx, cfg, deps, lp, env)
				channel.injectionDelivered(cid)
			}
		}
		channel.observeIdle(waitPane, livenessCenter)
		action, rc := ar.tickPane(ctx, lp.session, waitPane, channelCaptureOK && waitCaptureOK)
		switch rc {
		case 0, 1: // noop / responded
		case 2:
			// Agent self-signalled progress ("extend_timeout"): restart the
			// current review interval so the signal counts as activity. Bounded
			// by the auto-respond loop guard (case 86) — an agent cannot defer
			// the reviewer indefinitely by repeating the same extend prompt.
			if parseExtendSecs(action) > 0 {
				intervalStart = elapsed
				fmt.Fprintf(deps.Stderr, "%s agent extend signal — review interval refreshed\n", pfx)
			}
		case 3:
			if strings.TrimSpace(waitPane) != "" {
				lastGoodPane = waitPane
			}
			lastEv = StopEvent{
				Kind:           StopArtifactTimeout,
				Phase:          cfg.Agent,
				Cycle:          cfg.Cycle,
				ElapsedS:       elapsed,
				IntervalS:      interval,
				Busy:           false,
				StdoutTail:     lastLines(lastGoodPane, 40),
				InjectedPrompt: ar.injectedPrompt,
				State:          panestream.LivenessIdle,
			}
			lastVerdict = ReviewVerdict{Action: ReviewStop, Reason: "transient upstream error persisted for 60s"}
			transientShortcircuit = true
		case 85:
			fmt.Fprintf(deps.Stderr, "%s auto-respond escalation; abandoning run\n", pfx)
			return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitUnknownPrompt
		case 86:
			fmt.Fprintf(deps.Stderr, "%s auto-respond loop guard tripped; abandoning run\n", pfx)
			return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitRespondLoopGuard
		}
		if transientShortcircuit {
			break
		}
		// Review checkpoint: a full interval elapsed without the artifact.
		if elapsed-intervalStart >= interval {
			rawPane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			curPane, renderWedged := recoverBlankPane(ctx, deps, lp.session, lp.bootScrollback, rawPane, pfx)
			recordTokens(curPane)
			// CB.6: evidence survives the session's death. When the server
			// is killed mid-phase every later capture is empty, so the
			// escalation report's final_pane carried nothing and cycle-286's
			// retro misattributed the failure. Retain the last NON-EMPTY
			// pane; a dead capture falls back to it as the freshest real
			// evidence (the live pane still wins whenever it renders).
			if strings.TrimSpace(curPane) != "" {
				lastGoodPane = curPane
			}
			evidencePane := lastGoodPane
			// Progressed = the pane changed during the interval. Stage-0 signal:
			// good for the common cases (growing token counters, new tool calls),
			// but a pure spinner/clock animation also reads as progress — so the
			// maxExtends backstop, not this diff, bounds a spinner-stuck agent
			// (~maxExtends×interval). Stage 1's reviewer inspects StdoutTail to
			// disambiguate genuine work from animation.
			// Liveness WITHOUT the exhaustion override (empty ExhaustedRegex): a
			// wall in the pane must not collapse the real converging/idle signal,
			// because the exhaustion decision is made SEPARATELY below — on the
			// stripped pane and persistence-gated — so wall-shaped text a working
			// agent merely rendered cannot pause or kill it. Safe: nothing
			// consumes the center's Exhausted SignalEvent (no RegisterSignalHandler
			// caller); this fast-fail was its only actor.
			livenessProfile := paneProfile
			livenessProfile.ExhaustedRegex = ""
			livenessCenter.Observe(lp.session, curPane, livenessProfile)
			livenessState := livenessCenter.Aggregate()
			// Exhaustion fast-fail: a quota/rate-limit WALL means the artifact will
			// NEVER come — fail over to the fallback CLI (exit 85) instead of
			// burning the full artifact timeout. Scanned on the agent-stripped pane
			// and persistence-gated (exhaustion_persistence.go): a re-printing real wall is
			// present every checkpoint and still crosses (preserving the agy
			// hang-without-exit fix), while wall text passing through a WORKING
			// agent's pane clears by the next checkpoint — the raw-pane false-FAIL
			// class the go-review of the per-model regex fix surfaced.
			walled := livenessCenter.ExhaustedOf(strippedForExhaustionScan(curPane, ar.injectedPrompt), paneProfile)
			// Corroboration contract shared with the fast-poll site, one
			// probe per phase, latched (wallcorroborate.go checkpointWallState).
			if escalate, suppressNow := checkpointWall.decide(ctx, deps.CorroborateWall, lp.name, checkpointExhaustGate.observe(walled)); escalate {
				fmt.Fprintf(deps.Stderr, "%s EXHAUSTED: pane shows a quota/rate-limit wall (persisted %d checkpoints, corroborated by live probe) — failing over to fallback CLI (exit %d)\n", pfx, exhaustionPersistObservations, ExitUnknownPrompt)
				return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitUnknownPrompt
			} else if suppressNow {
				fmt.Fprintf(deps.Stderr, "%s EXHAUSTION-SUPPRESSED: pane matched wall vocabulary but a live probe (cheapest tier) answered — treating as content-induced; a tier-scoped wall would surface via artifact-timeout fallback instead\n", pfx)
			}
			// Progressed is sourced from the center's Changed(session)
			// projection (S4) — a consecutive-observation comparison, not the
			// interval-baseline diff. Behavior-preserving: .Progressed has no
			// decision consumer (evidence/logging only), so the shift from
			// baseline-relative to checkpoint-to-checkpoint is safe.
			progressed := livenessCenter.Changed(lp.session)
			// Render-wedge override (cycle-291): a blank pane from a live session
			// reads as Idle by the content-velocity detector (no affordance in blank
			// frame). recoverBlankPane already confirmed the session is alive
			// (renderWedged=true); treat as BusyButStagnant so the reviewer extends
			// rather than pausing a working agent on a pane-rendering failure.
			if renderWedged && livenessState == panestream.LivenessIdle {
				livenessState = panestream.LivenessBusyButStagnant
			}
			lastEv = StopEvent{
				Kind:       StopArtifactTimeout,
				Phase:      cfg.Agent,
				Cycle:      cfg.Cycle,
				ElapsedS:   elapsed,
				IntervalS:  interval,
				Attempt:    attempt,
				Progressed: progressed,
				Busy:       livenessCenter.Busy(lp.session) || renderWedged,
				StdoutTail: lastLines(evidencePane, 40),
				// Same source the exhaustion scan strips against, two lines up —
				// one resolved prompt, one meaning of "what the agent was told",
				// so the two detectors can never strip differently (cycle-1117).
				InjectedPrompt: ar.injectedPrompt,
				State:          livenessState,
			}
			// ADR-0044 C2: a known-fatal pane (model-invalid boot, CLI
			// self-update, dead shell) preempts the reviewer in enforce —
			// cycle-262's dead panes read as "progressed" because the
			// bridge's own nudge echoed into them, so the legacy
			// extend-while-progressing flow burned the full maxExtends
			// backstop on REPLs that no longer existed.
			// Persistence-gated (fatalpane_persistence.go): the match must be
			// present on consecutive checkpoints before it can preempt, so a
			// working agent that renders fatal-shaped text for one frame is
			// never killed for it, while a parked pane still exits one
			// checkpoint later than before.
			// The gate RECORDS a C2 evidence outcome on every gate-crossed
			// call (R8.3) — it must be called exactly once per stop-review
			// checkpoint, never retried for the same event, or the soak's C2
			// counts inflate silently.
			v, preempted := checkpointFatalGate.verdict(fatalDet, lastEv, recoveryStage, irec, deps.Stderr, pfx)
			if !preempted {
				v = reviewer.Review(lastEv)
			}
			lastVerdict = v
			fmt.Fprintf(deps.Stderr, "%s stop-review[%s] elapsed=%ds attempt=%d progressed=%v → %s: %s\n",
				pfx, StopArtifactTimeout, elapsed, attempt, progressed, lastVerdict.Action, lastVerdict.Reason)
			if deps.OnStopReview != nil {
				deps.OnStopReview(phaseName, string(lastVerdict.Action), lastVerdict.Reason)
			}
			if lastVerdict.Action != ReviewExtend {
				_, isDetVal := reviewer.(deterministicReviewer)
				_, isDetPtr := reviewer.(*deterministicReviewer)
				isDeterministic := isDetVal || isDetPtr
				// Nudge only on PAUSE (idle agent, remind it once). A fatal
				// ReviewStop (ADR-0044 C2) must exit now — nudging a dead
				// shell is exactly the echo that bought cycle-262's dead
				// panes their extensions. Behavior-identical for the legacy
				// reviewer, which only ever emits extend|pause.
				if lastVerdict.Action == ReviewPause && isDeterministic && !livenessCenter.Busy(lp.session) && !nudgeSent {
					nudgeMsg := fmt.Sprintf("Please write the deliverable to %s to complete the phase.", cfg.Artifact)
					_ = deps.Tmux.SendKeys(ctx, lp.session, nudgeMsg, true)
					// The recorded stall (cycles 1505/1510/1517): this nudge was
					// still parked at the `❯` input line in the final capture and
					// every interaction record read result=no_effect. Verify it
					// was submitted; re-send Enter, bounded, if it was not.
					// A fresh capture is unavoidable here: the nudge was sent
					// microseconds ago, so every earlier pane predates it.
					// Announce the nudge BEFORE verifying it, so an operator never reads
					// "re-sending Enter" above any line saying a nudge exists.
					fmt.Fprintf(deps.Stderr, "%s idle with missing artifact; sent one-shot nudge: %s\n", pfx, nudgeMsg)
					deps.Sleep(submitVerifySettle)
					nudgePane, nudgeCapErr := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
					if nudgeCapErr != nil {
						fmt.Fprintf(deps.Stderr, "%s submit-verify: nudge NOT verified — capture failed, input-line state unknown: %v\n", pfx, nudgeCapErr)
					}
					nudgeOutcome := verifySubmitted(ctx, deps, lp, pfx, "nudge", nudgePane, nudgeMsg)
					recordSubmitVerify(irec, phaseName, cfg.Cycle, "nudge", nudgeOutcome)
					if nudgeOutcome.Result == interaction.ResultSubmitWedged {
						lastVerdict.Reason = fmt.Sprintf("nudge %s (resends=%d)", nudgeOutcome.Result, nudgeOutcome.Resends)
						break
					}
					nudgeSent = true
					nudgeEv = &interaction.Event{
						Kind:    interaction.KindNudge,
						Phase:   phaseName,
						Cycle:   cfg.Cycle,
						Trigger: "idle_no_artifact",
						Payload: nudgeMsg,
					}
					nudgeAt = deps.Now()
					intervalStart = elapsed
					attempt++
					continue
				}
				break
			}
			attempt++
			intervalStart = elapsed
		}
	}
	if !completed {
		fmt.Fprintf(deps.Stderr, "%s FAIL: completion never signalled (artifact %s; stop-review paused after %d interval(s) of %ds)\n", pfx, cfg.Artifact, attempt+1, interval)
		fmt.Fprintf(deps.Stderr, "%s diagnostic: files present under workspace %s:\n", pfx, cfg.Workspace)
		for _, line := range listWorkspaceFiles(cfg.Workspace) {
			fmt.Fprintf(deps.Stderr, "%s   %s\n", pfx, line)
		}
		// Pause (ambiguous stall) and Stop (typed fatal fast-fail, ADR-0044
		// C2) both leave the operator-facing escalation report; extend keeps
		// the legacy no-report behavior.
		if lastVerdict.Action == ReviewPause || lastVerdict.Action == ReviewStop {
			_ = writeEscalationReport(cfg.Workspace, phaseName, cfg.Cycle, lastEv, lastVerdict)
		}
		// Fail-loud drift alarm (exhaustion_drift.go): if this timed-out pane looks
		// like a quota wall the exhausted_regex missed, the wall wording may have
		// drifted ahead of the pattern — surface it now so the NEXT drift is caught
		// in one cycle, not eight (as the per-model wording change was). Diagnostic
		// only; the exit-81 verdict stands.
		// Scanned on the SAME agent-stripped pane as the primary detector above
		// (strippedForExhaustionScan, line ~704): an alarm that scans the raw pane
		// fires on wall-shaped text the agent merely wrote or echoed, which the real
		// regex correctly ignored — an operator chasing a regex that is working.
		strippedPane := strippedForExhaustionScan(lastGoodPane, ar.injectedPrompt)
		warnExhaustionRegexDrift(deps.Stderr, pfx, lp.name, strippedPane, paneProfile.ExhaustedRegex)
		// Transient-upstream recognition (inbox item
		// transient-api-error-invisible-inside-artifact-timeout): 3 of 4 observed
		// router stalls (cycles 1523/1524/1526) spent the FULL silence budget on a
		// pane reading "API Error: 529 Overloaded … usually temporary" — a failure
		// no path recognizes. It is not a quota wall (exhausted_regex correctly
		// ignores it) and not a transient exit code (80/85/86). The distinguishing
		// evidence was already captured right here and simply never consulted.
		//
		// Family-agnostic by construction: the pattern is resolved from the
		// LAUNCHED cli's manifest (lp.name), so every CLI family declares its own
		// provider's transient signature and none of that vocabulary is hard-coded
		// here. A family that declares none is fail-open, not misreported.
		//
		// Carried as ONE driver-authored field on the marker line below, NOT as a
		// second cause candidate: extending that line means it can never displace
		// artifactTimeoutSummary's selection, so the class of regression where the
		// drift alarm or a workspace listing outranks the timeout summary stays
		// structurally unreachable. The exit code is untouched — 81 remains
		// non-transient (transient-bridge-retry AC-1) and the discrimination rides
		// the cause as data. A bool, never pane text, also keeps agent-authored
		// content out of the recorded cause (F1 indirect-prompt-injection).
		//
		// Scanned on the SAME agent-stripped pane as the drift alarm above: a raw
		// scan fires on error text the agent merely quoted, flagging a working
		// agent's own prose as an upstream outage.
		transient := classifyTransientPane(lp.name, strippedPane)
		// Self-describing death (inbox item deep-phase-artifact-budget-too-small):
		// exit 81 alone says nothing, and the diagnostic file listing printed above
		// used to become the recorded cause. This ONE marker line carries how long
		// the wait ran and how much of the extend budget it consumed, so the next
		// reader can separate "too slow — raise bridge.phase_artifact_timeout_s for
		// this phase" (extends_used == max_extends while the agent was busy) from
		// "wedged" (paused early, idle, no progress). The phase= field is
		// cfg.Agent — the SAME key bridge.phase_artifact_timeout_s is indexed on —
		// so the remedy is copy-pasteable from the diagnostic. Emitted LAST so it
		// is also the final stderr line, and lifted into the error by Engine.Launch.
		writeArtifactTimeoutMarker(transient)
		if transientShortcircuit && ctx.Err() == nil {
			// Deliberately a plain blocking pause: it throttles re-dispatch
			// against a provider that just spent 60s saying "overloaded", and
			// under cliadmit it holds this family's slot for the cooldown —
			// which IS the point during a provider-wide 529 storm. The ctx
			// guard skips it when the run is already cancelled; a mid-sleep
			// cancel waits out at most 15s, bounded and small against the
			// budget this path just saved.
			deps.Sleep(transientRedispatchDelay)
		}
		return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitArtifactTimeout
	}

	return replWaitResult{lastGoodPane: lastGoodPane, peakTokens: peakTokens}, ExitOK
}
