---
score_cap:
  - criterion: "The deliverable satisfies the ADR-0099 document contract: solutions/netflix-margin-device-experience/ holds >=2 options/*.md, recommendation.md (Options Compared + Recommendation) and assumptions-and-evidence.md (Assumptions + Evidence), no placeholder, options and recommendation cite the evidence file"
    max_if_missing: 3
    evidence: "cd go && go run ./cmd/evolve solution check netflix-margin-device-experience --project-root .."
  - criterion: "At least two option files, each carrying a quantified effect that rests on a named assumption (an A<n> id)"
    max_if_missing: 5
    evidence: "n=0; for f in solutions/netflix-margin-device-experience/options/*.md; do grep -qE '[0-9]' \"$f\" && grep -qE '\\bA[0-9]+\\b' \"$f\" || exit 1; n=$((n+1)); done; test \"$n\" -ge 2"
  - criterion: "Every A<n>/E<n> id cited by the options and the recommendation resolves to an entry in assumptions-and-evidence.md (no dangling citation), and at least one is cited"
    max_if_missing: 5
    evidence: "D=solutions/netflix-margin-device-experience; ids=$(grep -ohE '\\b[AE][0-9]+\\b' \"$D\"/options/*.md \"$D\"/recommendation.md | sort -u); test -n \"$ids\" || exit 1; for i in $ids; do grep -qE \"(^|[^A-Za-z0-9])$i([^0-9]|$)\" \"$D\"/assumptions-and-evidence.md || exit 1; done"
  - criterion: "The recommendation names the runner-up and states the evidence that would flip the choice"
    max_if_missing: 6
    evidence: "grep -qi 'runner-up' solutions/netflix-margin-device-experience/recommendation.md && grep -qi 'flip' solutions/netflix-margin-device-experience/recommendation.md"
  - criterion: "The recommendation is about the device experience (on-topic floor)"
    max_if_missing: 6
    evidence: "grep -qi 'device' solutions/netflix-margin-device-experience/recommendation.md"
  - criterion: "H1 (cycle-1692 audit r1): Options Compared has a timing-basis row and every option's cell cites an A<n> assumption stating its lag/ramp in months, so each matrix row compares options on one stated time basis"
    max_if_missing: 5
    evidence: "D=solutions/netflix-margin-device-experience; n=$(ls \"$D\"/options/*.md | wc -l | tr -d ' '); awk -v want=\"$n\" 'FNR==NR{if($0~/^## /){inA=($0~/^## Assumptions/);cur=\"\"}else if(inA&&match($0,/^- \\*\\*A[0-9]+\\*\\*/)){cur=substr($0,RSTART+4,RLENGTH-6);t[cur]=$0}else if(inA&&cur!=\"\"){t[cur]=t[cur]\" \"$0};next} /^## /{inM=($0~/^## Options Compared/)} inM&&/^\\|/{k=split($0,c,\"|\");r++;if(r==1){nc=k-3;next};l=tolower(c[2]);if(l~/^ *-+ *$/)next;if(l~/lag|timing|time basis|start|rollout|roll-out|go-live/){found=1;for(i=3;i<k;i++){s=c[i];ok=0;while(match(s,/A[0-9]+/)){id=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);if((id in t)&&tolower(t[id])~/month/)ok=1};if(!ok){print \"RED: timing cell for option \" i-2 \" cites no month-bearing A<n> assumption:\" c[i];bad=1}}}} END{if(nc<2||nc!=want){print \"RED: Options Compared has \" nc \" option columns, options/ has \" want;exit 1};if(!found){print \"RED: Options Compared has no timing-basis row (lag/start/rollout per option)\";exit 1};exit bad}' \"$D/assumptions-and-evidence.md\" \"$D/recommendation.md\""
  - criterion: "H1: every options/*.md labels its time-to-effect (build/rollout lag) with a month-bearing A<n> assumption; no year-1 figure silently assumes a month-1 go-live"
    max_if_missing: 5
    evidence: "D=solutions/netflix-margin-device-experience; for f in \"$D\"/options/*.md; do awk 'FNR==NR{if($0~/^## /){inA=($0~/^## Assumptions/);cur=\"\"}else if(inA&&match($0,/^- \\*\\*A[0-9]+\\*\\*/)){cur=substr($0,RSTART+4,RLENGTH-6);t[cur]=$0}else if(inA&&cur!=\"\"){t[cur]=t[cur]\" \"$0};next} function chk(u,  s,id){if(tolower(u)~/time[- ]to[- ]effect/){seen=1;s=u;while(match(s,/A[0-9]+/)){id=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);if((id in t)&&tolower(t[id])~/month/)ok=1}}} /^[ \\t]*$/||/^#/||/^- /||/^[0-9]+\\. /{chk(u);u=\"\"} {u=u\" \"$0} END{chk(u);if(!seen){print \"RED: \" FILENAME \" states no time-to-effect\";exit 1};if(!ok){print \"RED: \" FILENAME \" time-to-effect cites no month-bearing A<n> assumption (build/rollout lag unlabelled)\";exit 1}}' \"$D/assumptions-and-evidence.md\" \"$f\" || bad=1; done; test -z \"$bad\""
  - criterion: "H1: every option's year-1 margin effect is strictly below its year-3 effect; each option ramps by its own assumptions (A3 24-month AV1 ramp, A7 renewals, A12 compounding plus build lag), so a year-1 cell may not restate an end state"
    max_if_missing: 5
    evidence: "D=solutions/netflix-margin-device-experience; awk -F'|' '/^## /{inM=($0~/^## Options Compared/)} inM&&/^\\|/{r++;if(r==1){nc=NF-3;next};l=tolower($2);if(l!~/effect|margin/)next;if(l~/year[- ]?1([^0-9]|$)/&&!y1){y1=1;for(i=3;i<NF;i++)a[i]=match($i,/[0-9]+\\.[0-9]+/)?substr($i,RSTART,RLENGTH):\"x\"};if(l~/year[- ]?3([^0-9]|$)/&&!y3){y3=1;for(i=3;i<NF;i++)b[i]=match($i,/[0-9]+\\.[0-9]+/)?substr($i,RSTART,RLENGTH):\"x\"}} END{if(!y1||!y3||nc<2){print \"RED: Options Compared lacks margin-effect year-1/year-3 rows\";exit 1};for(i=3;i<3+nc;i++){if(a[i]==\"x\"||b[i]==\"x\"){print \"RED: option \" i-2 \" year-1/year-3 cell has no figure\";bad=1}else if(!(a[i]+0<b[i]+0)){print \"RED: option \" i-2 \" year-1 \" a[i] \"pp is not below its year-3 \" b[i] \"pp (year 1 restates an end state)\";bad=1}};exit bad}' \"$D/recommendation.md\""
  - criterion: "H1: every year-indexed figure in the Recommendation section (+X.XXpp in/by year N) equals a sum of that year's Options Compared row, and at least one is stated, so re-timing the matrix cannot leave the +1pp portfolio arithmetic stale"
    max_if_missing: 5
    evidence: "D=solutions/netflix-margin-device-experience; awk -F'|' '/^## /{inM=($0~/^## Options Compared/);inR=($0~/^## Recommendation/)} inM&&/^\\|/{r++;if(r==1){nc=NF-3;next};l=tolower($2);if(l!~/effect|margin/)next;if(match(l,/year[- ]?[0-9]/)){y=substr(l,RSTART+RLENGTH-1,1);if(!(y in has)){has[y]=1;for(i=3;i<NF;i++)v[y,i-3]=match($i,/[0-9]+\\.[0-9]+/)?substr($i,RSTART,RLENGTH)+0:0}}} inR{txt=txt\" \"$0} END{gsub(/\\*/,\"\",txt);gsub(/[ \\t]+/,\" \",txt);s=txt;while(match(s,/[0-9]+\\.[0-9]+ ?pp (by|in|at|within) (the end of )?year[- ]?[0-9]/)){m=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);n++;x=m+0;y=substr(m,length(m),1);if(!(y in has)){print \"RED: recommendation claims \" m \" but Options Compared has no year-\" y \" margin row\";bad=1;continue};ok=0;for(q=1;q<2^nc;q++){t=0;for(j=0;j<nc;j++)if(int(q/2^j)%2)t+=v[y,j];if(t-x<=0.015&&x-t<=0.015)ok=1};if(!ok){print \"RED: recommendation figure \\\"\" m \"\\\" is not a sum of the year-\" y \" matrix row (stale after re-timing?)\";bad=1}};if(!n){print \"RED: recommendation states no year-indexed portfolio figure (+X.XXpp by year N)\";exit 1};exit bad}' \"$D/recommendation.md\""
  - criterion: "M1: the flip section states the magnitude crossover the matrix implies (A9 churn cut and A8 cohort share, each scaled by Option 2 / Option 3 year-3 effect) either as the flip threshold or as the named edge of a justified band"
    max_if_missing: 6
    evidence: "D=solutions/netflix-margin-device-experience; awk 'FNR==NR{if($0~/^## /){inA=($0~/^## Assumptions/);cur=\"\"}else if(inA&&match($0,/^- \\*\\*A[0-9]+\\*\\*/)){cur=substr($0,RSTART+4,RLENGTH-6);t[cur]=$0}else if(inA&&cur!=\"\"){t[cur]=t[cur]\" \"$0};next} /^## /{inM=($0~/^## Options Compared/);inF=(tolower($0)~/^## .*flip/)} inM&&/^\\|/{k=split($0,c,\"|\");r++;if(r==1){for(i=3;i<k;i++){if(c[i]~/Option 2/)o2=i;if(c[i]~/Option 3/)o3=i};next};l=tolower(c[2]);if(l~/effect|margin/&&l~/year[- ]?3([^0-9]|$)/&&!y3){y3=1;if(match(c[o2],/[0-9]+\\.[0-9]+/))v2=substr(c[o2],RSTART,RLENGTH)+0;if(match(c[o3],/[0-9]+\\.[0-9]+/))v3=substr(c[o3],RSTART,RLENGTH)+0}} inF{f=f\" \"$0} END{if(!o2||!o3||!(v2>0)||!(v3>0)){print \"RED: cannot read Option 2 / Option 3 year-3 margin effects from Options Compared\";exit 1};if(!match(t[\"A9\"],/[0-9]+\\.[0-9]+ ?pp/)){print \"RED: A9 cohort churn cut (pp) not found\";exit 1};cut=substr(t[\"A9\"],RSTART,RLENGTH)+0;if(!match(t[\"A8\"],/[0-9]+(\\.[0-9]+)?%/)){print \"RED: A8 cohort share (%) not found\";exit 1};sh=substr(t[\"A8\"],RSTART,RLENGTH)+0;q=v2/v3;if(q>=1){print \"RED: Option 2 year-3 (\" v2 \") >= Option 3 (\" v3 \") yet Option 3 is recommended\";exit 1};cc=cut*q;cs=sh*q;if(f==\"\"){print \"RED: no flip section\";exit 1};s=f;while(match(s,/[0-9]*\\.?[0-9]+ ?pp/)){x=substr(s,RSTART,RLENGTH)+0;s=substr(s,RSTART+RLENGTH);if(x>=cc*0.85&&x<=cc*1.15)okc=1};s=f;while(match(s,/[0-9]+(\\.[0-9]+)?%/)){x=substr(s,RSTART,RLENGTH)+0;s=substr(s,RSTART+RLENGTH);if(x>=cs*0.85&&x<=cs*1.15)oks=1};if(!okc)printf \"RED: flip section never states the churn-cut crossover %.3fpp (A9 %.2fpp x %.2f/%.2f)\\n\",cc,cut,v2,v3;if(!oks)printf \"RED: flip section never states the cohort-size crossover %.1f%% (A8 %g%% x %.2f/%.2f)\\n\",cs,sh,v2,v3;exit !(okc&&oks)}' \"$D/assumptions-and-evidence.md\" \"$D/recommendation.md\""
  - criterion: "M2: neither the recommendation nor the options claim the runner-up cannot be tested before committing or only resolves through multi-year renegotiation (A5 has a ledger read, A6 a one-partner holdout), and no confidence cell is conditioned on contracts being signed"
    max_if_missing: 6
    evidence: "D=solutions/netflix-margin-device-experience; test -s \"$D/recommendation.md\" && test -s \"$D/options/2-device-partner-economics.md\" && ! cat \"$D/recommendation.md\" \"$D\"/options/*.md | tr -s '[:space:]' ' ' | tr -d '*' | grep -oiE 'cannot be tested before|only resolves? (through|via)|can only be (proved|proven|tested|known) (over|through|via|after)|high once contracts'"
  - criterion: "M2: the Options Compared time-to-know row treats options symmetrically; every option's cell cites an A<n> whose entry declares the Probe that answers it"
    max_if_missing: 6
    evidence: "D=solutions/netflix-margin-device-experience; awk 'FNR==NR{if($0~/^## /){inA=($0~/^## Assumptions/);cur=\"\"}else if(inA&&match($0,/^- \\*\\*A[0-9]+\\*\\*/)){cur=substr($0,RSTART+4,RLENGTH-6);t[cur]=$0}else if(inA&&cur!=\"\"){t[cur]=t[cur]\" \"$0};next} /^## /{inM=($0~/^## Options Compared/)} inM&&/^\\|/{k=split($0,c,\"|\");r++;if(r==1){nc=k-3;next};l=tolower(c[2]);if(l~/time to (know|learn|prove)|probe/&&!found){found=1;for(i=3;i<k;i++){s=c[i];ok=0;while(match(s,/A[0-9]+/)){id=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);if((id in t)&&tolower(t[id])~/probe/)ok=1};if(!ok){print \"RED: time-to-know cell for option \" i-2 \" cites no A<n> whose entry declares a Probe:\" c[i];bad=1}}}} END{if(nc<2){print \"RED: no Options Compared matrix\";exit 1};if(!found){print \"RED: Options Compared has no time-to-know row\";exit 1};exit bad}' \"$D/assumptions-and-evidence.md\" \"$D/recommendation.md\""
