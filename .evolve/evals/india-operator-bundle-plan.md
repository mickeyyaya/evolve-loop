---
score_cap:
  - criterion: "AC5 floor: the ADR-0099 document contract holds — solutions/india-operator-bundle-plan/ has >=2 options/*.md, recommendation.md (Options Compared + Recommendation) and assumptions-and-evidence.md (Assumptions + Evidence), no placeholder, and every option and the recommendation cite the evidence file"
    max_if_missing: 3
    evidence: "cd go && go run ./cmd/evolve solution check india-operator-bundle-plan --project-root .."
  - criterion: "AC1: every option declares its deal mechanism on three axes — Who bills, Who bears acquisition cost, Entitlement delivery — and no two options share the same mechanism triple"
    max_if_missing: 5
    evidence: "D=solutions/india-operator-bundle-plan; set -- \"$D\"/options/*.md; { test -f \"$1\" && test \"$#\" -ge 2; } || { echo \"RED: need >=2 options/*.md under $D\"; exit 1; }; T=$(for f; do LC_ALL=C awk 'function norm(s){gsub(/[^a-z0-9]+/,\" \",s);gsub(/^ +| +$/,\"\",s);return s} function grab(line,lab,  l,i,n,c){l=tolower(line);if(l~/^[ \\t]*\\|/){n=split(l,c,\"|\");for(i=2;i<n;i++)if(c[i]~lab)return norm(c[i+1]);return \"\"};sub(/^[^:]*:/,\"\",l);return norm(l)} /^[ \\t]*(```|~~~)/{fz=!fz;next} fz{next} {l=tolower($0)} b==\"\"&&l~/who bills/{b=grab($0,\"who bills\")} a==\"\"&&l~/acquisition cost/{a=grab($0,\"acquisition cost\")} e==\"\"&&l~/entitlement/&&l~/deliver/{e=grab($0,\"entitlement\")} END{if(b==\"\"||a==\"\"||e==\"\")exit 1;print b\" | \"a\" | \"e}' \"$f\" || echo \"MISSING $f\"; done); if echo \"$T\" | grep -q '^MISSING '; then echo \"RED: an option does not state its Who bills / Who bears acquisition cost / Entitlement delivery mechanism:\"; echo \"$T\" | grep '^MISSING '; exit 1; fi; d=$(echo \"$T\" | sort | uniq -d); test -z \"$d\" || { echo \"RED: two options share one mechanism (who bills | acquisition cost | entitlement delivery): $d\"; exit 1; }"
  - criterion: "AC1: every option states a quantified subscription uplift (a number on a stated month/quarter/year horizon) in a unit that cites an A<n> defined under ## Assumptions"
    max_if_missing: 5
    evidence: "D=solutions/india-operator-bundle-plan; EV=\"$D/assumptions-and-evidence.md\"; test -f \"$EV\" || { echo \"RED: $EV missing\"; exit 1; }; set -- \"$D\"/options/*.md; { test -f \"$1\" && test \"$#\" -ge 2; } || { echo \"RED: need >=2 options/*.md under $D\"; exit 1; }; bad=; for f; do LC_ALL=C awk 'FNR==NR{if($0~/^#/){inA=(tolower($0)~/^##+ *assumptions/)}else if(inA&&match($0,/^[-*| \\t]*A[0-9]+/)){s=substr($0,RSTART,RLENGTH);sub(/^[^A]*/,\"\",s);defA[s]=1};next} function chk(u,  l,t,s,id){if(u==\"\")return;l=tolower(u);if(l!~/subscri|member|uplift|net add|gross add| subs[^a-z]/)return;if(l!~/month|year|quarter|fy[0-9]/)return;t=u;gsub(/[AE][0-9]+/,\"\",t);t=tolower(t);gsub(/(month|year|quarter|fy|q)[ -]*[0-9]+/,\"\",t);if(t!~/[0-9]/)return;s=u;while(match(s,/A[0-9]+/)){id=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);if(id in defA)ok=1}} /^[ \\t]*(```|~~~)/{chk(u);u=\"\";fz=!fz;next} fz{next} /^[ \\t]*$/||/^#/||/^[ \\t]*[-*] /||/^[ \\t]*[0-9]+\\. /||/^[ \\t]*\\|/{chk(u);u=\"\"} {u=u\" \"$0} END{chk(u);if(!ok){print \"RED: \" FILENAME \" states no quantified subscription uplift (a number and a horizon) resting on an A<n> defined under ## Assumptions\";exit 1}}' \"$EV\" \"$f\" || bad=1; done; test -z \"$bad\""
  - criterion: "AC2: every option has a Technical commitments section naming the integration surface, a timeline (days/weeks/months/quarters or a dated target) and who operates what"
    max_if_missing: 5
    evidence: "D=solutions/india-operator-bundle-plan; set -- \"$D\"/options/*.md; { test -f \"$1\" && test \"$#\" -ge 2; } || { echo \"RED: need >=2 options/*.md under $D\"; exit 1; }; bad=; for f; do LC_ALL=C awk 'function lv(s){match(s,/^#+/);return RLENGTH} /^[ \\t]*(```|~~~)/{fz=!fz;next} fz{next} /^#/{if(inT&&lv($0)<=L)inT=0;if(!inT&&tolower($0)~/technical commitment/){inT=1;seen=1;L=lv($0);next}} inT{t=t\" \"tolower($0)} END{if(!seen){print \"RED: \" FILENAME \" has no Technical commitments heading\";exit 1};m=\"\";if(t!~/integrat|api|sdk|billing|entitlement/)m=m\" integration-surface\";if(t!~/[0-9]+[ -]*(to|-|–)?[ -]*[0-9]*[ -]*(day|week|month|quarter)|(day|week|month|quarter)s? *[0-9]+|(by|in|from|until) (q[1-4] )?20[2-3][0-9]/)m=m\" timeline\";if(t!~/operates|operated by|owns|owned by|runs |run by|hosts|hosted by|responsible|owner/)m=m\" who-operates-what\";if(m!=\"\"){print \"RED: \" FILENAME \" technical commitments lack:\" m;exit 1}}' \"$f\" || bad=1; done; test -z \"$bad\""
  - criterion: "AC2: every option names at least two risks under a Risks heading, each carrying an early-detection signal (a Detect/signal/indicator text, or a filled signal column in a risk table)"
    max_if_missing: 5
    evidence: "D=solutions/india-operator-bundle-plan; set -- \"$D\"/options/*.md; { test -f \"$1\" && test \"$#\" -ge 2; } || { echo \"RED: need >=2 options/*.md under $D\"; exit 1; }; bad=; for f; do LC_ALL=C awk 'function lv(s){match(s,/^#+/);return RLENGTH} function flush(){if(inR&&it!=\"\"&&tolower(it)~/detect|signal|indicator|early warning|tripwire|leading/)q++;it=\"\"} /^[ \\t]*(```|~~~)/{flush();fz=!fz;next} fz{next} /^#/{flush();if(inR&&lv($0)<=L)inR=0;if(!inR&&tolower($0)~/risk/){inR=1;L=lv($0);hdr=\"\";sc=0};next} !inR{next} /^[ \\t]*\\|/{flush();n=split($0,c,\"|\");if(hdr==\"\"){hdr=$0;for(i=2;i<n;i++)if(tolower(c[i])~/detect|signal|indicator|early warning|tripwire|leading/)sc=i;next};if($0~/^[ \\t]*\\|[ \\t:|-]*$/)next;if(sc&&c[sc]!~/^[ \\t-]*$/)q++;next} /^[-*] |^ ?[0-9]+\\. /{flush();it=$0;next} /^[ \\t]*$/{next} {if(it!=\"\")it=it\" \"$0} END{flush();if(q<2){print \"RED: \" FILENAME \" names \" q+0 \" risk(s) with an early-detection signal under a Risks heading; need the top two\";exit 1}}' \"$f\" || bad=1; done; test -z \"$bad\""
  - criterion: "AC3: the Options Compared matrix has exactly one column (or one option row) per options/*.md, a runner-up sentence names an existing Option N, and the flip-the-choice evidence cites the A<n>/E<n> entry it would overturn"
    max_if_missing: 6
    evidence: "D=solutions/india-operator-bundle-plan; EV=\"$D/assumptions-and-evidence.md\"; R=\"$D/recommendation.md\"; { test -f \"$R\" && test -f \"$EV\"; } || { echo \"RED: $R or $EV missing\"; exit 1; }; n=$(ls \"$D\"/options/*.md 2>/dev/null | wc -l | tr -d ' '); test \"$n\" -ge 2 || { echo \"RED: need >=2 options/*.md under $D\"; exit 1; }; LC_ALL=C awk -v n=\"$n\" 'FNR==NR{if($0~/^#/)sec=tolower($0);else if(sec~/^##+ *(assumptions|evidence)/&&match($0,/^[-*| \\t]*[AE][0-9]+/)){s=substr($0,RSTART,RLENGTH);sub(/^[^AE]*/,\"\",s);def[s]=1};next} function lv(s){match(s,/^#+/);return RLENGTH} function unit(  i,k,sn){if(tolower(u)~/runner.up/){k=split(tolower(u),sn,/[.;!?][ \\t]+/);for(i=1;i<=k;i++)if(sn[i]~/runner.up/)ru=ru\" \"sn[i]\".\"};u=\"\"} /^[ \\t]*(```|~~~)/{unit();fz=!fz;next} fz{next} /^#/{unit();if(inF&&lv($0)<=FL)inF=0;inM=(tolower($0)~/^##+ *options compared/)} /^[ \\t]*$/||/^[ \\t]*[-*] /||/^[ \\t]*[0-9]+\\. /||/^[ \\t]*\\|/{unit()} {u=u\" \"$0} inM&&/^[ \\t]*\\|/{r++;k=split($0,c,\"|\");if(r==1){nc=k-3;tp=(tolower(c[2])~/option/)}else if(tolower(c[2])~/option/)dr++} !inF&&tolower($0)~/flip/{inF=1;FL=($0~/^#/)?lv($0):2;fl=1} inF{ft=ft\" \"$0} END{unit();if(!r){print \"RED: no table under ## Options Compared\";exit 1};if(tp?(dr!=n):(nc!=n)){print \"RED: Options Compared compares \" (tp?dr+0\" option row(s)\":nc\" option column(s)\") \"; options/ holds \" n \" option(s)\";bad=1};ok=0;s=ru;while(match(s,/option *[0-9]+/)){v=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);gsub(/[^0-9]/,\"\",v);if(v+0>=1&&v+0<=n)ok=1};if(!ok){print \"RED: no runner-up line naming Option N (1..\" n \")\";bad=1};ok=0;s=ft;while(match(s,/[AE][0-9]+/)){v=substr(s,RSTART,RLENGTH);s=substr(s,RSTART+RLENGTH);if(v in def)ok=1};if(!fl){print \"RED: recommendation states no evidence that would flip the choice\";bad=1}else if(!ok){print \"RED: the flip evidence cites no A<n>/E<n> entry it would overturn\";bad=1};exit bad}' \"$EV\" \"$R\""
  - criterion: "AC4: every A<n>/E<n> cited by the options and the recommendation resolves to an entry in the right section of assumptions-and-evidence.md (A<n> under ## Assumptions, E<n> under ## Evidence), and at least one is cited"
    max_if_missing: 5
    evidence: "D=solutions/india-operator-bundle-plan; EV=\"$D/assumptions-and-evidence.md\"; { test -f \"$EV\" && test -f \"$D/recommendation.md\"; } || { echo \"RED: $EV or $D/recommendation.md missing\"; exit 1; }; LC_ALL=C awk 'FNR==NR{if($0~/^#/)sec=tolower($0);else if(match($0,/^[-*| \\t]*[AE][0-9]+/)){s=substr($0,RSTART,RLENGTH);sub(/^[^AE]*/,\"\",s);if((s~/^A/&&sec~/^##+ *assumptions/)||(s~/^E/&&sec~/^##+ *evidence/))def[s]=1};next} /^[ \\t]*(```|~~~)/{fz=!fz;next} fz{next} {s=$0;p=\"\";while(match(s,/[AE][0-9]+/)){pre=(RSTART>1)?substr(s,RSTART-1,1):p;id=substr(s,RSTART,RLENGTH);post=substr(s,RSTART+RLENGTH,1);s=substr(s,RSTART+RLENGTH);p=substr(id,length(id),1);if(pre~/[A-Za-z0-9]/||post~/[A-Za-z0-9]/)continue;cited++;if(!(id in def)){print \"RED: \" FILENAME \":\" FNR \" cites \" id \", which is no entry under ## Assumptions (A<n>) / ## Evidence (E<n>)\";bad=1}}} END{if(!cited){print \"RED: options and recommendation cite no A<n>/E<n> entry\";exit 1};exit bad}' \"$EV\" \"$D\"/options/*.md \"$D/recommendation.md\""
  - criterion: "AC4: every quantitative claim (%, pp, x, ₹/$/INR/Rs, crore/lakh/million/M/k/bn) in the options and the recommendation cites an A<n>/E<n> in its paragraph or list item — a table cell may inherit the citation from its row label or its column header"
    max_if_missing: 6
    evidence: "D=solutions/india-operator-bundle-plan; test -f \"$D/recommendation.md\" || { echo \"RED: $D/recommendation.md missing\"; exit 1; }; LC_ALL=C awk 'function hasid(u,  s,p,pre,post,id){s=u;p=\"\";while(match(s,/[AE][0-9]+/)){pre=(RSTART>1)?substr(s,RSTART-1,1):p;id=substr(s,RSTART,RLENGTH);post=substr(s,RSTART+RLENGTH,1);s=substr(s,RSTART+RLENGTH);p=substr(id,length(id),1);if(pre!~/[A-Za-z0-9]/&&post!~/[A-Za-z0-9]/)return 1};return 0} function qty(u,  l){l=tolower(u);return (l~/[0-9][0-9,.]* *(%|pp[^a-z]|pp$|x[^a-z0-9]|x$|×)/||l~/(₹|\\$|€|inr|rs\\.?|usd) *[0-9]/||l~/[0-9][0-9,.]* *(crore|cr[^a-z]|cr$|lakh|million|mn[^a-z]|mn$|bn[^a-z]|bn$|billion|m[^a-z]|m$|k[^a-z]|k$)/)} function chk(u,f,w){if(u!=\"\"&&qty(u)&&!hasid(u)){print \"RED: \" f \":\" w \" quantitative claim cites no A<n>/E<n>:\" substr(u,1,120);bad=1}} function unit(){chk(u,uf,ul);u=\"\"} FNR==1{unit();fz=0;tr=0} /^[ \\t]*(```|~~~)/{unit();fz=!fz;next} fz{next} /^[ \\t]*\\|/{unit();tr++;n=split($0,c,\"|\");if(tr==1){delete hc;for(i=2;i<n;i++)hc[i]=c[i];next};if($0~/^[ \\t]*\\|[ \\t:|-]*$/)next;if(hasid(c[2]))next;for(i=2;i<n;i++)if(!hasid(hc[i]))chk(c[i],FILENAME,FNR);next} {tr=0} /^#/{unit();next} /^[ \\t]*$/||/^[ \\t]*[-*] /||/^[ \\t]*[0-9]+\\. /{unit()} /^[ \\t]*$/{next} {if(u==\"\"){uf=FILENAME;ul=FNR};u=u\" \"$0} END{unit();exit bad}' \"$D\"/options/*.md \"$D/recommendation.md\""
  - criterion: "AC4: every E<n> entry under ## Evidence names its source (a URL or a Source: line), so sourced numbers are distinguishable from labelled assumptions"
    max_if_missing: 6
    evidence: "EV=solutions/india-operator-bundle-plan/assumptions-and-evidence.md; test -f \"$EV\" || { echo \"RED: $EV missing\"; exit 1; }; LC_ALL=C awk 'function fl(){if(id!=\"\"&&tolower(t)!~/https?:\\/\\/|source:|sources:/){print \"RED: \" id \" under ## Evidence names no source (a URL or Source:)\";bad=1};id=\"\";t=\"\"} /^#/{fl();inE=(tolower($0)~/^##+ *evidence/);next} !inE{next} match($0,/^[-*| \\t]*E[0-9]+/){fl();s=substr($0,RSTART,RLENGTH);sub(/^[^E]*/,\"\",s);id=s;n++} id!=\"\"{t=t\" \"$0} END{fl();if(!n){print \"RED: ## Evidence holds no E<n> entry\";exit 1};exit bad}' \"$EV\""
  - criterion: "On-topic floor: the recommendation is about Netflix and Indian telecom operators"
    max_if_missing: 6
    evidence: "R=solutions/india-operator-bundle-plan/recommendation.md; grep -qi 'india' \"$R\" && grep -qiE 'operator|telecom|telco|carrier' \"$R\" && grep -qi 'netflix' \"$R\""
