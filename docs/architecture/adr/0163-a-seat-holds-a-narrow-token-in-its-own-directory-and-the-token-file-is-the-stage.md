# ADR-0163: A seat holds a narrow token in its own directory, and the token file is the stage

- **Status:** Proposed (2026-11-06).
- **Plan:** [seat-login-isolation-2026-11](../../plans/seat-login-isolation-2026-11.md). **Research:** [claude-login-split-2026-11](../../research/claude-login-split-2026-11.md), section 7. **Design:** [seat-login](../seat-login.md).
- **Relates to:**
  - [ADR-0161](0161-a-pipeline-blocker-fix-meets-six-test-rows-and-a-signal-states-its-cause-and-next-step.md): each signal of the design meets its contract, and the test rows follow its six rows.
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): the seat is an isolated worker, and the verbs that move the stage are Denied to an agent.
  - [ADR-0156](0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md): the stage is a file and a CLI parameter, not an environment flag.
- **Evidence:**
  - Seven failure clusters since 2026-11-02 start mostly in a seat, with 264 auth errors in 4 days (research F2.1, F2.3).
  - A sandboxed seat cannot write the Keychain and can write `~/.claude`. A refused write moves the login to the file (research F3.1, F3.2, F3.3).
  - A process with an environment token still renews the stored login (research F4.2).
  - The vendor documents a token for scripts and one login for each `CLAUDE_CONFIG_DIR` (research F7.1, F7.3).
  - No page names a user's own loop of seats as allowed or refused (research F7.8). Only Anthropic can close this gap.

## Context

The operator asked on 2026-11-05: "could you investigate why claude code continue to logout after a certain period of time? e.g. 12 or 24 hours I need to re-login". The research found that each seat shares the stored login of the host. A seat can make a split between the Keychain and the file.

The operator chose design C, a seat directory and a narrow token. The answer was "C" on 2026-11-05. On 2026-11-06 the operator said: "follow the research best option and start to build the code". Research section 7 names the `setup-token` token on the operator's own subscription as the documented path that fits best.

The first stage protocol for the move failed four reviews: 2 CRITICAL and 12 HIGH, 1 CRITICAL and 9 HIGH, 6 HIGH, and 4 HIGH. The operator set the rule: "once we failed to 4th review, it should trigger the redesign". Nine of the ten HIGH findings of rounds 3 and 4 were on one mechanism, a stage that moves across processes. This record covers the redesign. The count of blocks restarted with it, and a second count past three goes to the operator.

The operator decided design C and the token. The rest of this record is a console proposal.

## Decision

1. A `claude` seat runs with `CLAUDE_CONFIG_DIR` set to a seat directory that holds no credential.
2. A seat authenticates with one `setup-token` token. The token reaches the seat only through the `0600` consume-once env file.
3. The file `<home>/.evolve-seat/holder/token` is the only stage. If it holds a token, each isolated seat uses it.
4. If the holder is absent, each seat runs as it does today. There is no stage record, no lock and no census.
5. `seatlogin.Resolve` reads the holder once for each launch, at each door, and returns a Decision: `HostIsolated`, `SeatToken`, `Dir` and `Refusal`.
6. A manifest field, `seat_login: isolated|stored`, decides which family joins the token. A family with no field reads as stored.
7. `evolve auth seat-token set` and `clear` move the stage with one atomic rename. They are Denied to an agent.
8. Each verb refuses while a loop lane is live. The gate is a courtesy, not a safety property.
9. `evolve agent run --seat-token-file` runs one seat first. The holder moves only after that seat proves the token.
10. A bad holder refuses an isolated launch with a pinned text and the exit class `seat_token_refused`. The deny on the stored login stays.
11. The cell never reads the holder directory. The deny is unconditional, in each state, for each family.
12. A token seat always runs inside a sandbox. A failed wrap refuses the launch, and `--sandbox-off` refuses a token seat.
13. Each `claude` launch records `seat_login` and its transcript root. A plane without the reader writes no field and reads as stored.

**The exception to the rule "no cell holds a credential".** On 2026-11-03 the operator told the console that no cell gets a credential. The repo has no record of this. Design C breaks the rule as written, because the cell holds the token. The operator accepted the exception by answering "C" on 2026-11-05.

The token can only make model requests (research F7.2). It replaces the wider read of the stored login.

**The policy and its gap.** Research section 7 reads the vendor terms. The unmodified binary, with the token on the operator's own subscription, is the nearest documented path (research F7.1, F7.3, F7.4). Gap F7.8 stays open: no page covers a loop of one user's own seats.

F7.7 adds that advertised limits "assume ordinary, individual usage". A fleet of seats is less clearly that than one script. This record does not close the gap.

## Consequences

**Good**
- A seat has no path to the stored login, so a seat cannot end the login of the operator.
- The move has one state, one file and one rename. A reader sees the old state or the new one.
- The rollback is one verb at a boundary.
- One seat proves the token before any fleet move.
- Each pull request is inert until the last one, so a pull request cannot arm a binary that lacks the rest.

**Bad**
- The cell holds a credential. This is the accepted exception.
- The token has no claude.ai connectors and no Remote Control (research F7.2).
- The token lasts one year, and a seat does not renew it. The operator must renew it.
- A lost holder puts each later seat on the stored login. The record and a `doctor` warning make that visible. They do not prevent it.
- The change touches the launch path, the sandbox profiles, the readers and the record. Test files that launch an isolated driver need one shared helper, up to 89 files.

**Risks**
- A seat that started before the move stays on the stored login until its process ends. The harm equals the risk of today for that time.
- A plane with a binary behind the reader still uses the stored login. G1 is not met until each plane is current.
- The fault can have another cause (research F2.4, F5.3). L0 and L8 measure what remains after the move.
- The behaviour of the CLI near an expiry can change. L2 runs after each boundary update of the CLI.

**What changes this decision**
- L2 shows that a seat with an empty directory still changes the stored login. Then design C fails, and the gateway returns to the table.
- Anthropic states that a loop of own seats is refused. Then the API key becomes the path.
- The operator renews the token more than once a year. Then a source for the token (not a file) needs its own decision.
- A fourth block past the restarted count of three goes to the operator.

## Rejected

| Option | The fact that rules it out |
|---|---|
| A stage record with a lock and a census | Four reviews failed. Nine of ten HIGH findings of rounds 3 and 4 sat on this mechanism. Each fix added a part with a new fault. The parts were a first state, a lock across a seat start, release points, a census, a rule for old binaries, and a `policy.json` exception. The harm of the race equals the risk of today, so the record buys little. |
| A credential gateway (plan design D) | The plan scores D at 28 and C at 29 of 35 (plan section 4). D adds a service that each seat depends on, and its cost scores 1. D needs the same empty seat directory, because a renewal also occurs with `ANTHROPIC_BASE_URL` (research F4.2). The operator chose C. |
| API keys | No fact in the terms rules it out. F7.6 and F7.9 call it the one path with no doubt. The operator chose the token on 2026-11-06, because research section 7 names it as the best fit on the operator's own subscription. The key is the fallback if Anthropic closes F7.8 against the token. |
| The token in the environment, with the shared directory (plan design A) | A process with an environment token still renews the stored login (research F4.2). It removes no path from the seat to the stored login. |
| A Keychain write grant for the cell (plan design B) | The cell gets a write path to the whole login keychain, and each seat still rotates one shared login. The operator decision of 2026-11-03 rules out a macOS-only design. |
| Fewer seats and wait (plan design E) | It costs the throughput of the wave, and it does not remove the fault. |
