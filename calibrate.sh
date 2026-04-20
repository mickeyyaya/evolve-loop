#!/bin/bash

# Dimension 1: Documentation Completeness
TOTAL_SKILLS=$(find skills/ -name "*.md" -not -name "SKILL.md" | wc -l | tr -d ' ')
WITH_FM=$(grep -rl "^---" skills/ --include="*.md" | grep -v SKILL.md | wc -l | tr -d ' ')
SKILL_FM_PCT=$((WITH_FM * 100 / (TOTAL_SKILLS > 0 ? TOTAL_SKILLS : 1)))
TOTAL_AGENTS=$(find agents/ -name "*.md" | wc -l | tr -d ' ')
AGENT_FM=$(grep -rl "^tools:" agents/ --include="*.md" | wc -l | tr -d ' ')
AGENT_FM_PCT=$((AGENT_FM * 100 / (TOTAL_AGENTS > 0 ? TOTAL_AGENTS : 1)))
README_EXISTS=$(test -s README.md -o -s CLAUDE.md -o -s GEMINI.md -o -s AGENTS.md && echo 100 || echo 0)
BROKEN_LINKS=0
for src in $(grep -rlE '\]\([^)]*\.md\)' skills/ agents/ docs/ 2>/dev/null); do
  srcdir=$(dirname "$src")
  for link in $(grep -oE '\]\([^)]*\.md[^)]*\)' "$src" | grep -oE '\([^)]+\)' | tr -d '()' | sed 's/#.*//'); do
    if [ ! -f "$srcdir/$link" ] && [ ! -f "$link" ]; then
      BROKEN_LINKS=$((BROKEN_LINKS + 1))
    fi
  done
done
LINK_SCORE=$((BROKEN_LINKS == 0 ? 100 : (BROKEN_LINKS < 3 ? 75 : (BROKEN_LINKS < 6 ? 50 : 25))))
DOCS_SCORE=$(test -d docs && test "$(ls docs/*.md 2>/dev/null | wc -l)" -gt 0 && echo 100 || echo 50)
DOC_AUTO=$(( (SKILL_FM_PCT + AGENT_FM_PCT + README_EXISTS + LINK_SCORE + DOCS_SCORE) / 5 ))

# Dimension 2: Specification Consistency
AGENT_REFS=$(grep -oE 'evolve-[a-z]+\.md' skills/evolve-loop/SKILL.md | sort -u)
MISSING_AGENTS=0
for a in $AGENT_REFS; do test -f "agents/$a" || MISSING_AGENTS=$((MISSING_AGENTS + 1)); done
AGENT_REF_SCORE=$((MISSING_AGENTS == 0 ? 100 : (100 - MISSING_AGENTS * 25)))
PHASES_FIELDS=$(grep -oE 'state\.json\.?[a-zA-Z]+' skills/evolve-loop/phases/phases.md 2>/dev/null | sed 's/state\.json\.*//' | sort -u | head -20)
PROTO_FIELDS=$(grep -oE '"[a-zA-Z]+"' skills/evolve-loop/memory-protocol.md 2>/dev/null | tr -d '"' | sort -u)
MISSING_FIELDS=0
for f in $PHASES_FIELDS; do [ -z "$f" ] && continue; echo "$PROTO_FIELDS" | grep -qw "$f" || MISSING_FIELDS=$((MISSING_FIELDS + 1)); done
FIELD_SCORE=$((MISSING_FIELDS == 0 ? 100 : (100 - MISSING_FIELDS * 10)))
PHASE_NUMS=$(grep -oE 'Phase [0-9]+' skills/evolve-loop/phases/phases.md | grep -oE '[0-9]+' | sort -n | uniq)
MAX_PHASE=$(echo "$PHASE_NUMS" | tail -1)
EXPECTED_SEQ=$(seq 0 $MAX_PHASE)
SEQ_SCORE=$(test "$(echo "$PHASE_NUMS" | tr '\n' ' ')" = "$(echo "$EXPECTED_SEQ" | tr '\n' ' ')" && echo 100 || echo 50)
PROTO_FILES=$(grep -oE '[a-z]+-[a-z]+\.md' skills/evolve-loop/memory-protocol.md | sort -u)
PHASE_FILES=$(grep -oE 'workspace/[a-z]+-[a-z]+\.md' skills/evolve-loop/phases/phases.md | sed 's|workspace/||' | sort -u)
FILE_DIFF=$(comm -23 <(echo "$PHASE_FILES") <(echo "$PROTO_FILES") | wc -l | tr -d ' ')
FILE_SCORE=$((FILE_DIFF == 0 ? 100 : (100 - FILE_DIFF * 20)))
SPEC_AUTO=$(( (AGENT_REF_SCORE + FIELD_SCORE + SEQ_SCORE + FILE_SCORE) / 4 ))