---

# Eval: india-operator-bundle-plan (document task)

> Pins the partnership-deal deliverable for Indian telecom operators selling Netflix subscriptions
> with Netflix-supported integration (inbox `india-operator-bundle-plan`, cycle 1695; ADR-0099
> document kind, second PR-F soak item after `netflix-margin-device-experience`). A document cycle has no
> Go predicate inventory. The deterministic floor is the `solutioncheck` engine (cap 1, the same engine
> the build handoff floor and the audit gate run). Caps 2–10 go further than that engine's shape check:
> they parse the options, the Options Compared table and the `## Assumptions` / `## Evidence`
> entries and cross-check them, so they are consistency checks across the documents, not keyword greps.
> They carry the cycle-1692 audit lesson (round 1 FAILed a build whose shape passed but whose numbers and
> matrix did not hold together; `.evolve/runs/cycle-1692/audit-report.md`) into this item's own
> acceptance. Each cap is RED on the empty tree, GREEN on a well-formed calibration fixture, and RED
> on at least one fixture built to break its criterion (`.evolve/runs/cycle-1695/test-red-output.txt`).
>
> Cap 8 is deliberately stricter than the level the precedent was accepted at. AC4 says *every*
> quantitative claim cites, so a derived number must cite its inputs in its paragraph or list item. A table cell
> may inherit the citation from its row label or column header. The shipped
> `netflix-margin-device-experience` deliverable would fail this cap on derived table cells.
>
> Authoring format the caps parse (restated in the cycle-1695 TDD handoff): options are
> `options/<N>-<name>.md` and are referred to as "Option N". Each option declares `Who bills:`,
> `Who bears acquisition cost:` and `Entitlement delivery:` as list lines or table rows, and has a
> `Technical commitments` heading and a `Risks` heading. Evidence-file entries are list items (or table rows)
> starting with the id, e.g. `- **A1** — …` under `## Assumptions` and `- **E1** — … Source: <url>`
> under `## Evidence`.

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go run ./cmd/evolve solution check india-operator-bundle-plan --project-root ..`

## Acceptance Checks
- `[code]` `grep -qi 'india' solutions/india-operator-bundle-plan/recommendation.md && grep -qi 'runner-up' solutions/india-operator-bundle-plan/recommendation.md && grep -qi 'flip' solutions/india-operator-bundle-plan/recommendation.md`

## Model-Based Checks
- `[model]` Rubric: "Each option under options/*.md changes a DIFFERENT deal mechanism: who bills (operator invoice vs Netflix billing vs wholesale), who bears subscriber-acquisition cost, and how entitlement is delivered (operator-ID sign-on, code redemption, device preload). Options are not the same lever at different sizes. Each states a causal chain from its mechanism to incremental paid Netflix subscriptions and a quantified uplift on a stated horizon that rests on a named assumption" — threshold: >= 70
- `[model]` Rubric: "Each option states the technical commitments Netflix would support: the integration surface (billing/entitlement APIs, sign-on, app preload, network-aware delivery), a timeline, and who operates what. It also names its top two risks, each with an early-detection signal observable before the damage lands" — threshold: >= 70
- `[model]` Rubric: "Every quantitative claim in the options and the recommendation cites an A<n>/E<n> entry in assumptions-and-evidence.md whose source supports the magnitude and direction. India market figures (operator subscriber bases, ARPU, published bundle precedents) are sourced. Numbers without a source are labelled assumptions" — threshold: >= 80
- `[model]` Rubric: "The recommendation follows from the Options Compared matrix, names the runner-up, and states the evidence that would flip the choice" — threshold: >= 70
- `[model]` Rubric: "Every Options Compared row compares all options on one stated horizon. An option phased into a later wave contributes only its time-shifted effect. The flip thresholds sit at the matrix crossover, or name it and justify the band. The runner-up's rationale does not contradict assumptions-and-evidence.md (cycle-1692 audit H1/M1/M2). Any option that zero-rates Netflix data or prices data differently by content addresses India's net-neutrality rules (TRAI Prohibition of Discriminatory Tariffs for Data Services Regulations, 2016) as a named risk or design constraint" — threshold: >= 80

## Thresholds
- All `[code]` checks: pass@1 = 1.0
- `[model]` rubrics: each at or above its threshold; the auditor's steelman of the non-recommended options must not overturn the recommendation

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-floor | ADR-0099 document contract holds (AC5 floor) | 3/10 | `evolve solution check india-operator-bundle-plan` |
| distinct-mechanism | each option's (who bills, who bears CAC, entitlement delivery) triple is stated and unique (AC1) | 5/10 | awk: per-option mechanism triple, `uniq -d` |
| quantified-uplift | each option: number + horizon + resolving A<n> in one unit (AC1) | 5/10 | awk: option units × `## Assumptions` ids |
| technical-commitments | each option: integration surface + timeline + who-operates (AC2) | 5/10 | awk: Technical commitments section |
| top-two-risks | each option: ≥2 risks with an early-detection signal (AC2) | 5/10 | awk: Risks section list items / signal column |
| matrix-runner-up-flip | matrix width = option count, runner-up names Option N, flip cites an A/E (AC3) | 6/10 | awk: Options Compared table + sentences |
| citation-resolution | every cited A<n>/E<n> resolves in its own section (AC4) | 5/10 | awk: cited ids ⊆ section-scoped entries |
| every-number-cites | every quantitative unit/cell cites an A<n>/E<n> (AC4) | 6/10 | awk: quantity regex × citation per unit/cell |
| evidence-sourced | every E<n> names a URL or `Source:` (AC4) | 6/10 | awk: `## Evidence` entries |
| on-topic | recommendation is about Netflix × Indian operators | 6/10 | `grep` India / operator / Netflix |

AC5's other halves are per-cycle pipeline properties, not properties of the deliverable. Those halves are: the audit report cites
path:line per criterion and per option, the routing plan justifies tdd as not run, and the ship carries the
`solution(india-operator-bundle-plan)` prefix. The auditor grades them from the cycle workspace. They are
deliberately NOT permanent score caps, because a replayed cap would read another cycle's routing plan.
