# History: how the decision conflicts were found and resolved, 2026-11-06

- **Purpose:** this record shows the progress of the reasoning and holds the chain of evidence for [ADR-0164](../architecture/adr/0164-all-decisions-serve-one-goal-and-one-amendment-resolves-their-conflicts.md).
- **Writer:** the console, a Claude Code session. Times are Asia/Taipei.

## 1. The chain of evidence

Git hashes pin what this review read. A change to one byte of one file changes its tree hash. So a reader can prove the input of this review with the commands below.

| Input | Hash |
|---|---|
| Commit that the review read | `6ae4972c389ddcc57aebd34a797b92c4ab48740c` |
| `docs/` tree | `e39270f74aee96a47037912de592a217fa267f9a` |
| `docs/architecture/adr/` tree | `b04287505a83081a62c64f20040a0f8e0343e808` |
| `docs/plans/` tree | `a50358b03f334180a6dfa4a4cda54213521acee3` |
| `docs/research/` tree | `400ba52360b30f8bafc7c2d046a19608b182cc8d` |
| `.evolve/policy.json` of the runtime plane on the host, SHA-256. This file is not in git, so only its digest is proof. | `9e0b79be9beba1e219699c1eeeba65a55fb62166eff2e9978d9e8f4afc5cdaca` |

To verify, run:

```
git rev-parse 6ae4972c3:docs/architecture/adr
git show 6ae4972c3:docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md
```

The commit that lands this record chains to that commit. The landing commit is the seal: a later edit of this record shows in `git log`. A review report and a train record add to the seal.

**Limits of the chain.**
- A hash proves what the files held. It does not prove that the reasoning is right.
- The review reports live outside git, in the session scratchpad. Their run labels are in the commit gate attestation.
- The web pages can change. The research names each URL and the date of the search.

## 2. The request, in the words of the operator

| Order | Words |
|---|---|
| 1 | "Could you review all docs to identify the contradiction within different adrs, plans, research and investigate with online research to find the best practical solution with 3 options and pick the winner with others advantages" |
| 2 | "And compare with the current code implementation to learn if there is any fap between design, architecture, best practice, ai driven factory pipeline" |
| 3 | "I want to make sure all adrs and plans are actually pointing to the same goal : a self healing ai driven development loop which continue to deliver the the best quality features with top design and top quality deliverables. It relentless building features and is capable of fixing any issues it faced with logic and research to build the best practical options" |
| 4 | "Review all adrs and start with docs review and fix first before touching the code" |
| 5 | "Make the amendment to resolve the confliction and record all the history with reasoning and actions and activities, the goal is to show the progress of reasoning and secure the chain of evidence" |
| 6 | "Build the constitution based on the goal I give that all LLM agent need to know the ultimate goal we try to achieve to avoid only on the local optimization" |
| 7 | "This goal should be the top law that all agents need to follow as it's "constitution" for this project" |

## 3. The progress of the reasoning

