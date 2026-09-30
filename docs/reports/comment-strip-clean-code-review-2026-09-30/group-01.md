## go/internal/envchain/envchain.go
verdict: MINOR
readability: 4
removed: REDUNDANT=2 RECOVERABLE=2 INVARIANT=1 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 9-20, 22-30 | G5, ch4-RECOVERABLE | The tier order (request env > process env > profile > default) was spelled out only in the removed package doc. The code shows it as three if-blocks, and `ResolveNoOS` repeats the reqEnv and profile tiers. | Body of `Resolve` becomes `return cmp.Or(reqEnv[key], os.Getenv(key), profile, def)` and `ResolveNoOS` becomes `return cmp.Or(reqEnv[key], profile, def)`. `cmp.Or` is already used in commentaudit/cli.go and evalgate/floorbinding.go, and a nil-map read yields "". The precedence then reads left to right on one line. |
| 9, 22 | N1, F (4 args) | `profile` holds a resolved value, not a profile; `def` is an abbreviation. | Rename params `profile`→`profileValue`, `def`→`fallback`. |
| 22 | N4/G20, ch4-RECOVERABLE | `ResolveNoOS` reads as "no operating system". The usage rule was lost: it is for keys whose profile is the source of truth and where a process-env override is unwanted. | Rename `ResolveNoOS`→`ResolveIgnoringProcessEnv` (2 non-test callers). |
| 10, 23 | ch4-INVARIANT | Lost: an explicit empty reqEnv value counts as absent and falls through; it never masks the lower tiers. For `Resolve` this is kept by `TestResolve_PrecedenceOrder/empty-req-env-falls-through-to-process`. Nothing pins it for `ResolveNoOS`. | Add `TestResolveNoOS_EmptyReqEnvFallsThroughToProfile`. |
| 1 | ch4-DESIGN | Lost: envchain is the single source that keeps the runner's permission resolution and the bridge's policy resolution aligned (Chain of Responsibility). Also lost: where each tier comes from (orchestrator-set reqEnv, operator shell, `.evolve/profiles/<phase>.json`), and that `PhaseEnvKey` exists to stop drift across PERMISSION_MODE/MODEL/PLAN_INPUT/PLAN_OUTPUT. | Record in `docs/architecture/packages/internal-envchain.md` (file does not exist yet). |
| 32-35 | ch4-REDUNDANT | The `PhaseEnvKey` examples are gone, but the code shows the transform and `TestPhaseEnvKey_CanonicalForm` pins the examples. | none |

## go/internal/envchain/typed.go
verdict: MINOR
readability: 4
removed: REDUNDANT=2 RECOVERABLE=3 INVARIANT=2 DESIGN=1 HISTORY=2
| line | smell | finding | refactor |
|---|---|---|---|
| 9, 29 | G16/G19, ch4-RECOVERABLE | `Resolve(key, reqEnv, "", "")` passes two bare empty strings. Lost: the typed getters use only the env tiers on purpose, because the typed `def` argument is the fallback. | Extract `func resolveEnvTiersOnly(key string, reqEnv map[string]string) string { return cmp.Or(reqEnv[key], os.Getenv(key)) }` and call it from `Int` and `Bool`. |
| 20 | G20/N4, ch4-RECOVERABLE | `IntMin` reads as "clamp to min", but a value below min returns `def`, not `min`. The param `min` also shadows the Go 1.21 builtin. | Rename `IntMin`→`IntAtLeastOrDefault` and param `min`→`minValid`. The behaviour is already pinned by `TestIntMin/zero-below-min-returns-default`. |
| 20-26 | ch4-INVARIANT | Lost: when def < min, the below-min fallback returns def unchanged, by design. No test covers def < min. | Add `TestIntMin_DefaultBelowMinIsReturnedUnchanged`. |
| 8-18 | ch4-INVARIANT | Lost: `Int` mirrors raw `strconv.Atoi` with no whitespace trimming, so migrating a call site keeps its behaviour. Kept by `TestInt/leading-space-is-unparseable`. | none (test exists) |
| 28, 32 | N4/G20, ch4-RECOVERABLE | The names `Bool` and `BoolValue` do not say which one reads the live process env. Lost: frozen per-cycle snapshots must use `BoolValue` so they never fall back to os.Getenv. | Rename `BoolValue`→`ParseBool`. It only parses, and its signature takes no key, so it cannot look anything up. |
| 28 | ch4-DESIGN | Lost: how each flag style maps onto the helper: default-off enable flags use `Bool(k, env, false)`, default-on flags use `Bool(k, env, true)`, and inverse `*_DISABLE` flags pass their own default. | Add a "typed getters" section to `docs/architecture/packages/internal-envchain.md`. |
| — | ch4-HISTORY | Lost: the story of per-site strconv boilerplate drifting before typed.go, and the note that migrating legacy "1"/"0" call sites is behaviour-preserving. | none |

