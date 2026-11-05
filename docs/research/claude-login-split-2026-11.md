# Research: why the Claude Code login fails every 8 to 24 hours on this host

- **Status:** complete, 2026-11-05. The cause is a strong hypothesis. Section 6 states what is not proven.
- **Plan:** [seat-login-isolation-2026-11](../plans/seat-login-isolation-2026-11.md).
- **Related:** [llm-isolation](../architecture/llm-isolation.md), and the inbox item `sandboxed-seats-cannot-save-a-refreshed-login`.
- **Method:**
  - a sweep of the Claude Code transcripts for `isApiErrorMessage` lines, 21 days back, on 2026-11-05;
  - metadata of the two credential stores, never their contents;
  - one probe from a real sandboxed seat;
  - code reads at `origin/main` e69d7f45b, with `path:line`;
  - strings of the installed binary, version 2.1.318;
  - live web searches and page reads on 2026-11-05.

**Terms used throughout:**

- **Stored login:** the claude.ai login that `/login` saves. It holds an access token, a refresh token and an expiry time.
- **Seat:** a Claude Code process that the bridge starts for a lane or for the console.
- **Cell:** the sandbox of a seat (`sandbox-exec` on macOS).
- **Split:** the state where the Keychain and `~/.claude/.credentials.json` hold different logins.
- **Rotation:** a refresh that spends the old refresh token and returns a new one.

## Table of contents

