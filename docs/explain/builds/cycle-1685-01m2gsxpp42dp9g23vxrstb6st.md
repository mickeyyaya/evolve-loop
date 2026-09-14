# Build Explanation — Cycle 1685

## Build Binding
- Cycle: 1685
- Base SHA: d56e668cf1fb4bade371b2b76a2c55cada4ddc65

## Summary
`evalgate` can now tell the two zero-slug scout-report shapes apart. A report
whose `## Selected Tasks` section is present and full of real task prose that
yielded no slug is reported by the new `SelectedTasksParseMiss`, and Gate A
surfaces it as a non-blocking advisory on the line it already emits. A report
that simply claims no work stays silent, and the parser now also reads the
backtick-wrapped slug form that real scout reports emit inside the
`- **Slug:**` bullet.

## Rationale
`SelectedSlugs` returns `nil` both when nothing was claimed and when something
was claimed in a form the parser could not read. Gate A's `check()` — the only
seam an operator or agent ever observes — returned `("", false)` for both, so
"nothing to check" was byte-identical to "something to check that we failed to
read". That is cycle 1570: scout selected `config-gate-default-policy-authority`,
authored no eval, the section parsed to `nil`, Gate A's fail-open path approved,
and the missing eval surfaced three phases later as an audit H1 — after triage,
tdd and build had already spent their tokens.

The signal is advisory rather than a new hard block, and that is the load-bearing
decision. Blocking every zero-slug report would false-block every genuinely
converged cycle, which is precisely the failure the package header warns against
when it commits the blocking gates to firing only on CERTAIN violations. The
defect here is not that fail-open is wrong; it is that fail-open was SILENT. The
fix restores the missing observation without touching the blocking rule, so
`block` is latched from the missing/ungraded slug sets before the advisory is
appended and a parse-miss can never promote itself into a rejection.

The detector is scoped to the bounded `## Selected Tasks` body and to slug
bullets only. A malformed `## Decision Trace` is a separate fail-open path with
its own semantics, and folding it in would make one advisory answer for two
independent parse surfaces — the operator could no longer tell from the message
which one drifted.

Widening `slugLineRE` to tolerate a backticked slug was measured, not assumed.
Of the 84 cycle-16xx scout reports carrying a `## Selected Tasks` section, 12
wrap the slug in backticks inside that bullet, which the old pattern did not
match. Those 12 are the *marginal* contribution of that one formatting variant —
about one cycle in seven, the widening's effect in isolation and NOT the rate at
which the advisory speaks.

The widening is not blocking-neutral, and the first version of this document said
it was. That sentence rested on a count of `## Decision Trace` headings, which is
a different claim from union equality and was never checked against it. Measured
properly on 2026-09-15 — running the shipped `SelectedSlugs` over every
`.evolve/runs/cycle-16*/scout-report.md` and diffing against the pre-widening
pattern — the union differs on 6 of the 84 reports: cycle-1605, -1610, -1634,
-1664, -1665 and -1669, each going from empty to non-empty. All six do carry a
`## Decision Trace`, but each states its selection as a `"selected_tasks"` string
array, a shape `decisionTraceSelected` does not read, so the trace supplies
nothing and the bullet is the only source of the slug. On 2 of those 6 the newly
parsed slug has no eval file at either path `evalFilePath` checks, so Gate A
blocks at `StageEnforce` where it previously fail-opened: cycle-1664
(`settle-wait-stability-shortcircuit`) and cycle-1669
(`verdict-tool-call-claudep`).

That is a deliberate capability increase rather than a regression. Catching a
selected slug whose eval was never written is exactly Gate A's cycle-166 job, and
on both of those cycles the eval really is absent — the widening makes the gate
see two claims it had been silently dropping. The cost is the honest one: two
reports of that shape would now be stopped at scout rather than at audit. Because
the corpus that measurement ran over is gitignored and absent in CI, the claim is
also carried by `TestSlugLineWideningIsNotBlockingNeutral`, which reproduces both
reports and re-derives the effect on every run instead of asserting it.

The advisory's own operating point was measured on 2026-09-15 by running the
shipped detector over every `.evolve/runs/cycle-16*/scout-report.md`. All 84 of
those reports carry a `## Selected Tasks` section, and `SelectedTasksParseMiss`
fires on 59 of 84 — 70.2%. That is the figure to reason about, and it is a
finding rather than a false-positive rate: 56 of the 59 firing reports have an
entirely empty `SelectedSlugs` union, so on roughly two of every three cycles
Gate A really has been checking nothing. (For scale: the pre-fix pattern fired on
71 of the 84, so the backtick widening is what removed 12 of them.) Shipping the
signal advisory-only is what makes that rate survivable — a hard block at this
operating point would have stopped the majority of cycles on its first day.

The rejected alternative was to make the detector consider the full
`SelectedSlugs` union rather than the section's own bullets. It would have
satisfied the predicates equally, but it conflates two questions: a drifted
section beside a healthy trace is still drift worth naming, and a signal called
`SelectedTasksParseMiss` that silently consults the trace would mislead the next
reader of Gate A's log line.

## Changed Areas
- `go/internal/evalgate/slugs.go` — adds the exported `SelectedTasksParseMiss`
  detector; extracts `selectedTasksBody` and `slugBullets` so the miss signal and
  `selectedTaskSlugs` share ONE section bound, since a signal computed over a
  different span than the parse it reports on would accuse a section the parser
  never read; widens `slugLineRE` to accept the backticked bullet real reports
  emit.