## go/internal/ciparity/graduation_prescription.go
verdict: MINOR
readability: 4
removed: REDUNDANT=0 RECOVERABLE=2 INVARIANT=0 DESIGN=1 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 13 | G28/G5, ch4-RECOVERABLE | `strings.Contains(pkg, "...")` lost its reason: a recursive pattern names no single directory, so no test-file path can be prescribed. The same predicate appears again at ciparity.go:48. | Extract `func isRecursivePattern(enforceEntry string) bool` in ciparity.go and use it at both sites. Pinned by `TestGraduationPrescription_PatternSuffixSkipsBogusPath`. |
| 8 | N1, ch4-RECOVERABLE | `pkgs` is vague. Lost: the input is the output of `NewUngraduatedPackages`, in `./internal/<pkg>` enforce-list form, so the prescription can only name packages the detector flagged. | Rename param `pkgs`→`ungraduatedEnforceEntries`. |
| 14, 18 | G5 | The obligation tail "in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)" appears verbatim in both branches. | `const realAssertionObligation = "in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)"`. |
| 12, 17 | G25/G19 | The literal `go/.apicover-enforce` and the mapping `"go/" + strings.TrimPrefix(pkg, "./")` have no names. The same TrimPrefix appears at ciparity.go:51. | `const apicoverEnforcePath = "go/.apicover-enforce"`; extract `func repoDirOf(enforceEntry string) string`. |
| — | ch4-DESIGN | Lost: why the helper is exported from ciparity. The build seam (core/phase_bindings_graduation.go) and the audit seam (phases/audit/ciparitygate/graduation.go) render the same fix, and both already import ciparity for `NewUngraduatedPackages`. | `docs/architecture/packages/internal-ciparity.md` (file does not exist yet). |
| — | ch4-HISTORY | Lost: the cycle-1329 relocation and the "byte-identical move" note. | none |