- [1. Request](#1-request)
- [2. What the failure looks like](#2-what-the-failure-looks-like)
- [3. The sandbox and the credential stores](#3-the-sandbox-and-the-credential-stores)
- [4. Outside evidence](#4-outside-evidence)
- [5. The onset](#5-the-onset)
- [6. What is not proven](#6-what-is-not-proven)
- [7. Sources](#7-sources)

## 1. Request

The operator, 2026-11-05: "could you investigate why claude code continue to logout after a certain period of time? e.g. 12 or 24 hours I need to re-login". Then: "Could you investigate logout issue first and research to generate plan?"

A logout stops each lane and voids each running review. On 2026-11-05 it voided three results at one time.

## 2. What the failure looks like

The answer: the failure has no fixed period, and one event ends the login for each process on the host.

**F2.1** The sweep found 274 auth errors in 21 days. 264 of them are in seven clusters since 2026-11-02. The clusters start at these times (UTC):

| Start | First message | Gap to the next |
|---|---|---|
| 11-02 02:10 | `OAuth token revoked` | 8.5 h |
| 11-02 10:39 | `Login expired` | 23.9 h |
| 11-03 10:33 | `OAuth token revoked` | 10.1 h |
| 11-03 20:38 | `OAuth token revoked` | 11.6 h |
| 11-04 08:14 | `OAuth token revoked` | 8.8 h |
| 11-04 17:01 | `Login expired` | 14.1 h |
| 11-05 07:05 | `OAuth token revoked` | |

*Implication:* a session limit of 12 or 24 hours does not explain these gaps. An event does.

**F2.2** In five of the seven clusters, `revoked` is first. Then `Login expired` spreads over some minutes. On 11-05: `revoked` at 07:05:19 in one seat, then eight `Login expired` lines in other trees by 07:10:15.
*Implication:* one process meets the fault first, and the others meet it when each next refreshes or reads.

**F2.3** By the working directory, 173 of the 274 errors are under `worktrees/cycle` or `runs/cycle` (lanes). 88 are under `dev/cl-` (console seats and trees), and 13 are elsewhere.
*Implication:* since 11-02, seats meet the fault most. The operator sees it after them.

**F2.4** A second pattern is older. Four clusters, each `Login expired`, start at 10-17 22:18, 10-18 17:36, 10-19 20:10 and 10-20 15:05 UTC, with gaps of 19.3, 26.6 and 18.9 hours. Their working directory is the console, the runtime root or `dev/cl-handoff-docs`, not a lane.
*Implication:* these gaps are close to the "12 or 24 hours" of the request. They can have another cause, such as a limit on the life of a login, and the split does not explain them on this evidence.

## 3. The sandbox and the credential stores

The answer: a sandboxed seat can read the Keychain and cannot write it, and it can write the file. So a seat can make a split.

**F3.1** The documentation states the store: "On macOS, credentials are stored in the encrypted macOS Keychain. When the Keychain rejects the write ... Claude Code stores your login in `~/.claude/.credentials.json`" ([authentication](https://code.claude.com/docs/en/authentication)).
*Implication:* a refused Keychain write does not fail. It moves the new login to another store.

**F3.2** The probe of 2026-11-05 ran in a real seat through `evolve agent run`:

| Probe | Result |
|---|---|
| `security add-generic-password` on a dummy item | exit 161, `UNIX[Operation not permitted]` |
| `security find-generic-password` on the Claude item, metadata only | exit 0 |
| `touch` and `rm` of a file in `~/.claude` | exit 0, exit 0 |

The dummy item did not remain on the host (exit 44 from the host read).
*Implication:* this is the condition that F3.1 names, for each sandboxed seat.

**F3.3** The profile grants writes under `~/.claude` and has no grant for `~/Library/Keychains` (`go/internal/adapters/sandbox/sandbox.go:42`, `homeStateWriteDirs`).
*Implication:* the probe result follows from the design of the cell. It is not a fault of one run.

**F3.4** On 2026-11-05 the two stores differ in time. The Keychain item was last written at 07:12:37 UTC (`mdat`), which is the `/login` of the operator. `~/.claude/.credentials.json` was last written at 07:05:24 UTC, 5 seconds after the first `revoked` line, and `/login` did not change it.
*Implication:* the two stores do not move together. This is one data point. A save replaces the file, so its birth time gives no more evidence.

**F3.5** The bridge passes each seat the real `HOME` (`go/internal/bridge/hermetic_env.go:28`).
*Implication:* each seat on the host shares one stored login, so one bad refresh reaches each seat.

**F3.6** The manifest of `claude-tmux` already passes `CLAUDE_CONFIG_DIR` and `CLAUDE_CODE_OAUTH_TOKEN` (`go/internal/bridge/manifests/claude-tmux.json`, `env_passthrough`). The transcript reader already follows `CLAUDE_CONFIG_DIR` (`go/internal/tokenusage/locate.go:51`).
*Implication:* a seat login of its own needs little new plumbing.

## 4. Outside evidence

The answer: other users report the same chain, the upstream bug is open, and no version up to 2.1.318 fixes it.

**F4.1** Issue [#104359](https://github.com/anthropics/claude-code/issues/104359) (open, 2.1.307 to 2.1.310, no maintainer reply) describes a sandboxed child on macOS. A fresh `.credentials.json` appeared with renewed tokens. 14 minutes later, a Keychain renewal of another process returned HTTP 400, and Claude Code cleared the Keychain login.
*Implication:* this is the chain that section 3 predicts, reported by another user.

**F4.2** The same issue reports that a process with an environment credential still renews the stored login. By the reporter, the check before a request reads the provider and the 300,000 ms to expiry, and not the credential. A Linux comment, with `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN`, measured 8 such renewals in 3.7 days.
*Implication:* a token in the environment alone does not protect the stored login. The seat must also have no stored login to renew.

**F4.3** Issue [#94464](https://github.com/anthropics/claude-code/issues/94464) (open, 2.1.270 to 2.1.273) reports the same split from the Claude Desktop engine, with `Login expired` each day. It has a timeline and differing token fingerprints. Its verified workaround is `claude setup-token` with `CLAUDE_CODE_OAUTH_TOKEN`.
*Implication:* the token works for that reporter, in a setup with no other holder of the stored login in that process.

**F4.4** A merged change in another project states that "Anthropic's refresh token is single-use and rotating" ([hermes-agent #115690](https://github.com/NousResearch/hermes-agent/pull/115690)). The documentation does not state this.
*Implication:* rotation is a third-party statement that agrees with the logs. It is not an official fact.

**F4.5** The documentation gives this precedence: `CLAUDE_CODE_OAUTH_TOKEN` ranks above the `/login` credentials. The token lasts one year, is for "CI pipelines, scripts", and "can only make model requests". With `CLAUDE_CONFIG_DIR` set, the credential file and the Keychain entry are keyed to that directory ([authentication](https://code.claude.com/docs/en/authentication)).
*Implication:* a seat with its own directory and a token has no path to the login of the operator. This is synthesis, and the plan must prove it.

**F4.6** The [changelog](https://code.claude.com/docs/en/changelog) through 2.1.318 has fixes near this subject, such as 2.1.318 "Fixed jobs exiting with `Not logged in` when another terminal or host signs in again". No entry states a fix for the split or for the renewal of F4.2.
*Implication:* a CLI update is not a fix to wait for.

## 5. The onset

The answer: the failures rise sharply on 2026-11-02, and the sandbox alone does not explain the date.

**F5.1** The sweep has 10 errors from 10-17 to 10-20, 0 from 10-21 to 11-01, and 264 from 11-02 to 11-05.

**F5.2** Lanes ran with no auth error on 10-26 and 10-27: 81 lane transcripts, on versions 2.1.310 and 2.1.311. The Keychain rule of F3.3 was already in force. Few transcripts exist from 10-28 to 11-01, so that span proves little.
*Implication:* the split mechanism existed before the rise. Something else raised the rate, or two days were too short to meet the fault.

**F5.3** Two changes landed on 11-02, before the first cluster at 02:10 UTC. The boundary update moved the CLI to 2.1.316 at 00:07 UTC (file time of the version). PR #1231 changed the launch of each project-scoped seat to `--setting-sources project --settings` (`go/internal/bridge/manifests/claude-tmux.json:97`).
*Implication:* either change can raise the rate. This research cannot separate them.

**F5.4** Sessions on two versions run at one time. Failures come from 2.1.301 sessions and from newer seats on 11-02, 11-03 and 11-04. On 11-05 they come only from 2.1.318.
*Implication:* a long-lived console process lacks later fixes, such as the refresh fix of 2.1.317. This is a possible contributor, not a measured cause.

## 6. What is not proven

| Claim | State | What settles it |
|---|---|---|
| A sandboxed seat refreshes, and writes the file | Inferred from F3.2, F3.4, F4.1. No log shows a seat refresh. | A fingerprint comparison of the two stores, run by the operator (plan L0) |
| The access token lasts about 8 hours | Public reports only. Gaps of 8.5, 8.8 and 10.1 h agree, and others do not. | The stored expiry, read by the operator (plan L0) |
| One rotation kills the other holders | Reported in F4.1 and F4.4. Not stated by Anthropic. | The plan does not need this settled |
| The cause of the rise on 11-02 | Open (F5.3) | L0 and L2 of the plan test the split. Neither separates the CLI update from PR #1231. |
| The cause of the console clusters of 10-17 to 10-20 | Open (F2.4) | The stored expiry (plan L0), and the sweep by place after seats move (plan L8) |
| Concurrent refresh without any sandbox | Reported on Linux in F4.2. Not measured here. | The same design removes this cause too |

**A test that this host can run.** The login of 07:12 UTC on 11-05 can fail near 15:12 UTC if the token lasts 8 hours and lanes are busy. A failure then agrees with this research. No failure does not refute it, because the reports describe failures on some refreshes and not on each one.

## 7. Sources

- [Claude Code authentication](https://code.claude.com/docs/en/authentication)
- [Claude Code changelog](https://code.claude.com/docs/en/changelog), read through 2.1.301 in full, older versions in part
- [anthropics/claude-code #104359](https://github.com/anthropics/claude-code/issues/104359)
- [anthropics/claude-code #94464](https://github.com/anthropics/claude-code/issues/94464)
- [anthropics/claude-code #54443](https://github.com/anthropics/claude-code/issues/54443), closed as not planned, with a capture of `400 invalid_grant`
- [NousResearch/hermes-agent #115690](https://github.com/NousResearch/hermes-agent/pull/115690)
