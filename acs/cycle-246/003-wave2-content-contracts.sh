#!/usr/bin/env bash
# ACS cycle-246/003 — AC7-AC10: catalog §3 content contracts on the authored
# phase specs themselves.
#   AC7  benchmark-gate agent.md: multi-sample statistical comparison +
#        benchstat; phase.json gates on perf.significant.
#   AC8  cleanup-sweep agent.md: explicit detection-only, forbids edits;
#        phase.json writes_source == false (the load-bearing floor guard).
#   AC9  fuzz-probe routing scoped to parser/decoder/unmarshal surfaces;
#        phase.json gates on fuzz.crashers > 0.
#   AC10 rollback-plan gates on rollback.ready == false.
# acs-predicate: config-check (the agent.md/phase.json content IS the
# config-only deliverable of this cycle; there is no runtime to invoke until
# the advisor routes these phases in a live cycle — shadow-run is the
# post-ship verification per catalog §7).
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
P="$ROOT/.evolve/phases"

# AC7 — benchmark-gate
grep -qi 'benchstat' "$P/benchmark-gate/agent.md" \
  || { echo "RED: benchmark-gate agent.md lacks benchstat reference" >&2; exit 1; }
grep -qiE 'count=|multi[- ]sample|samples|[0-9]+ (runs|times|iterations)' "$P/benchmark-gate/agent.md" \
  || { echo "RED: benchmark-gate agent.md lacks multi-sample instruction" >&2; exit 1; }
jq -e '.classify.fail_if_signal["perf.significant"] == "==true"' "$P/benchmark-gate/phase.json" >/dev/null 2>&1 \
  || { echo "RED: benchmark-gate fail_if_signal perf.significant != ==true" >&2; exit 1; }

# AC8 — cleanup-sweep
grep -qiE 'detection[ -]only' "$P/cleanup-sweep/agent.md" \
  || { echo "RED: cleanup-sweep agent.md lacks detection-only statement" >&2; exit 1; }
grep -qiE 'do (NOT|not).*(edit|remove|delete|modify)|no (file )?(edits|removals|deletions)' "$P/cleanup-sweep/agent.md" \
  || { echo "RED: cleanup-sweep agent.md does not forbid edits/removals" >&2; exit 1; }
[ "$(jq -r '.writes_source // false' "$P/cleanup-sweep/phase.json" 2>/dev/null)" = "false" ] \
  || { echo "RED: cleanup-sweep writes_source != false" >&2; exit 1; }

# AC9 — fuzz-probe
jq -r '.routing' "$P/fuzz-probe/phase.json" 2>/dev/null | grep -qiE 'pars|decod|unmarshal' \
  || { echo "RED: fuzz-probe routing not scoped to parser/decoder/unmarshal" >&2; exit 1; }
jq -e '.classify.fail_if_signal["fuzz.crashers"] == ">0"' "$P/fuzz-probe/phase.json" >/dev/null 2>&1 \
  || { echo "RED: fuzz-probe fail_if_signal fuzz.crashers != >0" >&2; exit 1; }

# AC10 — rollback-plan
jq -e '.classify.fail_if_signal["rollback.ready"] == "==false"' "$P/rollback-plan/phase.json" >/dev/null 2>&1 \
  || { echo "RED: rollback-plan fail_if_signal rollback.ready != ==false" >&2; exit 1; }

echo "PASS"; exit 0
