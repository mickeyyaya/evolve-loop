# Plan: A seat cannot end the login of the operator

- **Status:** decided. The operator answered "C" on 2026-11-05 (Q2), and said "follow the research best option and start to build the code" on 2026-11-06 (Q1). The decision record and the spec are written. The first stage protocol failed four reviews, and the operator's rule "once we failed to 4th review, it should trigger the redesign" replaced it. The redesign is the token file as the stage.
- **Decision:** [ADR-0163](../architecture/adr/0163-a-seat-holds-a-narrow-token-in-its-own-directory-and-the-token-file-is-the-stage.md). **Design:** [seat-login](../architecture/seat-login.md).
- **Research:** [claude-login-split-2026-11](../research/claude-login-split-2026-11.md).
- **Related:** [llm-isolation](../architecture/llm-isolation.md), [operating-policy](../operations/operating-policy.md).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)

## 1. Request

The operator, 2026-11-05: "could you investigate why claude code continue to logout after a certain period of time? e.g. 12 or 24 hours I need to re-login". Then: "Could you investigate logout issue first and research to generate plan?"

## 2. Facts

| Fact | Research id |
|---|---|
| 7 failure clusters since 11-02, with gaps of 8.5 to 24 hours | F2.1 |
| 4 older clusters in the console, with gaps of 19 to 27 hours, and their cause is open | F2.4 |
| Most clusters start with `OAuth token revoked` in a seat, then spread | F2.2, F2.3 |
| A refused Keychain write moves the login to `~/.claude/.credentials.json` | F3.1 |
| A sandboxed seat cannot write the Keychain, and it can write under `~/.claude` | F3.2, F3.3 |
| The two stores differ in time today | F3.4 |
| Each seat shares one stored login | F3.5 |
| The manifest already passes `CLAUDE_CONFIG_DIR` and `CLAUDE_CODE_OAUTH_TOKEN` | F3.6 |
| A process with an environment token still renews the stored login | F4.2 |
| No CLI version through 2.1.318 states a fix | F4.6 |
| The rise on 11-02 has two possible causes, not separated | F5.3 |

## 3. Goals and non-goals

**Goals**

| Goal | Measure | Now | Target |
|---|---|---|---|
| G1. No forced login caused by a seat | Failure clusters that start in a seat | 7 in 4 days (F2.1) | 0 |
| G2. A seat cannot touch the stored login | Writes to the Keychain item or `~/.claude/.credentials.json` from a seat | possible (F3.2) | Refused, by a contract test |
| G3. No auth error in a seat | Auth errors for each wave | 264 in 4 days (F2.1) | 0 |
| G4. The design holds on each OS | One source of the credential on macOS and Linux | none | one |
| G5. A logout reads as a logout | The signal names the login and the next step | quota (B10) | exact text |

**Non-goals**

- This plan does not fix the upstream bug.
- This plan does not change how the operator logs in to the console.
- This plan does not explain the rise on 11-02 (F5.3) or the console clusters (F2.4).
- This plan does not promise that each logout ends. If the console clusters have another cause, the operator still logs in at that rate. L0 and L8 measure this.

## 4. Candidate designs

| Design | Summary |
|---|---|
| A. Token in the environment | Each seat gets `CLAUDE_CODE_OAUTH_TOKEN`. The stored login stays in the shared directory. |
| B. Keychain write for cells | The cell gets a write grant for `~/Library/Keychains`. |
| C. A seat directory and a token | Each seat gets `CLAUDE_CONFIG_DIR`, set to a pipeline directory with no stored login, and the token. |
| D. A credential gateway | A host process holds the login, and seats reach the API through `ANTHROPIC_BASE_URL`. |
| E. Fewer seats, and wait | One or two lanes, until an upstream fix. |

Each criterion scores 1 (poor) to 5 (good). The first criterion counts twice, because it is the fault.

| Criterion | A | B | C | D | E |
|---|---:|---:|---:|---:|---:|
| Removes the path from a seat to the stored login (×2) | 1 (2) | 2 (4) | 5 (10) | 5 (10) | 1 (2) |
| Works if the cause is not the split (F2.4, F5.3) | 2 | 1 | 3 | 3 | 2 |
| Holds on each OS (operator decision of 2026-11-03) | 5 | 1 | 5 | 5 | 5 |
| Cost to build | 5 | 5 | 3 | 1 | 5 |
| Less exposure of a credential to a cell | 2 | 1 | 3 | 5 | 2 |
| Keeps the throughput of the wave | 5 | 5 | 5 | 4 | 1 |
| **Total (of 35)** | **21** | **17** | **29** | **28** | **17** |

