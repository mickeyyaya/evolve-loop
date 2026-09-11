package bridge

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

const (
	modelDispatchArgv       = llmcalls.DispatchArgv
	modelDispatchREPL       = llmcalls.DispatchREPL
	modelDispatchPositional = llmcalls.DispatchPositional
	modelDispatchCLIDefault = llmcalls.DispatchCLIDefault
	modelDispatchResumed    = llmcalls.DispatchResumed
	modelDispatchUnknown    = llmcalls.DispatchUnknown
)

// modelDispatch is the selector evidence observed at the actual process or
// session boundary. It never contains argv, prompts, or environment values.
type modelDispatch struct {
	model        string
	source       string
	selectorKind modelSelectorKind
	uncertain    bool
	terminated   bool
}

type modelSelectorKind uint8

// Selector kinds are ordered by Codex precedence. A dedicated CLI model flag
// overrides a generic config model even when the config token appears later.
const (
	modelSelectorNone modelSelectorKind = iota
	modelSelectorConfig
	modelSelectorDedicated
)

// modelSelector identifies one selector token in an argv vector and how to
// rewrite that exact token without changing the surrounding arguments.
type modelSelector struct {
	model             string
	argIndex          int
	replacementPrefix string
	kind              modelSelectorKind
}

func observeModelDispatch(deps Deps, observation modelDispatch) {
	if deps.onModelDispatch != nil {
		deps.onModelDispatch(observation)
	}
}

func defaultModelDispatch() modelDispatch {
	return modelDispatch{source: modelDispatchCLIDefault}
}

// modelDispatchFromArgs applies the provider's selector precedence. args must
// be the flag-only portion assembled by a driver; keeping prompts and artifact
// paths out prevents their text from impersonating a model flag.
func modelDispatchFromArgs(cli string, base modelDispatch, args []string) modelDispatch {
	return scanModelDispatch(cli, base, args, false)
}

// modelDispatchFromExtraArgs accepts only an unambiguous sequence of selector
// overrides. An unknown pass-through option may consume later text according
// to provider-specific grammar, so retaining any inferred selector would risk
// persisting an option value or prompt fragment as a model identity.
func modelDispatchFromExtraArgs(cli string, base modelDispatch, args []string) modelDispatch {
	return scanModelDispatch(cli, base, args, true)
}

// modelDispatchFromFinalizedFlags derives selector evidence from the exact
// emitted flags after order-preserving deduplication. trusted must remain an
// exact prefix; otherwise a malformed flag/value pair crossed the provenance
// boundary and the selector is conservatively unknown.
func modelDispatchFromFinalizedFlags(cli string, trusted, emitted []string) modelDispatch {
	if len(trusted) > len(emitted) || !slices.Equal(trusted, emitted[:len(trusted)]) {
		return modelDispatch{source: modelDispatchUnknown, uncertain: true}
	}
	dispatched := scanModelDispatch(cli, modelDispatch{}, trusted, false)
	return scanModelDispatch(cli, dispatched, emitted[len(trusted):], true)
}

func scanModelDispatch(cli string, base modelDispatch, args []string, rejectUnknown bool) modelDispatch {
	if base.uncertain || base.terminated {
		return base
	}
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			base.terminated = true
			return base
		}
		selector, consumed, recognized := modelSelectorAt(cli, args, i)
		if recognized {
			i += consumed
			if isCodexCLI(cli) && selector.kind == modelSelectorDedicated && selector.model == "" {
				return modelDispatch{source: modelDispatchUnknown, uncertain: true}
			}
			if repeatedCodexDedicatedSelector(cli, base, selector.kind) {
				return modelDispatch{source: modelDispatchUnknown, uncertain: true}
			}
			base = applyModelSelector(base, selector)
			continue
		}
		if rejectUnknown {
			return modelDispatch{source: modelDispatchUnknown, uncertain: true}
		}
	}
	return base
}

// effectiveCodexModelSelector returns the selector Codex resolves before `--`.
// Dedicated -m/--model flags take precedence over generic config overrides,
// regardless of their relative argv positions. Repeated config overrides are
// ordered, while repeated dedicated flags are a provider argument conflict and
// have no effective selector.
func effectiveCodexModelSelector(args []string) (modelSelector, bool) {
	var effective modelSelector
	dedicatedFound := false
	found := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		selector, consumed, recognized := modelSelectorAt("codex", args, i)
		if !recognized {
			continue
		}
		i += consumed
		if selector.kind == modelSelectorDedicated {
			if dedicatedFound {
				return modelSelector{}, false
			}
			dedicatedFound = true
		}
		if selector.model != "" && selector.kind >= effective.kind {
			effective, found = selector, true
		}
	}
	return effective, found
}

