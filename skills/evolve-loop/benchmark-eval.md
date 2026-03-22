# Project Benchmark Evaluation

Persistent, multi-dimensional quality score (0-100 per dimension) that exists independently of individual tasks. Measures **project quality**, not process quality. Overall score = unweighted average of all 8 dimensions.

**Scoring formula per dimension:** `0.7 * automated_score + 0.3 * llm_score`

## Anti-Gaming Policy

1. **This file is checksummed** — Builder MUST NOT modify `benchmark-eval.md`. The orchestrator computes `sha256sum skills/evolve-loop/benchmark-eval.md` during Phase 0 and verifies it before every delta check. Tampering triggers HALT.
2. **Automated checks dominate** (70% weight) — deterministic, not gameable by LLM judgment.
3. **Monotonic ratchet** — once a dimension hits 80+, its high-water mark is tracked in `state.json.projectBenchmark.highWaterMarks`. Regression below `(HWM - 10)` triggers mandatory remediation task.
4. **External anchoring** — every 10 cycles, the Operator recalibrates thresholds against current project reality.

---

## Dimension 1: Documentation Completeness

All features documented, examples accurate, no stale references.

### Automated Checks (score 0-100)

```bash
# 1. Every skill file has frontmatter (name + description)
TOTAL_SKILLS=$(find skills/ -name "*.md" -not -name "SKILL.md" | wc -l | tr -d ' ')
WITH_FM=$(grep -rl "^---" skills/ --include="*.md" | grep -v SKILL.md | wc -l | tr -d ' ')
SKILL_FM_PCT=$((WITH_FM * 100 / (TOTAL_SKILLS > 0 ? TOTAL_SKILLS : 1)))

# 2. Every agent file has frontmatter (name + description + tools)
TOTAL_AGENTS=$(find agents/ -name "*.md" | wc -l | tr -d ' ')
AGENT_FM=$(grep -rl "^tools:" agents/ --include="*.md" | wc -l | tr -d ' ')
AGENT_FM_PCT=$((AGENT_FM * 100 / (TOTAL_AGENTS > 0 ? TOTAL_AGENTS : 1)))

# 3. README or project instructions file exists and is non-empty
# Checks for platform-agnostic instruction files: README.md, CLAUDE.md, GEMINI.md, AGENTS.md
README_EXISTS=$(test -s README.md -o -s CLAUDE.md -o -s GEMINI.md -o -s AGENTS.md && echo 100 || echo 0)

# 4. No broken internal links (markdown references to files that don't exist)
# Note: uses -oE (not -oP) for macOS compatibility
# Resolves each link relative to the SOURCE FILE's directory, not repo root
BROKEN_LINKS=0
for src in $(grep -rlE '\]\([^)]*\.md\)' skills/ agents/ docs/ 2>/dev/null); do
  srcdir=$(dirname "$src")
  for link in $(grep -oE '\]\([^)]*\.md[^)]*\)' "$src" | grep -oE '\([^)]+\)' | tr -d '()' | sed 's/#.*//'); do
    # Try relative to source file directory first, then repo root
    if [ ! -f "$srcdir/$link" ] && [ ! -f "$link" ]; then
      BROKEN_LINKS=$((BROKEN_LINKS + 1))
    fi
  done
done
LINK_SCORE=$((BROKEN_LINKS == 0 ? 100 : (BROKEN_LINKS < 3 ? 75 : (BROKEN_LINKS < 6 ? 50 : 25))))

# 5. docs/ directory exists with content
DOCS_SCORE=$(test -d docs && test "$(ls docs/*.md 2>/dev/null | wc -l)" -gt 0 && echo 100 || echo 50)

# Composite: average of all checks
DOC_AUTO=$(( (SKILL_FM_PCT + AGENT_FM_PCT + README_EXISTS + LINK_SCORE + DOCS_SCORE) / 5 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | No documentation exists |
| 25 | Some files documented but most features undocumented |
| 50 | Core features documented but examples missing or stale |
| 75 | Most features documented with accurate examples |
| 100 | All features documented, examples verified, no stale references |

---

## Dimension 2: Specification Consistency

Agents, skills, and memory-protocol agree on schemas, workflows, and field names.

### Automated Checks (score 0-100)

```bash
# Note: uses -oE (not -oP) for macOS/BSD grep compatibility throughout