## go/internal/panetrust/panetrust.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=2 RECOVERABLE=6 INVARIANT=4 DESIGN=4 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 9 | N1/G16, ch4-RECOVERABLE | `ansiRE` is an opaque two-part regex; lost: it matches CSI and OSC sequences. It is a verbatim copy of bridge/tmux.go:245, kept on purpose because of the leaf rule. | `const csiSequence = "\x1b\\[[0-9;]*[a-zA-Z]"`, `const oscSequence = "\x1b\\][^\x07]*\x07"`, `var ansiEscapeRE = regexp.MustCompile(csiSequence + "\|" + oscSequence)`. |
| 13-16, 48-50 | G16 (mental mapping), ch4-RECOVERABLE | `[...][2]string` is read through `mb[0]`/`mb[1]`. Lost: which strict parser each marker defeats (the phasecontract verdict sentinel `<!-- evolve-verdict: … -->` and the ADR-0037 channel breadcrumb). | `const verdictSentinelMarker = "evolve-verdict:"`, `const channelBreadcrumbKey = "\"evolve_channel\""`. Replace the array and loop with `var houseMarkerDefanger = strings.NewReplacer(verdictSentinelMarker, "evolve-verdict"+defangTag+":", channelBreadcrumbKey, "\"evolve_channel"+defangTag+"\"")` behind `func defangHouseMarkers(s string) string`. |
| 46-50 | G31, ch4-INVARIANT | The strip → redact → defang order matters but is now implicit. Defanging after the ANSI strip is what stops an escape-split marker from reassembling. Kept by `TestDigest_CapsStripsNeutralizes/ansi_split_marker_still_neutralized`. | Extract `func neutralize(pane string) string { return defangHouseMarkers(redactSecrets(stripANSI(pane))) }` so the whole order is one expression. |
| 20-29 | N1/G25, ch4-RECOVERABLE | Eight anonymous regexes. Their inline labels are gone: OpenAI/Anthropic key, AWS access key id, GitHub token, GitHub fine-grained PAT, Slack token, Slack app token, JWT, PEM private-key header. | Named table `var secretPatterns = []struct{ kind string; re *regexp.Regexp }{{"aws-access-key-id", …}, {"jwt", …}, …}`, or one named var per pattern (`awsAccessKeyIDRE`, `jwtRE`, …). |
| 20-29 | ch4-INVARIANT (gap) | Only 5 of the 8 families are planted in `TestDigest_PlantedSecretSentinelNeverSurvives`. `github_pat_`, `xapp-` and JWT have no test, so deleting any of those patterns would go unnoticed. | Add `TestDigest_RedactsEverySecretPatternKind`: a table test over `secretPatterns` with one planted sample per kind. |
| 31 | G16, ch4-INVARIANT | The 12-way alternation is hard to read. Lost: compound keys (access_token, private_key, …) are listed before bare `token`/`secret` so the longer name is captured (the S6 gap). Kept by `TestDigest_CompoundCredentialKeyNamesRedacted`. | Build it from `var credentialKeyNames = []string{"access[_-]?token", …}` with `strings.Join(…, "\|")`. Rename `kvSecretRE`→`credentialKeyValueRE`. |
| 33-40 | G16, ch4-DESIGN | `RedactSecrets` is a one-line exported wrapper around `redactSecrets`, and without its doc the indirection looks pointless. Lost: it serves the advisor's raw capture (ADR-0052 WS3-S1). That capture must not defang or truncate, or replay breaks, and it accepts the risk that a secret split by an ANSI escape slips through. The private name is pinned by a string search in core/seams_test.go:25. | Write the capture contract and the accepted risk into `docs/architecture/packages/internal-panetrust.md`. Point core/seams_test.go at `RedactSecrets` and fold `redactSecrets` into it. |
| 40 | ch4-INVARIANT (gap) | No test asserts that `RedactSecrets` leaves house markers and line count intact, which is the replay contract. | Add `TestRedactSecrets_LeavesMarkersAndLengthIntact`. |
| 42-61 | F1/G34, ch4-RECOVERABLE | `Digest` does five things (strip, redact, defang, tail cap, column cap) at mixed levels of abstraction. Lost: the line cap keeps the tail (recent lines win), and maxCols <= 0 means no column cap. That second rule is untested. | `return strings.Join(capColumns(tailLines(neutralize(pane), maxLines), maxCols), "\n")` with `func tailLines(s string, n int) []string` and `func capColumns(lines []string, maxRunes int) []string`. Add `TestDigest_NonPositiveMaxColsMeansNoCap`. |
| 63-72 | G16/N, ch4-RECOVERABLE | Now that the "byte length bounds rune count" fast-path note is gone, the double length check looks redundant. The param `max` shadows the builtin. | Rename `max`→`maxRunes` and replace both checks with `if utf8.RuneCountInString(s) <= maxRunes { return s }`. |
| 1 | ch4-INVARIANT (gap) | Lost the leaf rule: the package imports only the standard library, so bridge, core and interaction can depend on it without import cycles. | Add `TestPanetrust_ImportsStdlibOnly`, which parses the package's non-test files with go/parser. |
| 1, 11, 18 | ch4-DESIGN | Lost the threat model. Pane text is attacker-influenceable (OWASP LLM: segregate untrusted content), so every pane→prompt, decision or ledger path MUST go through this package. The defang tag keeps the text readable for humans while strict parsers reject it. Digests persist and may reach a fallback CLI from a different vendor (S6). Digest never joins pane text with env or templates (S1/S10). | `docs/architecture/packages/internal-panetrust.md` (create it), recording threats S1/S6/S10. |
| — | ch4-HISTORY | Lost: the "Slice 1 shipped with I1; I5-full later" rollout story. | none |

