# internal/cloudlaunch

> The spec of record is [llm-isolation.md](../llm-isolation.md) §5.2. The writer of a session is the `claude-cloud` driver of [internal-bridge](internal-bridge.md#the-cloud-seat-driver_claudecloudgo-manifest_cloudgo). The readers are [internal-cli-agentcmd](internal-cli-agentcmd.md), [internal-launchrecord](internal-launchrecord.md) and [internal-cli-cloudcmd](internal-cli-cloudcmd.md).

## Purpose

`internal/cloudlaunch` holds the typed outcome of one cloud session launch. It also holds the repository checks that come before a launch. The bridge writes the outcome, and the CLI reads it, so the two sides share one type and one parser.

## Design

| Symbol | Meaning |
|---|---|
| `Schema` | `evolve.cloud.launch/1`, the schema of the outcome |
| `OutcomeLaunched` | `launched`, the only outcome. The cloud session does the work later, and its deliverable is a pull request. |
| `Session` | the session id, the URL, the title, the repository (`owner/name`), the branch, the profile, the driver and the UTC time |
| `Render(s, challenge)` | the launch report: an optional `challenge-token` line, a short text and one fenced `json evolve.cloud.launch/1` block |
| `Parse(report)` | the session of a report. It refuses a report with no block, an unclosed block, bad JSON, another schema, another outcome or no session id. |
| `Repo` | the branch, its upstream remote, the URL of the remote and the count of commits that are not pushed |
| `ReadRepo(ctx, run, dir)` | reads `Repo` through `git symbolic-ref`, `git config branch.<b>.remote`, `git remote get-url` and `git rev-list --count @{upstream}..HEAD` |
| `Repo.Check(hosts)` | refuses a remote on a host that is not in `hosts`, then a branch with commits that are not pushed |
| `Refusal` | the error type of each refusal. Its `Reason` names the cause. |
| `RemoteHost(url)`, `RepoSlug(url)` | the host in lower case and the `owner/name` of an HTTPS, SSH or scp-form remote URL |

The cloud session clones the remote at the pushed branch, from a host that the manifest names (`remote_hosts`). The refusal text names those hosts from the data, never one host in prose. A local commit that is not pushed is not in the clone. Thus the check refuses the launch before a session starts, and the reason names the count of commits.

## Invariants

- A refusal never prints the URL of the remote, because a URL can hold a token. It prints the host only (`TestCheck_RefusesUnpushedCommitsAndAnotherHost`).
- Each read error of `ReadRepo` is a `Refusal` with a cause: a detached HEAD, no upstream, no URL, or no upstream ref to compare (`TestReadRepo_RefusesATreeItCannotLaunchFrom`).
- `Render` then `Parse` gives the same session (`TestRender_ThenParse_ReturnsTheSession`). A review report holds no session (`TestParse_RefusesAReportThatHoldsNoLaunchedSession`).