| Step | Activity | Result | What changed in the reasoning |
|---|---|---|---|
| 1 | Size the set with `find` and `wc`. | 2209 Markdown files, about 2 million words in the main folders. Usage 81%, brake 83%. | A full read is not possible now. So a digest comes first, and the limit is stated. |
| 2 | Write a digest of status and decision for 161 ADRs. Read it in full. SHA-256 of the first part: `990492bd0251540fda32f5bcb559c61182b3b9a54c08b7101521efd0587b6976`. | Candidate conflicts. | The first idea was a list of content conflicts. The status spread pointed to a deeper fault. |
| 3 | Read exact lines for each candidate. | C1 to C12. | A candidate counts only with a `path:line` on each side. |
| 4 | Compare with code through `evolve code` and the policy. | G1 to G7. | Stale code paths are rare: 12 of 602. So path rot is not the main fault. Status and authority are. |
| 5 | Run live web searches. | P1 to P7. | The practice is new record plus flipped status, and checks that fail CI. No source shows an automatic detector of contradictions. |
| 6 | Score three options. | B wins at 20, against 15 and 14. | The scores are a judgment. Step 1 of the winner takes the fast fix of A, and step 5 takes the reach of C. |
| 7 | Map each ADR and plan to the goal (request 3). | [loop-goal.md](../architecture/loop-goal.md). | The review question changes from "do they conflict" to "do they serve one goal". Research is the thinnest part. |
| 8 | Read ADR-0126 again. | Decision 14 already unifies the counts. | C11 is withdrawn as a conflict. The first reading took the title alone. |
| 9 | Read `skills/commit/SKILL.md` and the `evolve ship` usage. | The sanctioned path is `evolve ship --class manual`. | C12 is not a document conflict. It is a deviation by the console. An amendment must not bless it, so it stays open. |
| 10 | Write ADR-0164 and the notes. | Section 4. | The old text stays, and a note links to the amendment. |
| 11 | Build the constitution (request 6). Read `docs/architecture/guidance-layers.md` for the path that each agent loads. | [constitution.md](../architecture/constitution.md), two L0 rules. | A new loader needs code, and the operator said docs first. L0 already reaches each CLI file. The rules state a limit, because "serve the goal" can read as a license to skip a gate. |
| 12 | Fit the rule budget. `evolve guidance check` refused two statements of 33 and 34 words. | Each statement is now at most 25 words. | Words that left a statement moved to its reason text, so no fact was lost. |
| 13 | Rank the goal as the top law (request 7). | The first L0 rule starts with "Top law", and the constitution has a table of ranks. | A top law can read as a license to overrule a gate. The record states the opposite: an agent reports the conflict, and the operator corrects the lower text. This is the console's reading. |
| 14 | Test `evolve ship --class manual --dry-run` on the docs branch. | It reports a commit and push to that branch. | The sanctioned path works on a branch, so this PR uses it. Earlier commits did not. |
| 15 | Run the docs review, run `agent-docs-review-20261106T001125Z-204c`, on `claude-tmux` after the two agy routes exited 85. | FAIL: 1 HIGH, 5 MEDIUM, 3 LOW. It verified each tree hash and the map of 161 ADRs and 59 plans. | Three findings changed the reasoning. The limit on the law was absent from the digest, so it moved into a rule. Decisions 9 and 10 were stated as facts, and they now ask the operator. The body of the comment rule still said the old rule. |
| 16 | The same seat quoted the two first rules that it loaded. | Both lines match. | One claude seat carries the law. No agy, codex or ollama seat was tested, so the proof for those is the text in their files. |
| 17 | The review said that the name in row G3 is absent. | `evolve code find Oversight` returns the type at `oversight.go:44`. | The console had asked about a wrong name, `NewOversight`, and the seat missed the type. Each side corrected one error. |

## 4. The actions

| Action | Files |
|---|---|
| Goal map | `docs/architecture/loop-goal.md` |
| Amendment | `docs/architecture/adr/0164-*.md` |
| Research | `docs/research/doc-contradictions-2026-11.md`, with its index entry |
| Notes on superseded or corrected records | ADR-0001, 0058, 0072, 0080, 0096, 0126, 0139, 0151, 0154 |
| Notes on absent script paths | ADR-0001 to 0009, 0012, 0018, 0019, 0025, 0030, 0040, 0062 |
| Series note | `docs/adr/README.md` |
| Corrected specs | `docs/architecture/llm-isolation.md`, `docs/architecture/integration-driver.md` |
| Hard-rule text | `guidance/rules/code-zero-comments.md`, `guidance/rules/pipeline-only-controller.md`, and their generated copies |
| Constitution | `docs/architecture/constitution.md`, `guidance/rules/the-ultimate-goal.md`, `guidance/rules/serve-the-goal-not-the-metric.md` |
| Rule order | Each L0 rule moved down by two places, so the goal comes first |

## 5. Errors of the console in this period

The chain must hold these too.

| Error | Effect | Correction |
|---|---|---|
| The floor of `evolve auth store-times` skipped `internal/bridge`. | The train ejected the PR on `cover changed`, then on `apicover`. | Tests added, and the floor now covers each changed package. |
| A chained command closed PR 1313 in the same step that read a failed review. | The PR closed before its replacement was ready. | Replaced by PR 1319. Read a result before acting on it. |
| The review helper ran with no seats. | The commit gate refused, and nothing changed. | Checked the tree and the remote, then ran it with seats. |
| `git rebase --abort` ran during a conflict fix. | The resolution was lost. | Redone with `git rebase --continue`. |
| Commits used a bare `git commit` and `git push` after the gate. | Item C12. The skill names `evolve ship --class manual`. | Open. The operator decides. |
| The first summary of a review counted quoted numbers as findings (1 CRITICAL, 4 HIGH). | The report held one HIGH. | Read the report body. |
| The research cited a wrong symbol, two wrong lines and a count of 12 that held two withdrawn items. | The docs review found them. | Corrected before the landing. |

## 6. Related landings in the same period

| PR | Commit | What it did |
|---|---|---|
| 1316 | `bbfdb754e` | No cell can read the seat token holder. |
| 1317 | `234b34ec2` | Agent text cannot assert a logout. |
| 1321 | `6ae4972c3` | The integration driver composes lane deliveries at shadow. |