# 1. All agent files referenced in SKILL.md exist
AGENT_REFS=$(grep -oE 'evolve-[a-z]+\.md' skills/evolve-loop/SKILL.md | sort -u)
MISSING_AGENTS=0
for a in $AGENT_REFS; do test -f "agents/$a" || MISSING_AGENTS=$((MISSING_AGENTS + 1)); done
AGENT_REF_SCORE=$((MISSING_AGENTS == 0 ? 100 : (100 - MISSING_AGENTS * 25)))

# 2. state.json fields mentioned in phases.md exist in memory-protocol.md schema
PHASES_FIELDS=$(grep -oE 'state\.json\.?[a-zA-Z]+' skills/evolve-loop/phases.md 2>/dev/null | sed 's/state\.json\.*//' | sort -u | head -20)
PROTO_FIELDS=$(grep -oE '"[a-zA-Z]+"' skills/evolve-loop/memory-protocol.md 2>/dev/null | tr -d '"' | sort -u)
MISSING_FIELDS=0
for f in $PHASES_FIELDS; do [ -z "$f" ] && continue; echo "$PROTO_FIELDS" | grep -qw "$f" || MISSING_FIELDS=$((MISSING_FIELDS + 1)); done
FIELD_SCORE=$((MISSING_FIELDS == 0 ? 100 : (100 - MISSING_FIELDS * 10)))

# 3. Phase numbering is sequential (no gaps or duplicates)
PHASE_NUMS=$(grep -oE 'Phase [0-9]+' skills/evolve-loop/phases.md | grep -oE '[0-9]+' | sort -n | uniq)
EXPECTED_SEQ=$(seq 0 $(echo "$PHASE_NUMS" | tail -1))
SEQ_SCORE=$(test "$(echo "$PHASE_NUMS" | tr '\n' ' ')" = "$(echo "$EXPECTED_SEQ" | tr '\n' ' ')" && echo 100 || echo 50)

# 4. Workspace file names in memory-protocol.md match what phases.md references
PROTO_FILES=$(grep -oE '[a-z]+-[a-z]+\.md' skills/evolve-loop/memory-protocol.md | sort -u)
PHASE_FILES=$(grep -oE 'workspace/[a-z]+-[a-z]+\.md' skills/evolve-loop/phases.md | sed 's|workspace/||' | sort -u)
FILE_DIFF=$(comm -23 <(echo "$PHASE_FILES") <(echo "$PROTO_FILES") | wc -l | tr -d ' ')
FILE_SCORE=$((FILE_DIFF == 0 ? 100 : (100 - FILE_DIFF * 20)))