# Dimension 3: Defensive Design
ROLLBACK=$(grep -rci -e 'rollback' -e 'revert' -e 'git revert' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
ROLLBACK_SCORE=$((ROLLBACK >= 3 ? 100 : (ROLLBACK >= 1 ? 50 : 0)))
HALT_SKILLS=$(grep -rc 'HALT' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
HALT_OPERATOR=$(grep -c 'HALT' agents/evolve-operator.md 2>/dev/null || true)
HALT_OPERATOR=${HALT_OPERATOR:-0}
HALT_SCORE=$(( (HALT_SKILLS > 0 && HALT_OPERATOR > 0) ? 100 : 50 ))
RETRY=$(grep -rci -e 'max.*3' -e '3.*attempt' -e 'max.*iteration' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
RETRY_SCORE=$((RETRY >= 2 ? 100 : (RETRY >= 1 ? 50 : 0)))
CLEANUP=$(grep -rci -e 'worktree.*cleanup' -e 'worktree.*prune' -e 'worktree.*remove' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
CLEANUP_SCORE=$((CLEANUP >= 2 ? 100 : (CLEANUP >= 1 ? 50 : 0)))
VALIDATION=$(grep -rci 'validat' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
VALIDATION_SCORE=$((VALIDATION >= 2 ? 100 : (VALIDATION >= 1 ? 50 : 0)))
DEFENSE_AUTO=$(( (ROLLBACK_SCORE + HALT_SCORE + RETRY_SCORE + CLEANUP_SCORE + VALIDATION_SCORE) / 5 ))

# Dimension 4: Eval Infrastructure
EVAL_RUNNER=$(test -f skills/evolve-loop/utils/eval-runner.md && echo 100 || echo 0)
EVAL_SECTIONS=$(grep -cE '## Code Graders|## Regression Evals|## Acceptance Checks' skills/evolve-loop/utils/eval-runner.md 2>/dev/null || true)
EVAL_SECTIONS=${EVAL_SECTIONS:-0}
EVAL_FORMAT_SCORE=$((EVAL_SECTIONS >= 3 ? 100 : (EVAL_SECTIONS * 33)))
CHECKSUM=$(grep -rc -e 'checksum' -e 'sha256' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
CHECKSUM_SCORE=$((CHECKSUM >= 2 ? 100 : (CHECKSUM >= 1 ? 50 : 0)))
TAMPER=$(grep -rci 'tamper' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
TAMPER_SCORE=$((TAMPER >= 2 ? 100 : (TAMPER >= 1 ? 50 : 0)))
MUTATION=$(grep -rci 'mutation' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
MUTATION_SCORE=$((MUTATION >= 3 ? 100 : (MUTATION >= 1 ? 50 : 0)))
EVAL_AUTO=$(( (EVAL_RUNNER + EVAL_FORMAT_SCORE + CHECKSUM_SCORE + TAMPER_SCORE + MUTATION_SCORE) / 5 ))

# Dimension 5: Modularity
OVER_800=$(find skills/ agents/ docs/ -name "*.md" -exec wc -l {} + 2>/dev/null | awk 'index($0,"total")==0 && $1+0 > 800 {count++} END {print count+0}')
SIZE_SCORE=$((OVER_800 == 0 ? 100 : (OVER_800 == 1 ? 75 : (OVER_800 <= 3 ? 50 : 25))))
PHASES_LINES=$(wc -l < skills/evolve-loop/phases/phases.md | tr -d ' ')
PHASES_SCORE=$((PHASES_LINES < 400 ? 100 : (PHASES_LINES < 600 ? 75 : (PHASES_LINES < 800 ? 50 : 25))))
AGENT_MAX=$(find agents/ -name "*.md" -exec wc -l {} + 2>/dev/null | awk 'index($0,"total")==0 {if ($1+0 > max) max=$1+0} END {print max+0}')
AGENT_SIZE_SCORE=$((AGENT_MAX < 200 ? 100 : (AGENT_MAX < 300 ? 75 : (AGENT_MAX < 400 ? 50 : 25))))
SKILL_FILES=$(find skills/evolve-loop/ -name "*.md" | wc -l | tr -d ' ')
SEPARATION_SCORE=$((SKILL_FILES >= 5 ? 100 : (SKILL_FILES >= 3 ? 75 : 50)))
AGENT_CROSS=$(grep -rlE 'evolve-builder|evolve-auditor|evolve-operator' agents/evolve-scout.md 2>/dev/null | wc -l | tr -d ' ')
CIRCULAR_SCORE=$((AGENT_CROSS == 0 ? 100 : 50))
MOD_AUTO=$(( (SIZE_SCORE + PHASES_SCORE + AGENT_SIZE_SCORE + SEPARATION_SCORE + CIRCULAR_SCORE) / 5 ))

# Dimension 6: Schema Hygiene
INIT_FIELDS=$(grep -oE '"[a-zA-Z]+"' skills/evolve-loop/SKILL.md | head -40 | tr -d '"' | sort -u)
PROTO_SCHEMA=$(grep -oE '"[a-zA-Z]+"' skills/evolve-loop/memory-protocol.md | tr -d '"' | sort -u)
INIT_MISSING=0
for f in $INIT_FIELDS; do echo "$PROTO_SCHEMA" | grep -qw "$f" || INIT_MISSING=$((INIT_MISSING + 1)); done
INIT_SCORE=$((INIT_MISSING == 0 ? 100 : (100 - INIT_MISSING * 10)))
DUP_FIELDS=$(grep -oE '^ *"[a-zA-Z]+"' skills/evolve-loop/memory-protocol.md | sort | uniq -d | wc -l | tr -d ' ')
DUP_SCORE=$((DUP_FIELDS == 0 ? 100 : (100 - DUP_FIELDS * 20)))
JSON_BLOCKS=$(grep -c '```json' skills/evolve-loop/memory-protocol.md 2>/dev/null || true)
JSON_BLOCKS=${JSON_BLOCKS:-0}
JSON_SCORE=$((JSON_BLOCKS >= 3 ? 100 : (JSON_BLOCKS >= 1 ? 75 : 50)))
LAYERS=$(grep -oE 'Layer [0-9]+' skills/evolve-loop/memory-protocol.md | grep -oE '[0-9]+' | sort -n)
LAYER_SEQ_SCORE=$(test "$(echo $LAYERS | xargs)" = "0 1 2 3 4 5 6" && echo 100 || echo 50)
SCHEMA_AUTO=$(( (INIT_SCORE + DUP_SCORE + JSON_SCORE + LAYER_SEQ_SCORE) / 4 ))

# Dimension 7: Convention Adherence
AGENT_NAMING=$(find agents/ -name "evolve-*.md" | wc -l | tr -d ' ')
TOTAL_AGENT_FILES=$(find agents/ -name "*.md" | wc -l | tr -d ' ')
NAMING_SCORE=$((TOTAL_AGENT_FILES > 0 ? (AGENT_NAMING * 100 / TOTAL_AGENT_FILES) : 100))
NON_KEBAB=$(find skills/evolve-loop/ -name "*.md" ! -name "SKILL.md" | grep '[A-Z_]' | wc -l | tr -d ' ')
KEBAB_SCORE=$((NON_KEBAB == 0 ? 100 : (100 - NON_KEBAB * 20)))
CONVENTIONAL=$(git log --oneline -10 | grep -cE '^[a-f0-9]+ (feat|fix|refactor|docs|test|chore|perf|ci):' || true)
CONVENTIONAL=${CONVENTIONAL:-0}
COMMIT_SCORE=$((CONVENTIONAL * 10))
DIR_SCORE=$(test -d skills/evolve-loop -a -d agents -a -d docs && echo 100 || echo 50)
WRONG_HEADERS=$(grep -rn -E '^={4,}$|^-{4,}$' skills/ agents/ 2>/dev/null | wc -l | tr -d ' ')
HEADER_SCORE=$((WRONG_HEADERS == 0 ? 100 : (100 - WRONG_HEADERS * 10)))
CONV_AUTO=$(( (NAMING_SCORE + KEBAB_SCORE + COMMIT_SCORE + DIR_SCORE + HEADER_SCORE) / 5 ))

# Dimension 8: Feature Coverage
PHASES_DEFINED=$(grep -cE '### Phase [0-9]+:' skills/evolve-loop/phases/phases.md 2>/dev/null || true)
PHASES_DEFINED=${PHASES_DEFINED:-0}
PHASE_COV_SCORE=$((PHASES_DEFINED >= 5 ? 100 : (PHASES_DEFINED * 20)))
MISSING=0
for a in $(grep -oE 'evolve-[a-z]+\.md' skills/evolve-loop/SKILL.md | sort -u | tr '\n' ' '); do
  test -f "agents/$a" || MISSING=$((MISSING + 1))
done
AGENT_COV_SCORE=$((MISSING == 0 ? 100 : (100 - MISSING * 25)))
SUPPORT_FILES="skills/evolve-loop/utils/eval-runner.md skills/evolve-loop/memory-protocol.md skills/evolve-loop/phases/phases.md"
SUP_MISSING=0
for f in $SUPPORT_FILES; do test -f "$f" || SUP_MISSING=$((SUP_MISSING + 1)); done
SUPPORT_SCORE=$((SUP_MISSING == 0 ? 100 : (100 - SUP_MISSING * 33)))
if test -x publish.sh; then PUBLISH_SCORE=100
elif test -f publish.sh; then PUBLISH_SCORE=50
else PUBLISH_SCORE=0; fi
DOC_STUBS=0
for f in docs/*.md; do test -f "$f" && test "$(wc -l < "$f" | tr -d ' ')" -lt 5 && DOC_STUBS=$((DOC_STUBS + 1)); done 2>/dev/null
STUB_SCORE=$((DOC_STUBS == 0 ? 100 : (100 - DOC_STUBS * 20)))
FEAT_AUTO=$(( (PHASE_COV_SCORE + AGENT_COV_SCORE + SUPPORT_SCORE + PUBLISH_SCORE + STUB_SCORE) / 5 ))

# Output Results
cat <<EOF
{
  "documentationCompleteness": $DOC_AUTO,
  "specificationConsistency": $SPEC_AUTO,
  "defensiveDesign": $DEFENSE_AUTO,
  "evalInfrastructure": $EVAL_AUTO,
  "modularity": $MOD_AUTO,
  "schemaHygiene": $SCHEMA_AUTO,
  "conventionAdherence": $CONV_AUTO,
  "featureCoverage": $FEAT_AUTO
}
EOF
