# Build Explanation — Cycle 2243

## Build Binding
- Cycle: 2243
- Base SHA: fe9550c223267e2317b1e5b14b9f11b2a20ae788

## Summary
The advisor now sets the tier of the build phase from risk classes that the phase registry names. A build that matches a class runs on the deep tier. A build that matches no class runs on the balanced tier.

## Rationale
The plan prompt told the advisor that every build is a deep phase. About half of all builds ran deep and used the scarce Claude pools. The classes are data in the `config.deep_build_risk` block of the registry. An operator changes a class without a code change. The kernel applies the rule after it decodes the advisor answer. The kernel, not the model, must decide the build tier.

## Changed Areas
- `docs/architecture/phase-registry.json` — adds the `deep_build_risk` block with four classes: protected-surface, concurrency, security and kernel.
- `go/internal/config/deep_build_risk.go` — normalizes the markers of each class (trim, lower case, drop blanks) and stores them in the routing config.
- `go/internal/config/registry.go` — reads the `deep_build_risk` key of the registry and calls the normalizer.
- `go/internal/config/routing_config.go` — adds the `DeepBuildRisk` field; the `omitempty` tag keeps the frozen config goldens unchanged.
- `go/internal/core/advisor/deep_build_risk.go` — holds the rule: collect the evidence, match the markers, set the tier of the running build entry.
- `go/internal/core/advisor/advisor.go` — applies the rule to each plan and replan after decoding.
- `go/internal/core/advisor/parse.go` — applies the same rule when a captured response is replayed.
- `go/internal/core/advisor/prompt.go` — names the policy classes in the plan prompt and drops the blanket deep-build line when classes exist.
- `go/internal/router/router.go` — adds `Files` to `LaneItem` so the rule can match the declared files of the inbox item.
- `go/internal/core/advisor_lane_items.go` — fills `LaneItem.Files` from the inbox item.
- `go/internal/core/advisor/deep_build_risk_test.go` — covers every branch of the rule, the replay path and the prompt line.
- `go/internal/config/deep_build_risk_test.go` — covers the normalization and the absent block.

## Design Decisions
The evidence is the resolved lane items (acceptance and files). The goal text is the fallback only when no lane item resolves, so a general goal cannot raise every lane to deep. With no classes, or with no evidence, the proposal stays unchanged. A rejected option was a Go list of classes; it fails the criterion that the policy holds the classes.

## Verification
The acceptance predicates `TestC2243_001` to `TestC2243_008` and the unit tests of both packages run with `go test -count=1`. The coverage report of both rule files shows 100 percent for each function.

## Compatibility
A registry without the block keeps the previous behavior and the previous prompt line byte for byte. The config goldens do not change.

## Limitations
Matching is by substring on lower-case text, so a marker can match an unrelated word. The operator tunes the markers in the registry.