- `go/internal/evalgate/materialization.go` — Gate A's `check()` latches its
  blocking verdict from the missing/ungraded slug sets before appending the new
  `parseMissAdvisory` clause, so the advisory reaches the emitted log line
  without being able to change what blocks. The advisory's wording is scoped to
  what the gate actually did: it claims nothing was checked only when the union
  is empty, and otherwise names the slugs the `## Decision Trace` supplied.
- `go/internal/evalgate/parsemiss_test.go` — new: the four grader-named tests,
  pinning the real cycle-1570 report shape by its actual slug, the convergence
  and contentless-section negatives, the section bound, the Decision Trace
  scoping, and the caller proof that drives `NewReviewer(...).Review(...)` and
  asserts the advisory both appears and does not reject. Two further subtests pin
  the wording in both directions — the nothing-was-checked claim is kept on an
  empty union and dropped when the trace supplied slugs. It also carries
  `TestSlugLineWideningIsNotBlockingNeutral`, which reproduces the cycle-1664 and
  cycle-1669 reports and executes the widening's union and blocking deltas
  against the production reviewer.
- `go/acs/cycle1685/predicates_test.go` — the tdd phase's acceptance predicates
  for this cycle; authored by that phase, unmodified here.
- `.evolve/evals/evalgate-selectedslugs-nil-blindness.md` — the scout-authored
  durable eval whose seven `[code]` graders this build satisfies (five on the
  detector, two pinning this document's measured operating point).
- `.evolve/evals/evalgate-parse-miss-vs-convergence-signal.md` — the scout task's
  companion eval, authored by the scout phase and unmodified here.
- `.evolve/inbox/2026-08-26T15-00-00Z-evalgate-selectedslugs-nil-blindness.json` — removed: this is the
  inbox request the cycle implements, and it leaves the pending queue so a satisfied item is never
  re-planned. A move, not a discard: every field of the record survives at the consumed path below,
  unchanged except for the added `consumed` stamp (diffing the record at base `d56e668c` against the
  consumed copy shows that stamp as the only delta).
- `.evolve/inbox/consumed/2026-08-26T15-00-00Z-evalgate-selectedslugs-nil-blindness.json` — added: the same
  record at its consumed location, carrying the `consumed` stamp (`via: ship`) that records how the request
  was satisfied, so it stays auditable after it leaves the queue.

## Design Decisions
`SelectedTasksParseMiss` is true only when all three conditions hold: the heading
is present, the bounded body still holds non-whitespace content once HTML
comments are stripped, and no slug bullet parses out of that body. Stripping
comments matters because a `<!-- none selected this cycle -->` placeholder is a
claim of no work, not unreadable content, and a naive "heading present and zero
slugs" test would report it as drift.

`selectedTasksBody` is the single home of the section arithmetic. Two copies
could drift apart, and this cycle exists because two views of the same report
disagreed about what had been read.

The advisory's text names the remedy — restate the slug as a `- **Slug:**`
bullet or as a `## Decision Trace` entry — because Gate A's own history is that
a rejection naming only a bare stem is not actionable.

The message asserts exactly what the gate knows. A parse-miss contributes no
bullets, so a non-empty union at that point came from the `## Decision Trace`
alone and those slugs WERE checked; the message then names them instead of
claiming nothing was checked. That case is a minority but real — 3 of the 59
measured fires — and a signal that overstates on one fire in twenty is a signal
an operator learns to discount.

## Verification
The eval's `[code]` graders were run individually and each printed its
`--- PASS:` line, so none passed vacuously through a `-run` pattern that matched
no test. The cycle's seventeen acceptance predicates pass, including the caller
proof that the advisory reaches Gate A through the production reviewer, the
apicover check that the new export is named and documented, the two that
re-derive the operating point stated above from its own fraction, and the five
added in audit round 2 that pin the widening's measured effect and require every
deliverable to state it. The full Go module suite, `go vet ./...` and
`gofmt -l .` are clean, and the native ACS suite is green.

## Compatibility
`SelectedSlugs`'s signature and its fail-open contract are unchanged, and Gate A's
blocking RULE is unchanged: it still blocks only on a selected slug with no eval
file or with an eval carrying no `[code]` grader. What changes is the input that
rule sees. The widened `slugLineRE` accepts a strict superset of what it accepted
before, so some reports become newly blockable — measured over the cycle-16*
corpus, 2 of the 84 do: cycle-1664 and cycle-1669, each carrying a backticked
slug whose eval was never written. Callers that relied on a parse-miss reaching
them as an empty union will now receive that slug instead.

## Limitations
The advisory is not yet read by any consumer other than the phase log, and
nothing counts parse-misses over time, so the signal repeats without escalating.
At the measured 70.2% fire rate that is this cycle's most consequential
limitation rather than a hypothetical one: the scout persona is already drifting
on most cycles, so an operator sees the same un-escalated line again and again,
and habituation is the realistic failure mode. What to do about it — escalate on
a streak, or fix the persona the signal is reporting on — is deliberately left to
a follow-up; making the condition visible at all was this cycle's job. The
detector also does not attempt to recover the slug it failed to parse; it reports
that a claim was made and lost, not what the claim was.
