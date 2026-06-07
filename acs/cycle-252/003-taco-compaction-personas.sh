#!/usr/bin/env bash
# ACS — cycle-252 task `taco-trajectory-compression-builder`
# acs-predicate: config-check (grep waiver) — the deliverable IS persona
# prose (a "Mid-Trajectory Compaction Protocol" section in two agent
# definitions); the behavior it changes lives in the LLM reading it, so no
# subprocess can exercise it. Anchors are STRUCTURAL (exact section
# heading + the protocol's three load-bearing elements), not magic
# strings: scout's published grep (`CHECKPOINT|mid-trajectory`) was
# already pre-existing-GREEN on evolve-builder.md (Budget Checkpoint
# Protocol, line 48), so this predicate pins the stricter contract.
# Auditor: review waiver validity + prose quality.
set -uo pipefail

top=$(git rev-parse --show-toplevel)

fail=0
for persona in agents/evolve-builder.md agents/evolve-tdd-engineer.md; do
    f="$top/$persona"
    # Exact new section heading (NOT satisfied by the pre-existing
    # "Budget Checkpoint Protocol").
    if ! grep -qE '^##+ .*Mid-Trajectory Compaction Protocol' "$f"; then
        echo "RED: $persona missing '## Mid-Trajectory Compaction Protocol' section heading" >&2
        fail=1
        continue
    fi
    # Scope element checks to the section BODY (heading → next heading) so
    # pre-existing prose elsewhere (e.g. builder.md's "At turn 15" in the
    # Budget Checkpoint Protocol) cannot satisfy them.
    section=$(awk '/^##+ .*Mid-Trajectory Compaction Protocol/{inside=1; next} inside && /^#+ /{exit} inside{print}' "$f")
    # Element 1: the 15-turn boundary trigger.
    if ! echo "$section" | grep -qiE '(15[- ]turn|turn[- ]15|every 15 turns)'; then
        echo "RED: $persona compaction protocol lacks the 15-turn boundary trigger" >&2
        fail=1
    fi
    # Element 2: the 3-bullet CHECKPOINT block format.
    if ! echo "$section" | grep -qiE '(3|three)[- ]bullet.*CHECKPOINT|CHECKPOINT.*(3|three)[- ]bullet'; then
        echo "RED: $persona compaction protocol lacks the 3-bullet CHECKPOINT format" >&2
        fail=1
    fi
    # Element 3: the release-attention rule (drop raw tool results).
    if ! echo "$section" | grep -qiE 'releas(e|ing) attention|release.*raw tool results'; then
        echo "RED: $persona compaction protocol lacks the release-attention rule" >&2
        fail=1
    fi
done

[ "$fail" -eq 0 ] || exit 1
echo "GREEN: Mid-Trajectory Compaction Protocol present in builder + tdd-engineer personas (heading + 15-turn trigger + 3-bullet CHECKPOINT + release-attention)"
exit 0