# Composite
SPEC_AUTO=$(( (AGENT_REF_SCORE + FIELD_SCORE + SEQ_SCORE + FILE_SCORE) / 4 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | Agents reference schemas that don't exist in memory-protocol |
| 25 | Major inconsistencies between agent expectations and actual schemas |
| 50 | Core schemas consistent but edge cases diverge |
| 75 | Schemas mostly aligned, minor naming inconsistencies |
| 100 | All agents, skills, and protocols reference identical schemas and workflows |

---

## Dimension 3: Defensive Design

Error handling, rollback protocols, safety guards, input validation.

### Automated Checks (score 0-100)

```bash
# Note: all grep commands use '|| true' to prevent exit code 1 on zero matches
# Note: searches span skills/evolve-loop/ (not just phases.md) since content was split

# 1. Skill files contain rollback/revert instructions
ROLLBACK=$(grep -rci -e 'rollback' -e 'revert' -e 'git revert' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
ROLLBACK_SCORE=$((ROLLBACK >= 3 ? 100 : (ROLLBACK >= 1 ? 50 : 0)))

# 2. HALT protocol exists in operator and skill files
HALT_SKILLS=$(grep -rc 'HALT' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
HALT_OPERATOR=$(grep -c 'HALT' agents/evolve-operator.md || true)
HALT_OPERATOR=${HALT_OPERATOR:-0}
HALT_SCORE=$(( (HALT_SKILLS > 0 && HALT_OPERATOR > 0) ? 100 : 50 ))

# 3. Max retry limits defined (prevent infinite loops)
RETRY=$(grep -rci -e 'max.*3' -e '3.*attempt' -e 'max.*iteration' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
RETRY_SCORE=$((RETRY >= 2 ? 100 : (RETRY >= 1 ? 50 : 0)))

# 4. Worktree cleanup is enforced
CLEANUP=$(grep -rci -e 'worktree.*cleanup' -e 'worktree.*prune' -e 'worktree.*remove' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
CLEANUP_SCORE=$((CLEANUP >= 2 ? 100 : (CLEANUP >= 1 ? 50 : 0)))

# 5. Input validation at system boundaries (state.json validation)
VALIDATION=$(grep -rci 'validat' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
VALIDATION_SCORE=$((VALIDATION >= 2 ? 100 : (VALIDATION >= 1 ? 50 : 0)))

# Composite
DEFENSE_AUTO=$(( (ROLLBACK_SCORE + HALT_SCORE + RETRY_SCORE + CLEANUP_SCORE + VALIDATION_SCORE) / 5 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | No error handling, no rollback, no safety guards |
| 25 | Basic error handling exists but missing rollback or retry limits |
| 50 | Error handling and rollback exist but gaps in edge cases |
| 75 | Comprehensive error handling with rollback, but some paths uncovered |
| 100 | All failure modes handled, rollback tested, retry limits enforced, input validated |

---

## Dimension 4: Eval Infrastructure

Eval definitions comprehensive, mutation kill rate, self-referential coverage.

### Automated Checks (score 0-100)

```bash
# 1. eval-runner.md exists and defines execution steps
EVAL_RUNNER=$(test -f skills/evolve-loop/eval-runner.md && echo 100 || echo 0)

# 2. Eval definition format is documented with all 3 sections
EVAL_SECTIONS=$(grep -c '## Code Graders\|## Regression Evals\|## Acceptance Checks' skills/evolve-loop/eval-runner.md 2>/dev/null || true)
EVAL_SECTIONS=${EVAL_SECTIONS:-0}
EVAL_FORMAT_SCORE=$((EVAL_SECTIONS >= 3 ? 100 : (EVAL_SECTIONS * 33)))

# 3. Eval checksum mechanism exists (search all skill files)
CHECKSUM=$(grep -rc -e 'checksum' -e 'sha256' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
CHECKSUM_SCORE=$((CHECKSUM >= 2 ? 100 : (CHECKSUM >= 1 ? 50 : 0)))

# 4. Eval tamper detection documented (sum across multiple files)
TAMPER=$(grep -rci 'tamper' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
TAMPER_SCORE=$((TAMPER >= 2 ? 100 : (TAMPER >= 1 ? 50 : 0)))

# 5. Meta-cycle mutation testing documented (search skill files including phase5-learn.md)
MUTATION=$(grep -rci 'mutation' skills/evolve-loop/ 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}')
MUTATION_SCORE=$((MUTATION >= 3 ? 100 : (MUTATION >= 1 ? 50 : 0)))

# Composite
EVAL_AUTO=$(( (EVAL_RUNNER + EVAL_FORMAT_SCORE + CHECKSUM_SCORE + TAMPER_SCORE + MUTATION_SCORE) / 5 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | No eval infrastructure |
| 25 | Eval format defined but no tamper detection or mutation testing |
| 50 | Eval format + tamper detection, but no mutation testing or weak coverage |
| 75 | Full eval pipeline with mutation testing, minor gaps |
| 100 | Comprehensive evals, tamper detection, mutation testing >80% kill rate, self-referential coverage |

---

## Dimension 5: Modularity

File sizes, coupling, cohesion, no god-files.

### Automated Checks (score 0-100)

```bash
# 1. No files exceed 800 lines
OVER_800=$(find skills/ agents/ docs/ -name "*.md" -exec wc -l {} + 2>/dev/null | awk '$1 > 800 && !/total/ {count++} END {print count+0}')
SIZE_SCORE=$((OVER_800 == 0 ? 100 : (OVER_800 == 1 ? 75 : (OVER_800 <= 3 ? 50 : 25))))

# 2. phases.md line count (god-file risk)
PHASES_LINES=$(wc -l < skills/evolve-loop/phases.md | tr -d ' ')
PHASES_SCORE=$((PHASES_LINES < 400 ? 100 : (PHASES_LINES < 600 ? 75 : (PHASES_LINES < 800 ? 50 : 25))))

# 3. Agent files are focused (each under 300 lines)
AGENT_MAX=$(find agents/ -name "*.md" -exec wc -l {} + 2>/dev/null | awk '!/total/ {if ($1 > max) max=$1} END {print max+0}')
AGENT_SIZE_SCORE=$((AGENT_MAX < 200 ? 100 : (AGENT_MAX < 300 ? 75 : (AGENT_MAX < 400 ? 50 : 25))))

# 4. Separate files for separate concerns (skills/ has multiple files)
SKILL_FILES=$(find skills/evolve-loop/ -name "*.md" | wc -l | tr -d ' ')
SEPARATION_SCORE=$((SKILL_FILES >= 5 ? 100 : (SKILL_FILES >= 3 ? 75 : 50)))

# 5. No circular references between files
# Simplified: check that agents don't reference each other directly
AGENT_CROSS=$(grep -rl 'evolve-builder\|evolve-auditor\|evolve-operator' agents/evolve-scout.md 2>/dev/null | wc -l | tr -d ' ')
CIRCULAR_SCORE=$((AGENT_CROSS == 0 ? 100 : 50))

# Composite
MOD_AUTO=$(( (SIZE_SCORE + PHASES_SCORE + AGENT_SIZE_SCORE + SEPARATION_SCORE + CIRCULAR_SCORE) / 5 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | Single monolithic file, everything coupled |
| 25 | Some separation but god-files exist (>800 lines) |
| 50 | Reasonable separation but high coupling between modules |
| 75 | Good modularity, clear boundaries, minor coupling |
| 100 | High cohesion, low coupling, all files focused, no god-files |

---

## Dimension 6: Schema Hygiene

All state.json fields documented, no orphaned keys, schemas match code.

### Automated Checks (score 0-100)

```bash
# Note: uses -oE (not -oP) for macOS/BSD grep compatibility

# 1. state.json initialization in SKILL.md matches memory-protocol.md schema
INIT_FIELDS=$(grep -oE '"[a-zA-Z]+"' skills/evolve-loop/SKILL.md | head -40 | tr -d '"' | sort -u)
PROTO_SCHEMA=$(grep -oE '"[a-zA-Z]+"' skills/evolve-loop/memory-protocol.md | tr -d '"' | sort -u)
INIT_MISSING=0
for f in $INIT_FIELDS; do echo "$PROTO_SCHEMA" | grep -qw "$f" || INIT_MISSING=$((INIT_MISSING + 1)); done
INIT_SCORE=$((INIT_MISSING == 0 ? 100 : (100 - INIT_MISSING * 10)))

# 2. All state.json fields used in phases.md are documented in memory-protocol.md
# (reuse from Dimension 2, check 2)

# 3. No duplicate field definitions in memory-protocol.md
DUP_FIELDS=$(grep -oE '^ *"[a-zA-Z]+"' skills/evolve-loop/memory-protocol.md | sort | uniq -d | wc -l | tr -d ' ')
DUP_SCORE=$((DUP_FIELDS == 0 ? 100 : (100 - DUP_FIELDS * 20)))

# 4. JSON examples in memory-protocol.md are syntactically valid (spot check)
JSON_BLOCKS=$(grep -c '```json' skills/evolve-loop/memory-protocol.md || true)
JSON_BLOCKS=${JSON_BLOCKS:-0}
JSON_SCORE=$((JSON_BLOCKS >= 3 ? 100 : (JSON_BLOCKS >= 1 ? 75 : 50)))

# 5. Layer numbering is sequential
LAYERS=$(grep -oE 'Layer [0-9]+' skills/evolve-loop/memory-protocol.md | grep -oE '[0-9]+' | sort -n)
LAYER_SEQ_SCORE=$(test "$(echo $LAYERS | xargs)" = "0 1 2 3 4 5 6" && echo 100 || echo 50)

# Composite
SCHEMA_AUTO=$(( (INIT_SCORE + DUP_SCORE + JSON_SCORE + LAYER_SEQ_SCORE) / 4 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | No schema documentation, fields undefined |
| 25 | Some fields documented but many orphaned or undocumented |
| 50 | Core schema documented, some fields missing or inconsistent |
| 75 | Schema well-documented, minor gaps |
| 100 | All fields documented, no orphans, schemas match code, JSON valid |

---

## Dimension 7: Convention Adherence

Naming, file organization, commit format consistency.

### Automated Checks (score 0-100)

```bash
# 1. Agent files follow naming convention (evolve-<role>.md)
AGENT_NAMING=$(find agents/ -name "evolve-*.md" | wc -l | tr -d ' ')
TOTAL_AGENT_FILES=$(find agents/ -name "*.md" | wc -l | tr -d ' ')
NAMING_SCORE=$((TOTAL_AGENT_FILES > 0 ? (AGENT_NAMING * 100 / TOTAL_AGENT_FILES) : 100))

# 2. Skill files use kebab-case naming (SKILL.md excluded — conventional name like README.md)
NON_KEBAB=$(find skills/evolve-loop/ -name "*.md" ! -name "SKILL.md" | grep '[A-Z_]' | wc -l | tr -d ' ')
KEBAB_SCORE=$((NON_KEBAB == 0 ? 100 : (100 - NON_KEBAB * 20)))

# 3. Recent commits follow conventional format (type: description)
CONVENTIONAL=$(git log --oneline -10 | grep -cE '^[a-f0-9]+ (feat|fix|refactor|docs|test|chore|perf|ci):' || true)
CONVENTIONAL=${CONVENTIONAL:-0}
COMMIT_SCORE=$((CONVENTIONAL * 10))

# 4. Directory structure follows conventions
DIR_SCORE=$(test -d skills/evolve-loop -a -d agents -a -d docs && echo 100 || echo 50)

# 5. Markdown headers use consistent style (## not setext == or --)
# Note: matches 4+ chars to avoid false positives on '---' horizontal rules/frontmatter
WRONG_HEADERS=$(grep -rn -E '^={4,}$|^-{4,}$' skills/ agents/ 2>/dev/null | wc -l | tr -d ' ')
HEADER_SCORE=$((WRONG_HEADERS == 0 ? 100 : (100 - WRONG_HEADERS * 10)))

# Composite
CONV_AUTO=$(( (NAMING_SCORE + KEBAB_SCORE + COMMIT_SCORE + DIR_SCORE + HEADER_SCORE) / 5 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | No consistent conventions, mixed naming styles |
| 25 | Some conventions but inconsistently applied |
| 50 | Core conventions followed, edge cases diverge |
| 75 | Strong convention adherence, minor deviations |
| 100 | All naming, organization, and commit conventions followed consistently |

---

## Dimension 8: Feature Coverage

All documented phases/features implemented, no dead code.

### Automated Checks (score 0-100)

```bash
# 1. All 5 phases referenced in phases.md
PHASES_DEFINED=$(grep -cE '### Phase [0-9]+:' skills/evolve-loop/phases.md || true)
PHASES_DEFINED=${PHASES_DEFINED:-0}
PHASE_COV_SCORE=$((PHASES_DEFINED >= 5 ? 100 : (PHASES_DEFINED * 20)))

# 2. All agent files exist that SKILL.md references
SKILL_AGENTS=$(grep -oE 'evolve-[a-z]+\.md' skills/evolve-loop/SKILL.md | sort -u)
MISSING=0
for a in $SKILL_AGENTS; do test -f "agents/$a" || MISSING=$((MISSING + 1)); done
AGENT_COV_SCORE=$((MISSING == 0 ? 100 : (100 - MISSING * 25)))

# 3. All referenced support files exist (eval-runner.md, memory-protocol.md, etc.)
SUPPORT_FILES="skills/evolve-loop/eval-runner.md skills/evolve-loop/memory-protocol.md skills/evolve-loop/phases.md"
SUP_MISSING=0
for f in $SUPPORT_FILES; do test -f "$f" || SUP_MISSING=$((SUP_MISSING + 1)); done
SUPPORT_SCORE=$((SUP_MISSING == 0 ? 100 : (100 - SUP_MISSING * 33)))

# 4. publish.sh exists and is executable
PUBLISH_SCORE=$(test -x publish.sh && echo 100 || (test -f publish.sh && echo 50 || echo 0))

# 5. docs/ files are not empty stubs
DOC_STUBS=0
for f in docs/*.md; do test -f "$f" && test "$(wc -l < "$f" | tr -d ' ')" -lt 5 && DOC_STUBS=$((DOC_STUBS + 1)); done 2>/dev/null
STUB_SCORE=$((DOC_STUBS == 0 ? 100 : (100 - DOC_STUBS * 20)))

# Composite
FEAT_AUTO=$(( (PHASE_COV_SCORE + AGENT_COV_SCORE + SUPPORT_SCORE + PUBLISH_SCORE + STUB_SCORE) / 5 ))
```

### LLM Judgment Rubric

| Score | Criteria |
|-------|----------|
| 0 | Most documented features unimplemented |
| 25 | Core pipeline works but many documented features missing |
| 50 | Main features implemented, some documented features missing |
| 75 | Most features implemented, minor gaps |
| 100 | All documented phases/features implemented, no dead code, no stub files |

---

## Composite Score Calculation

```
overall = (DOC_AUTO*0.7 + DOC_LLM*0.3
         + SPEC_AUTO*0.7 + SPEC_LLM*0.3
         + DEFENSE_AUTO*0.7 + DEFENSE_LLM*0.3
         + EVAL_AUTO*0.7 + EVAL_LLM*0.3
         + MOD_AUTO*0.7 + MOD_LLM*0.3
         + SCHEMA_AUTO*0.7 + SCHEMA_LLM*0.3
         + CONV_AUTO*0.7 + CONV_LLM*0.3
         + FEAT_AUTO*0.7 + FEAT_LLM*0.3) / 8
```

Round each dimension composite to nearest integer. Round overall to 1 decimal place.

## Task-Type-to-Dimension Mapping

Used by the delta check to determine which dimensions to re-evaluate after a task:

| Task Type | Relevant Dimensions |
|-----------|-------------------|
| `feature` | Documentation Completeness, Feature Coverage, Modularity |
| `stability` | Defensive Design, Eval Infrastructure |
| `security` | Defensive Design, Convention Adherence |
| `techdebt` | Modularity, Schema Hygiene, Convention Adherence |
| `performance` | Modularity, Feature Coverage |
| `meta` | Specification Consistency, Eval Infrastructure, Schema Hygiene |

---

## Domain-Specific Dimension Examples

The 8 coding dimensions above are the **default benchmark** and are always used when `projectContext.domain == "coding"`. For other domains, some dimensions should be replaced with domain-appropriate equivalents. The scoring formula (70% automated + 30% LLM) and anti-gaming policy remain identical.

### Writing Domain

When `projectContext.domain == "writing"`, replace 4 of the 8 dimensions:

| Coding Dimension (replaced) | Writing Dimension | What it measures |
|------------------------------|-------------------|-----------------|
| Modularity | **Prose Clarity** | Readability scores, sentence complexity, jargon density |
| Schema Hygiene | **Structural Coherence** | Consistent heading hierarchy, logical flow, no orphaned sections |
| Convention Adherence | **Style Consistency** | Voice, tone, terminology consistency across documents |
| Feature Coverage | **Content Completeness** | All topics covered, no placeholder/TODO sections remaining |

**Prose Clarity automated checks** (example):
```bash
# Average sentence length (target: 15-25 words)
AVG_SENT=$(cat articles/*.md | tr '.' '\n' | awk '{print NF}' | awk '{sum+=$1; n++} END{print sum/n}')
# Heading-to-paragraph ratio (at least 1 heading per 500 words)
HEADINGS=$(grep -c "^#" articles/*.md)
```

**Prose Clarity LLM rubric:**
| Score | Criteria |
|-------|----------|
| 0 | Incomprehensible, no structure |
| 25 | Readable but dense, requires re-reading |
| 50 | Clear enough but inconsistent quality |
| 75 | Consistently clear, well-structured |
| 100 | Publication-ready, engaging, precise |

Retained from coding: Documentation Completeness, Specification Consistency, Defensive Design (error handling in guides), Eval Infrastructure.

### Research Domain

When `projectContext.domain == "research"`, replace 4 dimensions:

| Coding Dimension (replaced) | Research Dimension | What it measures |
|------------------------------|-------------------|-----------------|
| Feature Coverage | **Claim Accuracy** | Factual claims supported by cited sources, no hallucinated findings |
| Convention Adherence | **Source Coverage** | All research questions addressed, key sources consulted |
| Modularity | **Methodology Rigor** | Clear research questions, systematic approach, reproducible steps |
| Schema Hygiene | **Citation Hygiene** | All citations formatted consistently, no broken references, sources accessible |

**Claim Accuracy automated checks** (example):
```bash
# Count claims with citations vs total claims
TOTAL_CLAIMS=$(grep -c '\.' findings/*.md)
CITED_CLAIMS=$(grep -c '\[.*\]' findings/*.md)
ACCURACY_PCT=$((CITED_CLAIMS * 100 / TOTAL_CLAIMS))
```

Retained from coding: Documentation Completeness, Specification Consistency, Defensive Design, Eval Infrastructure.

### How Domains Select Dimensions

During Phase 0 CALIBRATE, the orchestrator checks `projectContext.domain` and loads the appropriate dimension set:
- `coding` → all 8 default dimensions (this file, unchanged)
- `writing` → 4 writing-specific + 4 retained coding dimensions
- `research` → 4 research-specific + 4 retained coding dimensions
- `design` → define design-specific dimensions when design domain is implemented
- `mixed` → use coding dimensions (broadest coverage, most automated checks available)

The dimension names and automated checks change per domain, but the scoring formula, composite calculation, delta check mechanics, and anti-gaming policy are universal.
