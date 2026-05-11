> **Agent Deployment Patterns** — Reference doc on deployment strategies for agent systems. Covers blue-green, canary, rolling, shadow, and A/B deployments with agent-specific metrics, automated rollback triggers, and mapping to the evolve-loop pipeline.

## Table of Contents

- [Deployment Strategies](#deployment-strategies)
- [Agent-Specific Metrics](#agent-specific-metrics)
- [Automated Rollback Triggers](#automated-rollback-triggers)
- [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
- [Implementation Patterns](#implementation-patterns)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Deployment Strategies

Compare deployment strategies by risk profile, operational complexity, rollback speed, and suitability for agent systems.

| Strategy | Description | Risk | Complexity | Rollback Speed | Agent Suitability |
|---|---|---|---|---|---|
| **Blue-Green** | Run two identical environments; switch traffic atomically from blue (old) to green (new) | Low | Medium | Instant (swap back) | High — swap entire agent version at once |
| **Canary** | Route a small percentage of traffic to the new version; increase gradually | Low | High | Fast (redirect traffic) | High — detect hallucination or eval regressions on a subset |
| **Rolling** | Replace instances incrementally; old and new coexist during rollout | Medium | Low | Moderate (re-deploy old) | Medium — mixed agent versions may produce inconsistent outputs |
| **Shadow/Dark** | Run new version in parallel without serving results to users; compare outputs | Very Low | High | N/A (never live) | Very High — validate agent behavior without user impact |
| **A/B Testing** | Split traffic between versions to measure behavioral differences statistically | Low | High | Fast (reassign traffic) | High — compare eval scores, user satisfaction across versions |

### Selection Guidance

| Scenario | Recommended Strategy |
|---|---|
| First deployment of a new agent role (Scout, Builder, Auditor) | Shadow/Dark, then Blue-Green |
| Prompt or system-prompt update | Canary with eval gate |
| Model version upgrade (e.g., Sonnet 4.5 to Sonnet 4.6) | Shadow/Dark, then Canary |
| Minor config or threshold change | Rolling |
| Comparing two prompt strategies | A/B Testing |

---

## Agent-Specific Metrics

Track these metrics to assess deployment health for agent systems. Standard web-service metrics (latency, error rate) are necessary but insufficient.

| Metric | Description | Measurement Method | Healthy Range |
|---|---|---|---|
| **Token Cost per Task** | Total input + output tokens consumed per completed task | Sum token counts from API responses | Within 20% of baseline |
| **Eval Pass Rate** | Percentage of eval assertions the agent passes | Run eval suite post-deploy; count passes/total | >= 90% (or >= baseline - 2%) |
| **Hallucination Rate** | Percentage of outputs containing fabricated facts | Automated fact-checking against ground truth | < 5% |
| **Safety Adherence** | Percentage of outputs passing safety classifiers | Run safety eval suite | >= 99% |
| **Latency (P50/P95)** | Time from request to final agent response | Measure end-to-end wall clock time | P95 < 2x baseline |
| **User Satisfaction** | Thumbs-up rate or CSAT score from end users | Collect explicit feedback | >= baseline - 5% |
| **Tool Call Success Rate** | Percentage of tool invocations that succeed | Log tool call outcomes | >= 95% |
| **Retry Rate** | Percentage of tasks requiring agent retry loops | Count retry events per task | < 10% |
| **Context Window Utilization** | Average percentage of context window consumed | Track token counts vs. model limit | < 80% |
| **Fitness Score Delta** | Change in fitnessScore between versions | Compare pre/post fitnessScore | >= 0 (no regression) |

---

## Automated Rollback Triggers

Define thresholds that trigger automatic rollback without human intervention. Configure these as hard gates in the deployment pipeline.

| Trigger | Threshold | Window | Action |
|---|---|---|---|
| Eval pass rate drop | > 5% below baseline | 10-minute rolling | Immediate rollback |
| Hallucination rate spike | > 10% of outputs | 5-minute rolling | Immediate rollback |
| Safety violation | Any single critical safety failure | Per-request | Immediate rollback + alert |
| Token cost explosion | > 3x baseline cost per task | 15-minute rolling | Pause canary, alert on-call |
| Latency P95 spike | > 3x baseline | 10-minute rolling | Pause rollout, investigate |
| Tool call failure rate | > 20% failures | 10-minute rolling | Immediate rollback |
| Error rate | > 5% of requests | 5-minute rolling | Immediate rollback |
| Fitness score regression | fitnessScore < previous version | Post-deploy eval | Block promotion, rollback |

### Rollback Decision Flow

1. Monitor agent-specific metrics continuously during deployment
2. Compare each metric against its threshold in the configured window
3. If any CRITICAL trigger fires (safety, hallucination) — rollback immediately
4. If any WARNING trigger fires (cost, latency) — pause rollout, alert operator
5. If all metrics remain healthy through the full canary window — promote to 100%

---

## Mapping to Evolve-Loop

Map deployment concepts to evolve-loop pipeline components.

| Deployment Concept | Evolve-Loop Equivalent | Details |
|---|---|---|
| **Deployment artifact** | `publish.sh` output | Package skill files, prompts, and config for distribution |
| **Release management** | Version bumping in `package.json` | Semantic versioning signals breaking changes to consumers |
| **Deployment health** | `fitnessScore` | Composite score from eval pass rate, token efficiency, and output quality |
| **Quality gate** | Benchmark delta check | Compare new version benchmarks against baseline; block deploy if regression detected |
| **Canary population** | Scout agent subset | Route a fraction of Scout tasks to the new version; compare reports |
| **Shadow deployment** | Parallel Builder execution | Run new Builder version alongside current; diff outputs without shipping |
| **Rollback** | Git revert + re-publish | Revert to previous tagged version and re-run `publish.sh` |
| **Staged rollout** | Scout -> Builder -> Auditor promotion | Deploy to Scout first, then Builder, then Auditor after validation at each stage |
| **Feature flag** | Gene toggles in `docs/reference/genes.md` | Enable/disable specific behaviors per agent role |
| **Monitoring dashboard** | Cycle metrics in `metrics/` | Track fitnessScore, token cost, and eval results across cycles |

### Evolve-Loop Deployment Sequence

1. Run `publish.sh` to build the deployment artifact
2. Tag the release with semantic version
3. Deploy to Scout agents first (canary)
4. Run eval suite; check fitnessScore >= baseline
5. Promote to Builder agents
6. Run integration evals; check benchmark delta
7. Promote to Auditor agents
8. Run full audit cycle; verify safety adherence
9. If any stage fails — rollback to previous tag

---

## Implementation Patterns

### Version Tagging

| Pattern | Command | Purpose |
|---|---|---|
| Semantic version tag | `git tag -a v1.2.3 -m "Release v1.2.3"` | Mark a deployable release point |
| Pre-release tag | `git tag -a v1.3.0-rc.1 -m "Release candidate 1"` | Mark a canary candidate |
| Rollback target | `git tag -a v1.2.3-rollback -m "Known-good rollback point"` | Explicit rollback destination |

### Staged Rollout Checklist

| Step | Gate | Owner |
|---|---|---|
| 1. Build artifact | All tests pass | CI pipeline |
| 2. Shadow deploy | Output diff < threshold | Automated |
| 3. Canary at 5% | Metrics within thresholds for 30 min | Automated |
| 4. Canary at 25% | Metrics within thresholds for 1 hour | Automated |
| 5. Canary at 50% | Metrics within thresholds for 2 hours | On-call engineer |
| 6. Full rollout (100%) | All gates passed | On-call engineer |

### Monitoring Dashboard Components

| Component | Data Source | Refresh Interval |
|---|---|---|
| Eval pass rate trend | Eval suite results | Per deployment |
| Token cost per task | API billing logs | 5 minutes |
| Hallucination rate | Fact-check classifier | 5 minutes |
| Fitness score history | `metrics/` directory | Per cycle |
| Rollback event log | Deployment system | Real-time |
| Agent version distribution | Load balancer config | 1 minute |

### Rollback Procedures

| Scenario | Procedure | Expected Duration |
|---|---|---|
| Blue-Green rollback | Swap traffic back to blue environment | < 1 minute |
| Canary rollback | Set canary weight to 0%; drain connections | < 2 minutes |
| Rolling rollback | Re-deploy previous version across all instances | 5-15 minutes |
| Git-based rollback | `git revert HEAD && publish.sh` | < 5 minutes |

---

## Prior Art

| System | Approach | Key Insight for Agents |
|---|---|---|
| **Kubernetes Deployments** | Rolling updates with readiness probes; automatic rollback on failed health checks | Use eval pass rate as a readiness probe equivalent |
| **Kubernetes Argo Rollouts** | Progressive delivery with analysis templates; automated canary promotion/rollback | Define analysis templates around agent-specific metrics |
| **LaunchDarkly Feature Flags** | Toggle features per user segment; instant kill switch | Use feature flags to enable/disable agent capabilities without redeployment |
| **Anthropic Model Deployment** | Staged rollout with safety evaluations at each stage; red-team testing before promotion | Run adversarial evals (red-team) before promoting agent versions |
| **OpenAI Staged Rollout** | Gradual capacity increase with monitoring; automatic throttling on anomalies | Monitor token cost and latency as early indicators of behavioral regression |
| **Istio Service Mesh** | Traffic splitting at the network layer; automatic retry and circuit breaking | Apply circuit-breaker patterns to agent tool calls |
| **Spinnaker Pipelines** | Multi-stage deployment pipelines with manual judgment gates | Insert human review gates for high-risk agent changes |

---

## Anti-Patterns

| Anti-Pattern | Risk | Mitigation |
|---|---|---|
| **Big-bang deployment** | Ship new agent version to 100% of traffic at once; no gradual validation | Use canary or blue-green; never skip staged rollout |
| **No rollback plan** | Assume the new version works; scramble when it does not | Define rollback procedures and test them before every deployment |
| **Ignoring agent-specific metrics** | Monitor only HTTP error rates; miss hallucination spikes or eval regressions | Track all metrics from the Agent-Specific Metrics table |
| **Deploying without eval gates** | Push changes without running eval suite; regressions reach users | Make eval pass rate a hard gate in the deployment pipeline |
| **Skipping shadow deployment for model changes** | Swap model versions without comparing outputs; unpredictable behavioral shifts | Always shadow-test model upgrades before live traffic |
| **Manual-only rollback** | Require human intervention for every rollback; slow response to incidents | Automate rollback triggers for critical thresholds |
| **Version sprawl** | Run too many concurrent agent versions; debugging becomes impossible | Limit to 2 concurrent versions (current + canary) |
| **Deploying Scout, Builder, and Auditor simultaneously** | All agent roles change at once; impossible to isolate regressions | Deploy one role at a time; validate before promoting the next |
| **No deployment observability** | Deploy without dashboards or alerts; discover problems from user complaints | Set up monitoring dashboard before first deployment |
| **Coupling deployment to development** | Every merge triggers a deployment; no stabilization period | Decouple CI (build/test) from CD (deploy); use explicit promotion gates |