Basis of the scores:

- **A:** F4.2 reports that this process still renews the stored login near its expiry. The console recommended A first, and this report refuted that recommendation.
- **B:** the split goes, but each seat still rotates one shared login. A cell also gets a write path to the whole login keychain. The operator decision of 2026-11-03 rules out a macOS-only Keychain design.
- **C:** a seat directory has no stored login, so a renewal has nothing to act on (synthesis, F4.5). The token does not refresh. L2 must prove this. The cell holds a credential, which the operator decision of 2026-11-03 forbids as written (Q2).
- **D:** this is the only design that meets the operator decision of 2026-11-03 as written, because a cell holds no credential. It adds a service that each seat depends on. It needs the same empty seat directory as C, because F4.2 reports a renewal with `ANTHROPIC_BASE_URL` too.
- **E:** this costs the throughput that the wave exists for, and it does not remove the fault.

**The operator chose C (Q2).** A, B and E fail on the first criterion or on throughput. C leads D by 1 point of 35, and D leads if the exposure row counts twice, so the score does not decide. C costs less, and D meets the operator decision as written. Our proposal: C now, if the operator accepts a narrow token in the cell. Otherwise D.

**What changes our mind:** L0 can show that the file does not move at a failure, which refutes the split. A live proof that a seat with an empty directory still changes the stored login also rules out C and D as written.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| P1 | A seat never uses the stored login of the host. | F3.5, F4.2 |
| P2 | Each `claude` seat runs with `CLAUDE_CONFIG_DIR` set to a pipeline directory that holds no login. | F3.6, F4.5 |
| P3 | Proposal, open until Q2: a seat authenticates with one token from `claude setup-token`, or through the gateway of D. | F4.3, F4.5 |
| P4 | The cell cannot write the stored login on each OS. It cannot read the file. L3 tests the Keychain read on macOS and states the result. | F3.2, F3.3 |
| P5 | The console and the usage probe keep the stored login. They run outside the cell, so a Keychain write works. | F3.1 |
| P6 | The manifest holds this as data. One parameter seat runs first, and the holder file moves the fleet at a boundary. | F3.6 |

## 6. Components

The owner `console-manual` means that the console builds it, because each component touches the bridge or the sandbox.

| Id | Component | Red test first | Owner |
|---|---|---|---|
| L0 | Measure. Q3 is done (F3.7). A read-only verb reports the store times across the next failure, so no console reads a credential. | The verb prints the `mdat` and the file time from a fixture, and it prints no token. | operator, console-manual |
| L1 | Seat directory. The bridge creates it with the settings, plugins and trust that a seat needs, and no credential. | A seat started through the real launch path has `CLAUDE_CONFIG_DIR` set, and the directory has no `.credentials.json`. | console-manual |
| L2 | Prove the isolation. A live seat with a token and an empty directory runs across a stored-login expiry. | The Keychain `mdat` and the file time do not change because of that seat. A stub test covers the launch shape. | console-manual |
| L3 | Sandbox rule. Deny read and write of `~/.claude/.credentials.json` for a cell, on each OS. Test if a cell can read the secret of the Keychain item, and deny it if the profile can. | A contract row: a cell gets `Operation not permitted` on the file. A mutant that removes the deny fails the row. A row states the Keychain result, as a measured limit if no deny exists. | console-manual |
| L4 | Token holder. `seatlogin.Resolve` reads the holder file once for each launch, and `evolve auth seat-token set` and `clear` write it. | A bad file refuses the launch with a named failure and the step `claude setup-token`. An absent file is the stored login, and this is a named limit. No token value is in a log, an argv or a signal. | console-manual |
| L5 | Signal (B10). A login failure gets its own class, and never reads as quota. | The real text of `Login expired` and `OAuth token revoked` gives the class. A real quota wall keeps its class. Equality tests hold each text. | console-manual |
| L6 | Readers. The usage probe, the transcript reader, gc and the post-mortem read the seat directory. | A seat in the new directory gives the same spend numbers and the same post-mortem as before. | console-manual |
| L7 | Stage. One parameter seat first, with `--seat-token-file`. Then the holder file at a boundary, through `evolve auth seat-token set`. | A parameter seat gets the isolated shape and leaves the holder untouched. A bad holder refuses an isolated launch. | console-manual |
| L8 | Count by place. A read-only verb counts auth errors by place: console or seat. | The verb gives the counts of F2.3 from a fixture. | console-manual |