// modelSelectorAt recognizes every concrete selector spelling supported by the
// named CLI. Codex config overrides are deliberately provider-specific: -c is
// an unrelated flag on other CLIs. recognized also covers non-model Codex
// config values so a strict raw scan consumes their value rather than treating
// it as a separate option.
func modelSelectorAt(cli string, args []string, index int) (selector modelSelector, consumed int, recognized bool) {
	arg := args[index]
	switch {
	case arg == "--model" || arg == "-m":
		if index+1 >= len(args) {
			return modelSelector{}, 0, false
		}
		return modelSelector{model: args[index+1], argIndex: index + 1, kind: modelSelectorDedicated}, 1, true
	case strings.HasPrefix(arg, "--model="):
		return modelSelector{
			model: strings.TrimPrefix(arg, "--model="), argIndex: index,
			replacementPrefix: "--model=", kind: modelSelectorDedicated,
		}, 0, true
	case strings.HasPrefix(arg, "-m="):
		return modelSelector{
			model: strings.TrimPrefix(arg, "-m="), argIndex: index,
			replacementPrefix: "-m=", kind: modelSelectorDedicated,
		}, 0, true
	case isCodexCLI(cli) && (arg == "-c" || arg == "--config"):
		if index+1 >= len(args) {
			return modelSelector{}, 0, false
		}
		value, ok := configModelValue(args[index+1])
		if !ok {
			return modelSelector{}, 1, true
		}
		return modelSelector{
			model: value, argIndex: index + 1,
			replacementPrefix: "model=", kind: modelSelectorConfig,
		}, 1, true
	case isCodexCLI(cli) && (strings.HasPrefix(arg, "-c=") || strings.HasPrefix(arg, "--config=")):
		flag, config, _ := strings.Cut(arg, "=")
		value, ok := configModelValue(config)
		if !ok {
			return modelSelector{}, 0, true
		}
		return modelSelector{
			model: value, argIndex: index,
			replacementPrefix: flag + "=model=", kind: modelSelectorConfig,
		}, 0, true
	default:
		return modelSelector{}, 0, false
	}
}

func isCodexCLI(cli string) bool {
	return cli == "codex" || cli == "codex-tmux"
}

func repeatedCodexDedicatedSelector(cli string, base modelDispatch, next modelSelectorKind) bool {
	return isCodexCLI(cli) && next == modelSelectorDedicated && effectiveSelectorKind(base) == modelSelectorDedicated
}

func applyModelSelector(base modelDispatch, selector modelSelector) modelDispatch {
	if selector.model == "" || selector.kind < effectiveSelectorKind(base) {
		return base
	}
	return modelDispatch{
		model: selector.model, source: modelDispatchArgv, selectorKind: selector.kind,
	}
}

func effectiveSelectorKind(dispatch modelDispatch) modelSelectorKind {
	if dispatch.selectorKind != modelSelectorNone {
		return dispatch.selectorKind
	}
	if dispatch.model != "" && dispatch.source == modelDispatchArgv {
		return modelSelectorDedicated
	}
	return modelSelectorNone
}

func configModelValue(config string) (string, bool) {
	key, value, ok := strings.Cut(config, "=")
	if !ok || strings.TrimSpace(key) != "model" || strings.TrimSpace(value) == "" {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(value), `"'`), true
}

func applyModelDispatch(cli string, base, effect modelDispatch) modelDispatch {
	if base.uncertain || base.terminated {
		return base
	}
	if effect.uncertain {
		return modelDispatch{source: modelDispatchUnknown, uncertain: true}
	}
	if repeatedCodexDedicatedSelector(cli, base, effectiveSelectorKind(effect)) {
		return modelDispatch{source: modelDispatchUnknown, uncertain: true}
	}
	if effect.model != "" && effectiveSelectorKind(effect) >= effectiveSelectorKind(base) {
		base.model = effect.model
		base.source = effect.source
		base.selectorKind = effectiveSelectorKind(effect)
	}
	if effect.terminated {
		base.terminated = true
	}
	return base
}

func modelDispatchFromRealization(cli string, base modelDispatch, realization Realization) modelDispatch {
	return applyModelDispatch(cli, base, realization.modelDispatchEffect)
}

func modelDispatchForTmux(cli string, realization Realization, extras []string) modelDispatch {
	observation := modelDispatchFromRealization(cli, defaultModelDispatch(), realization)
	return modelDispatchFromExtraArgs(cli, observation, extras)
}

// modelDispatchFromREPL recognizes selector commands at the boundary where a
// seed line has actually been sent. Keeping this separate from launch argv
// prevents a boot failure from claiming a model command that never reached the
// session.
func modelDispatchFromREPL(line string) (modelDispatch, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "/model" {
		return modelDispatch{}, false
	}
	return modelDispatch{model: strings.Join(fields[1:], " "), source: modelDispatchREPL}, true
}
