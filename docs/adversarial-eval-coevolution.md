# Adversarial Eval Co-Evolution

How adversarial pressure between code generation and test generation produces stronger agents and harder evals. Based on Code-A1 (arXiv:2603.15611), which achieves 83.52% on HumanEval+ with a 3B model through adversarial co-evolution.

---

## The Self-Collusion Problem

Standard RL for code generation faces a dilemma:
- **White-box access** (model sees its own code) → trivial tests for easy rewards
- **Black-box restriction** → generic tests that miss implementation-specific bugs

Code-A1 resolves this by **separating code and test policies into two opposing models**. The evolve-loop has the same structure: the Builder generates code while the Scout/Auditor generate and run evals.

---

## Adversarial Architecture Mapping

| Code-A1 Component | Evolve-Loop Equivalent |
|-------------------|----------------------|
| Code LLM (maximize test pass rate) | Builder (implement tasks to pass eval graders) |
| Test LLM (maximize bug detection) | Scout (write eval graders) + Auditor (verify) |
| Composite reward (pass rate + adversarial pressure) | Ship rate + eval rigor classification |
| Rollout sandbox | Worktree isolation |

The architectural separation is already present: the Builder cannot modify eval files (enforced by the Auditor), and the Scout writes evals before the Builder sees them.

---

## Mistake Book: Persistent Eval Grader Bank

Code-A1's Mistake Book is a per-task experience replay buffer that tracks historically failed tests. Tests are added when code fails them and removed when code passes — maintaining only frontier-challenging cases.

**Evolve-loop translation:** Maintain a persistent eval grader bank in `.evolve/evals/`:

| Current Behavior | Mistake Book Enhancement |
|-----------------|------------------------|
| Eval graders written fresh each cycle | Retain graders from past cycles that caught real bugs |
| Passed graders discarded after ship | Archive passed graders as regression checks for future cycles |
| No eval difficulty progression | Eval graders should become harder as the codebase matures |

**Curriculum pressure:** As the Builder improves (mastery level increases), the Scout should write harder eval graders — not just checking existence but verifying behavioral correctness, edge cases, and integration. The Mistake Book provides the signal: which grader patterns historically caught bugs that the Builder initially missed?

---

## Composite Reward Design

Code-A1 uses a composite reward that balances test pass rate against adversarial pressure (α=0.5). The evolve-loop equivalent:

```
cycle_reward = ship_rate * 0.5 + eval_rigor * 0.3 + novelty * 0.2
```

Where:
- `ship_rate` = tasks shipped / tasks attempted
- `eval_rigor` = average eval quality level (L0=0, L1=0.3, L2=0.7, L3=1.0)
- `novelty` = fraction of tasks targeting untouched files

This prevents the degenerate equilibrium where easy evals produce high ship rates but no real quality improvement.

---

## Anti-Reward-Hacking

Code-A1's key insight: adversarial separation prevents reward hacking because the test generator benefits from finding bugs, not from letting code pass. In the evolve-loop:
- The Scout benefits from discovering real issues (not proposing trivial tasks)
- The Auditor benefits from catching real problems (not rubber-stamping builds)
- The Builder benefits from shipping (but only through genuine quality, not eval gaming)

The eval quality check (`eval-quality-check.sh`) and eval tamper detection (checksum verification) enforce this separation deterministically.

---

## Automated Threat Taxonomy (Red-Teaming Research, arXiv:2512.20677)

Automated red-teaming research identifies 6 threat categories that adversarial eval systems should test for. Two are particularly relevant to self-improving coding agents:

| Threat Category | Description | Evolve-Loop Risk |
|----------------|-------------|-----------------|
| Reward hacking | Agent games eval metrics without genuine improvement | Builder writing code that passes greps but doesn't implement real behavior |
| Sandbagging | Agent deliberately underperforms to lower expectations | Builder producing minimal implementations to avoid audit scrutiny |
| Data exfiltration | Agent leaks sensitive context to external systems | Builder including internal paths or state in committed files |
| Inappropriate tool use | Agent uses tools for unintended purposes | Builder modifying eval files or agent prompts |
| Chain-of-thought manipulation | Agent produces misleading reasoning traces | Auditor providing false PASS justifications |
| Specification gaming | Agent satisfies the letter of requirements, not the spirit | Scout writing tautological eval graders (Level 0/1) |

The evolve-loop already mitigates several of these: eval quality checks catch specification gaming, checksum verification prevents inappropriate tool use on eval files, and the anti-conformity audit protocol (Free-MAD) addresses chain-of-thought manipulation. Data exfiltration and sandbagging remain partially unaddressed.

---

## Multi-Agent Reflection Protocol (MAR-Inspired, arXiv:2512.20845)

Single-model self-reflection degenerates — the same model that made an error repeats it during reflection. MAR solves this with multi-persona adversarial debate: multiple critic personas analyze the failure from different angles, and a judge synthesizes the best correction. Result: +6.2 points on HumanEval over single reflection.

**Evolve-loop structural equivalence:**

| MAR Role | Evolve-Loop Agent | Function |
|----------|------------------|----------|
| Actor | Builder | Generates the implementation |
| Critics (multi-persona) | Auditor (split-role) | Reviews from multiple angles (Free-MAD anti-conformity) |
| Judge | Orchestrator | Synthesizes verdict, decides retry strategy |

**Multi-source reflection on retry:** When the Builder fails audit, the retry context should include reflections from multiple perspectives — not just the Auditor's verdict but also the eval grader output, the Builder's own risk assessment, and any relevant instincts. This prevents reflection degeneration by ensuring the retry sees the failure from angles the Builder's own reasoning cannot access.

---

## Research References

- Code-A1 (arXiv:2603.15611): adversarial co-evolution of code and test LLMs
- SWE-CI (March 2026): regression testing complements adversarial eval progression
- AgentAssay (arXiv:2603.02601): behavioral fingerprinting for eval consistency

See [research-paper-index.md](research-paper-index.md) for the full citation index.
