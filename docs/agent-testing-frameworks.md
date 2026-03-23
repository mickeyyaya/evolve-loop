# Agent Testing Frameworks

> Reference document for systematic testing approaches for AI agent behavior.
> Apply these techniques to verify agent correctness, detect regressions, and
> ensure reliable behavior across evolve-loop cycles.

## Table of Contents

1. [Testing Pyramid for Agents](#testing-pyramid-for-agents)
2. [Test Techniques](#test-techniques)
3. [Auto-Test Generation](#auto-test-generation)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Implementation Patterns](#implementation-patterns)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Testing Pyramid for Agents

Structure agent tests in layers. Lower layers run fast and catch most bugs; upper layers confirm end-to-end correctness.

| Layer | Scope | What It Validates | Speed | Example |
|-------|-------|-------------------|-------|---------|
| **Unit** | Individual tool call or function | Single tool produces correct output for given input | < 1s | Verify Scout returns valid JSON from a search query |
| **Integration** | Phase transitions and handoffs | Data flows correctly between Scout, Builder, and Auditor | 5-30s | Confirm Builder receives and parses Scout's `scout-report.md` |
| **E2E** | Full cycle execution | Complete Scout → Build → Audit → Ship pipeline succeeds | 1-5 min | Run a cycle against a known task and verify all artifacts exist |
| **Regression** | Behavior preservation across changes | Previously passing behaviors still pass after code changes | Varies | Re-run stored test cases from `experiments.jsonl` after a refactor |

### Layer Distribution

| Layer | Target Coverage | Failure Signal |
|-------|----------------|----------------|
| Unit | 70% of all tests | Pinpoints broken function or tool call |
| Integration | 20% of all tests | Identifies broken contracts between agents |
| E2E | 8% of all tests | Catches emergent failures invisible at lower layers |
| Regression | 2% of all tests | Guards against silent behavior drift |

---

## Test Techniques

| Technique | Description | When to Use | Implementation |
|-----------|-------------|-------------|----------------|
| **Scenario-based testing** | Define input-output pairs for known situations; assert agent produces expected behavior | Verifying happy paths and known edge cases | Write scenario files with `input`, `expected_output`, `eval_cmd` fields |
| **Property-based testing** | Assert invariants that must hold regardless of input (e.g., "output is always valid JSON") | Catching unexpected inputs that break agents | Generate random valid inputs; check structural properties of outputs |
| **Mutation testing** | Inject faults into agent prompts or tool responses; verify the test suite catches them | Measuring test suite quality | Swap tool outputs, inject malformed data, flip boolean flags |
| **Adversarial testing** | Provide intentionally misleading or ambiguous inputs; verify graceful handling | Hardening agents against real-world noise | Craft inputs with conflicting instructions, malformed context, or missing data |
| **Replay testing** | Re-execute recorded agent traces from `experiments.jsonl`; compare outputs to baselines | Regression detection after prompt or logic changes | Store full trace (inputs, tool calls, outputs); replay and diff |
| **Snapshot testing** | Capture agent output for a fixed input; flag any deviation for manual review | Detecting unintended output changes | Serialize output to a snapshot file; compare on each run |
| **Boundary testing** | Test at context window limits, token budget edges, and max tool call depth | Verifying behavior under resource constraints | Set artificially low limits; confirm graceful degradation |

---

## Auto-Test Generation

### LLM-Generated Tests (JiTTesting Pattern)

Generate test cases at build time using the same LLM that powers the agents.

| Step | Action | Output |
|------|--------|--------|
| 1 | Extract function signatures and docstrings from agent code | Function metadata list |
| 2 | Prompt the LLM to generate test inputs covering edge cases | Candidate test cases |
| 3 | Execute candidates against the implementation | Pass/fail results |
| 4 | Filter: keep tests that pass (valid behavior) and tests that reveal bugs | Curated test suite |
| 5 | Store passing tests as regression baselines | `tests/generated/` directory |

### Test Case Synthesis from Build Traces

| Source | Extraction Method | Test Type |
|--------|-------------------|-----------|
| `experiments.jsonl` | Parse each entry's `input` and `result` fields | Replay test |
| `build-report.md` | Extract file paths and eval commands | Integration test |
| `audit-report.md` | Convert audit findings into assertion checks | Regression test |
| `scout-report.md` | Use task descriptions as scenario inputs | Scenario-based test |
| Git diffs | Compare before/after states for each cycle | Snapshot test |

### Guard Against Test Pollution

| Risk | Mitigation |
|------|------------|
| LLM generates tests that encode current bugs as expected behavior | Cross-validate with a second model or manual review |
| Generated tests are too tightly coupled to prompt wording | Test behavior (output structure, side effects), not exact text |
| Test suite grows unbounded | Prune redundant tests weekly; deduplicate by coverage |

---

## Mapping to Evolve-Loop

Map each testing layer to existing evolve-loop infrastructure.

| Testing Layer | Evolve-Loop Component | Role | Location |
|---------------|----------------------|------|----------|
| **Unit** | Eval graders | Assert specific output properties via bash commands (`grep`, `test`, `jq`) | Task `eval` field in cycle workspace |
| **Integration** | `phase-gate.sh` | Validate phase transitions: Scout → Build → Audit handoffs produce required artifacts | `scripts/phase-gate.sh` |
| **E2E** | `cycle-health-check.sh` | Verify full cycle completion: all reports exist, no integrity violations, metrics recorded | `scripts/cycle-health-check.sh` |
| **Regression** | `experiments.jsonl` replay | Re-run past experiment inputs and compare to stored baselines | `experiments.jsonl` |

### Agent-Specific Test Points

| Agent | Unit Test Focus | Integration Test Focus |
|-------|----------------|----------------------|
| **Scout** | Task selection logic, priority ordering, duplicate detection | Handoff format to Builder, task metadata completeness |
| **Builder** | Code generation correctness, file creation/modification accuracy | Eval grader pass rate, artifact completeness for Auditor |
| **Auditor** | Finding detection accuracy, severity classification, false positive rate | Report format compliance, feedback integration into next cycle |

### Eval Grader as Unit Test

| Grader Property | Testing Best Practice |
|-----------------|----------------------|
| Deterministic | Run the same grader 10 times on identical input; all results must match |
| Fast | Execute in < 5s to enable rapid iteration |
| Specific | Match exact patterns, not broad substrings (see `eval-grader-best-practices.md`) |
| Independent | No dependency on external services, network, or mutable state |

---

## Implementation Patterns

### Test Harness for Agents

| Component | Purpose | Implementation |
|-----------|---------|----------------|
| **Test runner** | Execute test suites, collect results, report pass/fail | Bash script wrapping eval graders with summary output |
| **Fixture loader** | Provide deterministic inputs to agents | JSON/JSONL files with frozen inputs and expected outputs |
| **Result comparator** | Diff actual vs expected outputs | `diff`, `jq`, or custom comparator scripts |
| **Report generator** | Produce human-readable test reports | Markdown template populated by test runner |

### Mock Environments

| Mock Target | What to Replace | Strategy |
|-------------|----------------|----------|
| File system | Agent reads/writes files | Use a temporary directory; seed with fixture files; verify post-state |
| Tool responses | External tool calls (search, fetch, LLM) | Record real responses; replay from cache on subsequent runs |
| Git state | Commit history, branch state | Initialize a throwaway repo with scripted commit history |
| Context window | Model context and token budget | Set `MAX_TOKENS` environment variable to a low value |

### Deterministic Replay

| Step | Action | Detail |
|------|--------|--------|
| 1 | Record | Capture all agent inputs, tool calls, and outputs during a live run |
| 2 | Serialize | Store the full trace as a JSONL entry in `experiments.jsonl` |
| 3 | Replay | Feed recorded inputs to the agent; intercept tool calls and return recorded responses |
| 4 | Compare | Diff the replayed output against the original; flag deviations |
| 5 | Classify | Mark deviations as regression (unexpected) or improvement (intentional) |

### Test Isolation Checklist

| Requirement | Verification |
|-------------|-------------|
| No shared mutable state between tests | Each test creates its own temp directory |
| No network dependencies | All external calls are mocked or recorded |
| No ordering dependencies | Tests pass when run in any order |
| No time dependencies | Use fixed timestamps in fixtures |
| Cleanup after execution | `trap` handlers remove temp files on exit |

---

## Prior Art

| Project / Paper | Year | Key Contribution | Relevance to Evolve-Loop |
|----------------|------|-------------------|--------------------------|
| **Meta JiTTesting** | 2026 | Just-in-time test generation using LLMs during CI | Model for auto-test generation from build traces |
| **AgentEval** (Microsoft) | 2024 | Multi-dimensional evaluation framework for LLM agents | Capability vector scoring for Scout, Builder, Auditor |
| **EvalHarness** (EleutherAI) | 2023 | Standardized benchmark harness for language models | Template for deterministic replay infrastructure |
| **pytest-agent** | 2025 | pytest plugin for testing agentic workflows with mock tool calls | Pattern for unit testing individual tool invocations |
| **AgentBench** (Tsinghua) | 2024 | Benchmark suite for LLM agents across 8 environments | Multi-environment testing methodology |
| **SWE-bench** (Princeton) | 2024 | Real-world software engineering tasks as agent benchmarks | Scenario-based testing from actual codebases |
| **LATS** (Zhou et al.) | 2024 | Language Agent Tree Search with built-in evaluation | Search-based testing of agent decision paths |
| **Inspect AI** (UK AISI) | 2024 | Framework for evaluating LLM agents with tool use | Eval composition patterns for multi-step agent tasks |

---

## Anti-Patterns

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| **Testing outcomes, not behavior** | Pass/fail on final result ignores HOW the agent arrived there; masks reward hacking | Assert intermediate steps: tool call sequence, reasoning structure, artifact contents |
| **Flaky agent tests** | Non-deterministic LLM output causes tests to pass/fail randomly | Pin model temperature to 0; test structural properties, not exact text; use snapshot tolerance |
| **Missing edge cases** | Only testing happy paths leaves agents brittle in production | Add adversarial inputs, boundary conditions, and malformed data to every test suite |
| **Testing with production state** | Tests depend on live data, repos, or services that change unpredictably | Use frozen fixtures, mock environments, and recorded tool responses |
| **Overfitting tests to prompts** | Tests break whenever prompt wording changes, even if behavior is preserved | Test output structure and properties, not verbatim text |
| **No regression baseline** | Cannot detect if a change broke previously working behavior | Store passing test outputs as baselines; diff on every run |
| **Testing the test generator** | LLM generates tests that validate its own bugs as correct behavior | Cross-validate generated tests with manual review or a second model |
| **Monolithic test suites** | Single slow test suite discourages frequent testing | Split into unit (fast) and E2E (slow) suites; run unit tests on every change |
| **Ignoring test maintenance** | Stale tests accumulate, producing noise that masks real failures | Prune tests quarterly; delete tests for removed features; update fixtures |