### The test rows

Each component has six rows: red proof, journey, preservation, wiring, mutation and signal. A row that does not apply states why. L2 is the journey row for the whole plan.

## 7. Order

The order is the build split of the design card. Each pull request is safe alone, and none arms a host that has no token. The [spec](../architecture/seat-login.md) holds the detail.

1. **L5, as a precondition.** It makes each later failure read true. Its pattern gives the class of a refused token.
2. **The denies and `paths`.** The holder deny is unconditional, and no token exists yet, so no live seat changes.
3. **The readers (L6).** An empty extra root changes nothing.
4. **`Resolve`, the seat directory, the token join and the parameter.** The holder is not read yet. The source scan of the seam lands here.
5. **L2 on parameter seats.** Order of measurement: L3 measurement, a minted token, a bad-token fixture, a shadow seat, the grant list, then L1.
6. **The holder read at launch and the verbs, in one pull request.** No verb can then arm a binary that lacks the rest.
7. **`seat-token set` at a boundary.** This moves the fleet. `seat-token clear` is the rollback.

Alongside: **L0** records which store moves first at the next failure. **L8** counts auth errors by place before and after step 7. The design card and its architecture review came before step 1.

## 8. Verification

| Goal | Before (2026-11-05) | After | How |
|---|---|---|---|
| G1 | 7 clusters in 4 days | 0 that start in a seat, in 7 days of waves | L8. A console cluster with no seat before it is not counted, and it gets its own research. |
| G2 | The write works from a cell | Refused | The contract rows of L3 |
| G3 | 264 auth errors in 4 days | 0 in seats, across a wave of more than 24 hours | L8 |
| G4 | No per-OS source | One file on macOS and Linux | The contract row on darwin, and a CI job with `bubblewrap` on Linux, with a stub binary. A real turn on Linux is a stated limit |
| G5 | `quota-pause` at 56 percent usage | The class of a login, and `/login` | The journey test of L5 |

**What a test cannot prove.** A stub cannot prove what the real CLI does near an expiry. So L2 is a live run, and the sweep of G3 is the final proof. The behaviour of the CLI can change at each update, so L2 also runs after each boundary update of the CLI.

## 9. Risks

| Risk | Control |
|---|---|
| A new directory drops a setting or a plugin, and each seat fails at one time, as on 11-02 | L1 starts from the real launch shape. L7 runs one parameter seat first, then the holder at a boundary. A bad holder refuses an isolated launch, and a lost holder puts later seats on the stored login (a named limit of the spec). |
| The token is inference-only, so a seat loses a feature | The authentication page names Remote Control and claude.ai connectors as lost. MCP servers configured locally still work. L2 runs a real seat. |
| A cell can read the token from its environment | The token can only make model requests, and it replaces the wider read of the stored login (L3). Q2 asks the operator to accept this. |
| The token leaks in a log | L4 tests that no value appears in a log, an argv or a signal. The existing rule is no cell env value in any argv. |
| A one-year token is lost or expires | A lost file puts each later seat on the stored login, and the launch record shows it. An expired token reads as a login failure with the next step (L5). The CLI warns before the end, per the changelog of 2.1.318. |
| The fault is something else (F2.4, F5.3) | L0 tests the split first, before any build. L8 shows the logouts that remain after seats move. |

## 10. Open questions

| Id | Question | Our proposal |
|---|---|---|
| Q1 | Is a `setup-token` token right for unattended lanes on this subscription? | Decided on 2026-11-06. The operator said: "follow the research best option and start to build the code". Research F7 names the token on the operator's own subscription as the documented path that fits best. Limit: no page names a user's own loop or a fleet of seats (F7.8), and only Anthropic can close that gap. An API key stays the one path with no doubt in the terms (F7.6). |
| Q2 | On 2026-11-03 the operator told the console that no cell gets a credential, and that the pipeline holds it behind one source for each OS. The repo has no record of this. C breaks this as written, and D does not. Is a narrow token in the cell an accepted exception? | Decided on 2026-11-05. The operator answered "C". So a narrow token in the cell is an accepted exception to that rule. |
| Q3 | Do the stores hold different logins? | Answered on 2026-11-05 (F3.7): the Keychain holds a token, and the file holds an empty one. |
| Q4 | Why the rise on 11-02, and what caused the console clusters of 10-17 to 10-20? | Open. The expiry in L0 is the first evidence. |
