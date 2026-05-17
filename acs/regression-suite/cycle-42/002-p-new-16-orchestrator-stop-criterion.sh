#!/usr/bin/env bash
# ACS predicate: verify P-NEW-16 orchestrator stop-criterion section exists with all 3 gates
# cycle: 42
# ac: AC1 — evolve-orchestrator.md has ## STOP CRITERION section; AC2 — section has 3 named gates; AC3 — section placement correct (after Phase Loop, before Output Artifact)
#
# AC-ID: cycle-42-002
# Description: P-NEW-16 STOP CRITERION section present with 3 gates and correct placement
# Evidence: agents/evolve-orchestrator.md
# Author: builder (evolve-builder)
# Created: 2026-05-09T00:00:00Z
# Acceptance-of: P-NEW-16
#
# metadata: {"id":"002","slug":"p-new-16-orchestrator-stop-criterion","cycle":42,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
ORCHESTRATOR="$REPO_ROOT/agents/evolve-orchestrator.md"
[ -f "$ORCHESTRATOR" ] || { echo "ERR: $ORCHESTRATOR not found"; exit 1; }

rc=0

# AC1: ## STOP CRITERION section exists in evolve-orchestrator.md
if ! grep -q '^## STOP CRITERION$' "$ORCHESTRATOR"; then
    echo "FAIL AC1: evolve-orchestrator.md missing '## STOP CRITERION' section"
    rc=1
else
    echo "PASS AC1: '## STOP CRITERION' section found in evolve-orchestrator.md"
fi

# AC2: Section contains all 3 named completion gates
for gate in "phase-sequence-complete" "verdict-written" "cycle-state-advanced"; do
    if ! grep -q "$gate" "$ORCHESTRATOR"; then
        echo "FAIL AC2: STOP CRITERION missing gate '$gate'"
        rc=1
    else
        echo "PASS AC2: gate '$gate' present in STOP CRITERION"
    fi
done

# AC3: STOP CRITERION section appears AFTER ## Phase Loop and BEFORE ## Output Artifact
# v10.7 persona refactor: '## What You Are NOT Allowed To Do' heading was removed; equivalent
# guidance is now under '### Banned Post-Report Patterns' (subsection) + '## Shared Constraints'
# (which references AGENTS.md). '## Output Artifact' is the new semantic upper boundary — it
# marks the transition from instructions-to-orchestrator into output-format spec.
_stop_line=$(grep -n '^## STOP CRITERION$' "$ORCHESTRATOR" | head -1 | cut -d: -f1)
_phase_loop_line=$(grep -n '^## Phase Loop' "$ORCHESTRATOR" | head -1 | cut -d: -f1)
_output_artifact_line=$(grep -n '^## Output Artifact' "$ORCHESTRATOR" | head -1 | cut -d: -f1)

if [ -z "$_stop_line" ] || [ -z "$_phase_loop_line" ] || [ -z "$_output_artifact_line" ]; then
    echo "FAIL AC3: Could not locate all required section markers (stop=$_stop_line, phase-loop=$_phase_loop_line, output-artifact=$_output_artifact_line)"
    rc=1
elif [ "$_stop_line" -gt "$_phase_loop_line" ] && [ "$_stop_line" -lt "$_output_artifact_line" ]; then
    echo "PASS AC3: STOP CRITERION at line $_stop_line — between Phase Loop (line $_phase_loop_line) and Output Artifact (line $_output_artifact_line)"
else
    echo "FAIL AC3: STOP CRITERION placement incorrect (stop=$_stop_line, phase-loop=$_phase_loop_line, output-artifact=$_output_artifact_line)"
    rc=1
fi

exit $rc