## go/internal/bridge/launchintent.go
verdict: NEEDS-REFACTOR
readability: 2
removed: REDUNDANT=1 RECOVERABLE=3 INVARIANT=2 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 5-7 | G25/G5, ch4-RECOVERABLE | The allowed values for `Permission` (bypass/plan/default), `SettingsScope` (project/all) and `SessionMode` ("ephemeral" / "named:<name>") were listed only in trailing comments. Raw literals are spread across files: "bypass" at realizer.go:29 and clicontrol_adapter.go:32; "ephemeral" and "named:" at launch.go:229,234 and realizer.go:80,82. | Add consts `PermissionBypass`, `PermissionPlan`, `PermissionDefault`, `SettingsScopeProject`, `SettingsScopeAll`, `SessionModeEphemeral`, plus `const sessionModeNamedPrefix = "named:"` and `func NamedSessionMode(name string) string`. Replace the literals at those sites. |
| 4 | ch4-RECOVERABLE (stale) | The removed comment said "haiku \| sonnet \| opus", but the canonical tiers are `modelcatalog.CanonicalTiers` = fast/balanced/deep/top. The old names survive only as v1 keys in manifest.go `translateV1TierKey`. The comment was already wrong. | Add `type Tier string` to modelcatalog with `CanonicalTiers []Tier`, and type the field `ModelTier modelcatalog.Tier` so the source of truth is the type. |
| 3-12 | ch4-INVARIANT (gap) | Lost: zero-value fields mean "unset" and realize to nothing. `Realize(m, LaunchIntent{})` appears in tests only to check default_args dedupe. | Add `TestRealize_ZeroValueIntentAddsOnlyManifestDefaults`. |
| 11 | N1, ch4-INVARIANT | Lost: `RawByCLI` is the per-CLI escape hatch, and a claude-only raw flag never reaches agy/codex. Kept by `TestRealizeFor_RealManifests_NoCrossCLILeak`. | Rename `RawByCLI`→`RawArgsByCLI`, since it holds argv tokens. |
| 18-19 | G9 (dead code) | `realizeSessionMode` writes `Ephemeral` and `SessionName` (realizer.go:81,83), but no production code reads them from a Realization; only realizer_test.go:64 does. The removed comments ("controller: kill the session on exit", "named/resumable session") described a consumer that does not exist. The drivers read `cfg.SessionName` instead. | Either wire the tmux controller to `Realization.Ephemeral`/`SessionName`, or delete both fields and the writes in `realizeSessionMode`. If they stay, rename `Ephemeral`→`KillSessionOnExit`. |
| 20 | N1/G20, ch4-RECOVERABLE | `ModelOmitted string` sounds like a boolean and does not say why. Lost: it holds the abstract tier token the realizer suppressed instead of a concrete model id, so drivers can log that the CLI default ran. It is read at driver_tmux_prepare.go:69, and no bridge test asserts it. | Rename `ModelOmitted`→`SuppressedTierToken`. Add `TestRealize_UnresolvedTierIsRecordedAsSuppressed`. |
| 14-23 | ch4-DESIGN | Lost the consumer protocol: launch with LaunchFlags, inject REPLInput after the boot marker, export Env in the pane or pass it to the headless process. Also lost: what `modelDispatchEffect` carries (selector, ambiguity and argv-terminator provenance from the final deduplicated flags), which drivers apply at the invocation boundary. | Add a "LaunchIntent → Realization" section to `docs/architecture/packages/internal-bridge.md`. |
| 17 | ch4-REDUNDANT | The Env comment (manifest default_env) is pinned by `TestRealize_DefaultEnvIsCopiedIntoTheRealization`. | none |

## go/internal/goalhash/goalhash.go
verdict: MINOR
readability: 4
removed: REDUNDANT=2 RECOVERABLE=2 INVARIANT=1 DESIGN=0 HISTORY=2
| line | smell | finding | refactor |
|---|---|---|---|
| 11-28 | G16/G19, ch4-RECOVERABLE | The hand-rolled rune loop with `prevSpace` lost its comment ("collapse every unicode-whitespace run to one ASCII space"), so the reader has to simulate it. | Replace the body with `return strings.Join(strings.Fields(strings.ToLower(raw)), " ")`. `strings.Fields` splits on `unicode.IsSpace` runs and drops the edges, which is the same contract. Pinned by `TestNormalize_KnownTransforms`, `TestCompute_GoldenVectors` and `TestCompute_MatchesBash`. |
| 36 | G25, ch4-RECOVERABLE | `[:8]` is a magic number. Lost: the short form is for display and batch-ID only, and state.json:currentBatch.goalHash must store the full `Compute` value. | `const shortHashLen = 8`; rename `Short`→`ShortForDisplay`. |
| 1 | ch4-INVARIANT | Lost: the hash is the goal's identity across sessions (state.json:currentBatch.goalHash). Any drift re-labels same-goal cycles as new batches, which breaks EVOLVE_INTENT_DELTA and resume. Kept by the fixed vectors in `TestCompute_GoldenVectors` and by `TestCompute_MatchesBash`. | none (tests exist) |
| — | ch4-HISTORY | Lost: the port text for bash `normalize_goal`/`sha256_of` (legacy/scripts/lifecycle/intent-batch-resolve.sh) and the trailing-newline equivalence note. | none |

