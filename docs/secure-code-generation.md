# Secure Code Generation

How the evolve-loop addresses security in AI-generated code, and why static analysis alone is insufficient. Based on research showing CodeQL is only 61% accurate and Semgrep only 65% on individual vulnerability samples (arXiv:2602.05868).

---

## The Static Analysis Reliability Problem

Traditional security gates rely on static analyzers (CodeQL, Semgrep, Bandit) as authoritative quality signals. Research (arXiv:2602.05868, arXiv:2503.15554) reveals critical limitations:

| Tool | Accuracy on Vulnerability Detection | Limitation |
|------|-------------------------------------|-----------|
| CodeQL | 61% | High false negative rate on novel patterns |
| Semgrep | 65% | Rule-based, misses semantic vulnerabilities |
| Bandit (Python) | ~70% | Language-specific, narrow scope |

**Implication for evolve-loop eval graders:** SecLayer-L2 eval graders that rely solely on static analysis output provide a false sense of security. A grader that runs `semgrep --config=auto` and checks exit code 0 only catches 65% of vulnerabilities — 35% pass through undetected.

---

## Persistent Feedback Loop Pattern

The validated alternative (arXiv:2602.05868): combine static analysis with a persistent feedback RAG loop. When a vulnerability is found:
1. Classify the vulnerability type and affected code pattern
2. Store the pattern as a searchable entry in a knowledge base
3. Before generating new code, retrieve relevant vulnerability patterns
4. Apply defensive coding practices proactively, not just reactively

**Evolve-loop mapping:** This maps directly to the instinct and gene system:

| Feedback Loop Step | Evolve-Loop Mechanism |
|-------------------|----------------------|
| Classify vulnerability | Auditor flags security finding with type |
| Store pattern | Extract instinct: "when editing auth code, check for X" |
| Retrieve before generation | Builder reads instinctSummary before implementing |
| Apply proactively | Gene with selector matching security-sensitive files |

---

## Security Eval Grader Best Practices

Given static analyzer limitations, security eval graders should be **layered**:

| Security Detection Layer | Technique | What It Catches |
|--------------------------|-----------|----------------|
| SecLayer-L1 | `grep` for known-bad patterns (hardcoded secrets, eval()) | 40% — obvious issues |
| SecLayer-L2 | Static analyzer (semgrep, bandit) | 65% — known vulnerability patterns |
| SecLayer-L3 | Behavioral test (run code, check for unsafe behavior) | 80%+ — runtime security |
| SecLayer-L4 | Instinct-based review (prior vulnerability patterns) | 90%+ — domain-specific issues |

> **Disambiguation:** Security Detection Layers (SecLayer-L1 through L4) measure vulnerability detection rate by technique. They are distinct from Eval Rigor Levels (Rigor-L0 through L3) which measure eval grader quality — see [adversarial-eval-coevolution.md](adversarial-eval-coevolution.md).

**Recommendation:** Security-sensitive tasks (touching auth, input handling, API endpoints) should require SecLayer-L3+ eval graders, not just SecLayer-L1/L2 string matching.

---

## Research References

- "Persistent Feedback for Secure Code" (arXiv:2602.05868): static analyzer accuracy benchmarks
- "Comprehensive Study of LLM Secure Code Generation" (arXiv:2503.15554): vulnerability taxonomy
- SecureAgentBench (arXiv:2509.22097): secure code generation benchmark

See [research-paper-index.md](research-paper-index.md) for the full citation index.