---

# Eval: netflix-margin-device-experience (document task)

> Pins the ADR-0099 document deliverable for inbox item `netflix-margin-device-experience`
> (PR-F soak #1): strategy options to raise Netflix operating margin by 1 percentage point
> through the device experience. The `[code]` graders are the deterministic floor — the
> same `solutioncheck` engine the build handoff floor and the audit gate run, plus
> citation-resolution and runner-up/flip checks on the emitted documents. The `[model]`
> rubrics are what the auditor grades with path:line evidence; the deterministic floor is
> necessary, not sufficient (a content-free two-option fixture passes `solution check` —
> cycle-1692 TDD probe P4, `.evolve/runs/cycle-1692/test-red-output.txt`).
> Source incident: cycle 1692 is the first document cycle to reach TDD; the Task Contract
> named this file as the authority while it did not yet exist. Shape follows
> `examples/eval-solution.md`.
>
> Caps 6–12 were added by the cycle-1692 audit-repair TDD round. Audit round 1 FAILed a build
> that passed caps 1–5 and had correct arithmetic but compared options on mixed timing (H1),
> put flip thresholds off the matrix crossover (M1), and gave the runner-up a rationale its own
> probes contradicted (M2) (`.evolve/runs/cycle-1692/audit-report.md`, Issues table). These caps
> parse the Options Compared table and the `## Assumptions` entries, so they are
> consistency checks across the documents, not keyword greps. Each one is RED on the round-1
> build, GREEN on a repaired fixture, and RED on an empty tree
> (`.evolve/runs/cycle-1692/test-red-output.txt`). They assume the audited ids: Option 3
> is the recommended option (the audit says it survives the steelman), Option 2 is the runner-up,
> A8 is the cohort share and A9 the cohort churn cut.

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go run ./cmd/evolve solution check netflix-margin-device-experience --project-root ..`

## Acceptance Checks
- `[code]` `grep -qi 'device' solutions/netflix-margin-device-experience/recommendation.md`
- `[code]` `grep -qi 'runner-up' solutions/netflix-margin-device-experience/recommendation.md && grep -qi 'flip' solutions/netflix-margin-device-experience/recommendation.md`

## Model-Based Checks
- `[model]` Rubric: "Each option under options/*.md changes a DIFFERENT mechanism (cost-to-serve, ARPU via device partner economics, churn via playback quality, partner bundle placement) — not the same lever at three sizes — and states a causal chain to operating margin" — threshold: >= 70
- `[model]` Rubric: "Every quantitative claim in the options and the recommendation cites an A<n>/E<n> entry in assumptions-and-evidence.md whose source supports the magnitude and direction; unsourced numbers are labelled assumptions" — threshold: >= 80
- `[model]` Rubric: "The recommendation follows from the Options Compared matrix, names the runner-up, and states the evidence that would flip the choice" — threshold: >= 70
- `[model]` Rubric: "Every Options Compared row measures all options on one stated timing basis (each option's build/rollout lag is a labelled A<n>). The recommendation's reasons and its +1pp portfolio claim use that same basis and the stated sequencing: an option deferred to a later wave contributes only its time-shifted effect. The flip thresholds sit at the matrix crossover, or name it and justify the band with non-magnitude criteria. The runner-up's ranking rationale does not contradict the probes in assumptions-and-evidence.md" — threshold: >= 80

## Thresholds
- All `[code]` checks: pass@1 = 1.0
- `[model]` rubrics: each at or above its threshold; the auditor's steelman of the non-recommended options must not overturn the recommendation

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-floor | ADR-0099 document contract holds (AC4) | 3/10 | `evolve solution check netflix-margin-device-experience` |
| quantified-options | >=2 options, each quantified on a named A<n> assumption (AC1) | 5/10 | per-option digit + `A<n>` grep |
| citation-resolution | every cited A<n>/E<n> resolves in the evidence file (AC3) | 5/10 | cited-id ⊆ evidence-file ids |
| runner-up-flip | runner-up named, flip evidence stated (AC2) | 6/10 | `runner-up` + `flip` in recommendation.md |
| on-topic | recommendation addresses the device experience | 6/10 | `device` in recommendation.md |
| timing-basis-row (H1) | matrix row states each option's lag/ramp via a month-bearing A<n> | 5/10 | awk: table row × `## Assumptions` entries |
| time-to-effect-labelled (H1) | each option file's time-to-effect cites a month-bearing A<n> | 5/10 | awk: per-bullet unit × `## Assumptions` entries |
| year1-below-year3 (H1) | no option's year-1 cell restates its year-3 end state | 5/10 | awk: year-1 vs year-3 matrix rows, per column |
| portfolio-sums (H1) | every "+X.XXpp in/by year N" equals a subset sum of the matrix row for year N | 5/10 | awk: prose figures × subset sums |
| flip-crossover (M1) | flip section states the crossover A9×(O2/O3) pp and A8×(O2/O3) % (±15%) | 6/10 | awk: recomputed from matrix + A8/A9 |
| runner-up-no-untestable-claim (M2) | no "cannot be tested before" / "only resolve through" / "can only be proved over" / "high once contracts" | 6/10 | whitespace-normalized `grep -oiE` over recommendation + options |
| time-to-know-probes (M2) | every time-to-know cell cites an A<n> that declares a Probe | 6/10 | awk: table row × `## Assumptions` entries |

AC5 (routing plan justifies tdd not run; ship under the `solution(netflix-margin-device-experience)`
prefix) is a per-cycle pipeline property, not a property of the deliverable — it is graded by the
auditor from the cycle workspace and is deliberately NOT a permanent score cap here (a replayed
cap would read another cycle's routing plan).