## go/internal/phaseoutputs/chainstatus.go
verdict: MINOR
readability: 4
removed: REDUNDANT=3 RECOVERABLE=5 INVARIANT=4 DESIGN=1 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 3 | N1, ch4-RECOVERABLE | `ShadowView`: shadow of what? Lost: it is the one-field slice of the auditchain shadow record that this package reads. The JSON tag stays pinned by `TestShadowView_DecodesTheAuditchainWireTag`. | Rename `ShadowView`→`AuditChainShadowView`. |
| 7-10, 25 | G28/G16, ch4-RECOVERABLE | `RecordReading{View *ShadowView; Corrupt bool}` packs three read outcomes (absent, exists but unparseable, parsed) into a pointer and a bool. The check `reading.View != nil \|\| reading.Corrupt` at line 25 means "a record exists" but has no name. | Add `func (r RecordReading) RecordExists() bool { return r.View != nil \|\| r.Corrupt }` and use it at line 25. Rename `Corrupt`→`Unparseable`. |
| 18 | N4, ch4-RECOVERABLE | `ChainAbsent` and `ChainRecordMissing` read as synonyms. Only the first is the non-compliance signal (the auditor emitted no chain); the second is an instrumentation gap. | Rename the identifier `ChainAbsent`→`ChainNotEmitted` and keep the wire value "chain-absent". |
| 40 | G20, ch4-RECOVERABLE | `InDenominator`: denominator of what? Lost: it counts toward the compliance-rate denominator, meaning cycles whose audit actually ran. | Rename `InDenominator`→`CountsTowardComplianceRate`. |
| 15-20 | ch4-INVARIANT | The per-state meanings and denominator membership are gone from the const block, but they are pinned by `TestCycleChainStatus_HasExactlyOneMeaningPerState` and `TestChainStatus_DenominatorMembership`. | none (tests exist) |
| — | ch4-DESIGN | Lost: why the status is a typed enum (consumers cannot invent new meanings), why denominator membership sits next to the states (no call site re-derives it), and why ShadowView is declared locally (ISP) instead of importing auditchain. | `docs/architecture/packages/internal-phaseoutputs.md`. |
| — | ch4-HISTORY | Lost: the live flaw where "absent" was recorded both for non-compliance and for audit-not-run. | none |

## go/internal/recovery/outcome.go
verdict: NEEDS-REFACTOR
readability: 2
removed: REDUNDANT=0 RECOVERABLE=5 INVARIANT=4 DESIGN=0 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 18 | N1/G20, ch4-RECOVERABLE | `AbortReason`: lost that it is also set when a recovery path recorded a transient and continued, so a non-empty value does not mean the phase or cycle died. The name says it does, and core/signal.go:69-72 already treats any non-empty value as `KindPhaseAborted` ("aborted after verdict=…"). | Rename `AbortReason`→`InterruptionReason`, or split it into `AbortReason` + `RecoveredTransientReason`. Add `TestEmitPhaseOutcome_RecoveredTransientIsNotReportedAsAbort` to settle how signal.go reads it. |
| 12, 14-15 | N1/G16, ch4-RECOVERABLE | `DurationMS` and `StartedAt`/`EndedAt` look like one measurement. Lost: DurationMS is the runner's self-reported compute time, StartedAt/EndedAt are the orchestrator's RFC3339 wall clock, and the gap between them is a signal. | Rename `DurationMS`→`RunnerDurationMS`, `StartedAt`→`WallStartedAt`, `EndedAt`→`WallEndedAt`. These are Go-only renames: there are no JSON tags, and the sidecar and signal maps name their own keys. |
| 16 | G25, ch4-RECOVERABLE | `Archetype string` lost its vocabulary (plan/build/evaluate/control). The value is `string(phasespec.Role)` (core/orchestrator.go:110-114), and consumers compare against a raw "evaluate" (phasetiming.go:136, core/evaluate_batch.go:28). | Type the field as `Archetype phasespec.Role`, if the import graph allows, and compare against `phasespec.RoleEvaluate`. |
| 19 | G25, ch4-RECOVERABLE | `ModelSource string` lost its vocabulary ("profile"/"pin"/"advisor"). The producer writes raw literals at phases/runner/routing.go:61,64,66. | Add `type ModelSource string` with `ModelSourceProfile`, `ModelSourcePin` and `ModelSourceAdvisor` in a shared leaf such as cyclestate, and use it on both sides. |
| 21 | N1, ch4-RECOVERABLE | `Tokens` reads as the phase total. Lost: it is the terminal attempt's usage only, not a sum across attempts. | Rename `Tokens`→`TerminalAttemptTokens`. Add `TestPhaseOutcomeFrom_TokensAreTheTerminalAttemptOnly`. |
| 8-23 | ch4-INVARIANT | Lost, but all still kept by tests: one record per terminal disposition (`TestPhaseOutcome_SingleChokepoint_OneRecordPerDispatch`); Verdict is the agent's verdict or a synthesized FAIL, and an abort never rewrites it (`TestPhaseOutcome_NeverInventsPass`, plus the review_gate_reject case in `TestPhaseOutcome_AbortPaths_AlwaysRecordTimingAndUsage`); cost, duration and boot are recorded even on abort (same test); Diagnostics are relayed unfiltered as the only durable FAIL reason (`TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail`). | none (tests exist) |
