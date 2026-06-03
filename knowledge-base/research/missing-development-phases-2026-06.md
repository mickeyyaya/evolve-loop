# Survey of Missing Development Lifecycle Phases

## Executive Summary
This survey maps the software development lifecycle (SDLC) phase taxonomies from open-source pipelines, methodology publications, and modern agentic-coding frameworks against our current Evolve Loop pipeline (which includes intent, scout, triage, architecture-design, plan-review, tdd, build-planner, build, tester, audit, ship, retrospective, and memo). We identify major functional gaps that our pipeline currently lacks or underuses and classify them for potential integration.

Our goal is to evolve the pipeline in an additive-only manner. We preserve all of the existing core phases in their current state, as they form the stable spine of our autonomous loop. Instead, we analyze how to adapt external ideas to our orchestrator/advisor framework.

## Industry Phase Taxonomies and Gaps
To analyze potential additions to the Evolve Loop, we surveyed several industry sources:
1. **Classic DevOps & DevSecOps Lifecycles**: The standard GitLab DevSecOps platform (source: https://docs.gitlab.com/ee/development/dev/) and the IBM DevOps framework (vendor doc: https://www.ibm.com/topics/devops) define standard stages such as Plan, Create, Verify, Package, Release, Configure, Monitor, and Govern. While Evolve Loop covers Plan/Create/Verify/Release well, it lacks explicit "Monitor" and "Govern" capabilities post-ship.
2. **Agentic Coding Frameworks**: Frameworks like AutoGen (source: https://microsoft.github.io/autogen/) and Spec Kit Agents (citation: arXiv:2604.05278) demonstrate the value of explicit validation hooks. In particular, they point out that agentic code generation often suffers from correlated failures where the builder and verifier share the same specification blind spots.
3. **Documentation Generation Frameworks**: Research such as DocAgent (citation: arXiv:2504.08725) and RepoAgent (citation: arXiv:2402.16667) highlights that LLM-generated code frequently suffers from compounding documentation debt.
4. **Threat Modeling & Secure SDLC**: Secure SDLC models like Harness SSDLC and ASTRIDE (citation: arXiv:2512.04785) suggest conducting threat-modeling pre-code to discover security flaws early.

## Candidate Analysis and Classification
Based on the surveyed taxonomies, we identify five candidate phases to address functional gaps in Evolve Loop:

### Specification Verification (spec-verify)
- **Gap Filled**: Correlated-failure loop prevention. Breaks the cycle where the implementation agent and the verification agent share the same specification misunderstandings.
- **Classification**: **Adopt**. We can run this after plan-review and before TDD to independently validate that the input specification/acceptance criteria are clear, unambiguous, and testable.
- **Pipeline Position**: After `triage` / `plan-review`, before `tdd`.

### Documentation Synchronization (doc-sync)
- **Gap Filled**: Compounding documentation debt. Ensures that codebase markdown documents, CHANGELOGs, and READMEs are kept in sync with code modifications.
- **Classification**: **Adopt**.
- **Pipeline Position**: After `build`, before `ship`.

### Threat Modeling (threat-model)
- **Gap Filled**: Early discovery of security vulnerabilities before code generation.
- **Classification**: **Adapt**. We will adapt this into a pre-build design guard, but defer implementation to a future cycle as it requires integration of security analysis tools.
- **Pipeline Position**: After `triage`, before `build`.

### Dependency Auditing (dependency-audit)
- **Gap Filled**: Post-build Software Composition Analysis (SCA) to detect vulnerable dependencies.
- **Classification**: **Reject**. We reject this as a standalone phase because dependency checks are better integrated as sub-steps within the existing `audit` phase rather than adding pipeline overhead.
- **Pipeline Position**: N/A.

### Post-Ship Metrics Collection (metrics-collection)
- **Gap Filled**: Dynamic production monitoring and runtime feedback.
- **Classification**: **Reject**. Reject as a pipeline phase because post-ship runtime monitoring requires persistent external infrastructure that is incompatible with our cycle-isolated, local-worktree execution model.
- **Pipeline Position**: N/A.

## Detailed Design of Selected Phases
We select **spec-verify** and **doc-sync** for implementation.
- **spec-verify**: Runs as a zero-Go specrunner phase. The agent (`evolve-spec-verifier`) reads the scout report and acceptance criteria, and evaluates if the specification has ambiguities, conflicts, or untestable checks. It outputs `spec-verify-report.md`.
- **doc-sync**: Runs as a zero-Go specrunner phase. The agent (`evolve-doc-sync`) reads the diff of modified files from the build phase, identifies modified APIs or user-facing changes, and writes/updates appropriate documentation files. It outputs `doc-sync-report.md`.

## Future Pipeline Evolution Recommendations
Any future additions to Evolve Loop should follow the strict additive-only constraint. The core phases (scout, build, audit, ship) are preserved and never omitted. We insert optional validation or enrichment phases that run asynchronously or under configuration flags to preserve the integrity floor.
