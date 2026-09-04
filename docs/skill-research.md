# Advertising skill research

Status: source review completed 2026-09-04. This document records how external
advertising knowledge influenced the repository skills. It is not an endorsement or a
runtime dependency.

## Source hierarchy

1. TikTok Help Center and the official Business API SDK define platform objects,
   fields, metric semantics, workflows, and current recommendations.
2. The host's typed tools and evidence determine what an active skill may claim or do.
3. External agent-skill repositories contribute organization patterns and candidate
   questions only. Their thresholds or performance claims are not imported unless an
   official source and the available account data support them.

Primary platform sources reviewed:

- [TikTok Business API SDK](https://github.com/tiktok/tiktok-business-api-sdk):
  advertiser-scoped API families and generated request models.
- [Ads Manager campaign optimization playbook](https://ads.tiktok.com/business/library/Ads_Manager_Playbook_SMB_EN.pdf):
  campaign construction, diagnosis categories, budget, bidding, and creative workflow.
- [Basic metric definitions](https://ads.tiktok.com/help/article/basic-data?lang=en):
  distinct click and conversion denominators.
- [Budget semantics](https://ads.tiktok.com/resources/help/article/budget?lang=en):
  daily versus lifetime budgets, level minimums, learning-sensitive change guidance.
- [Learning phase](https://ads.tiktok.com/help/article/learning-phase?lang=en):
  volatility and edits that can affect learning.
- [Auction delivery troubleshooting](https://ads.tiktok.com/help/article/troubleshooting-auction-ad-delivery-solutions):
  bid/budget, audience, learning, and creative as separate diagnostic branches.
- [Performance creative guidance](https://ads.tiktok.com/help/article/creative-best-practices):
  TikTok-first asset requirements and evidence needed before calling fatigue.
- [Attribution overview](https://ads.tiktok.com/help/article/attribution-overview?lang=en):
  CTA, VTA, EVTA, and attribution-window distinctions.
- [Ad-group creation](https://ads.tiktok.com/help/article/create-ad-group?lang=en) and
  [ad creation](https://ads.tiktok.com/help/article/ad-set-up?lang=en-GB): the dependency
  graph that prevents a generic one-form TikTok campaign creator.

## External skill review

GitHub adoption is a discovery signal, not evidence of advertising correctness. Star
counts below are a point-in-time GitHub API snapshot from 2026-09-04.

| Repository | Stars | What was useful | What was not imported |
| --- | ---: | --- | --- |
| [coreyhaines31/marketingskills](https://github.com/coreyhaines31/marketingskills) | 46,796 | Intent routing, input brief, decision frameworks, reference-style progressive disclosure, output checklists | Cross-platform heuristics, universal thresholds, and vendor integrations |
| [thatrebeccarae/claude-marketing](https://github.com/thatrebeccarae/claude-marketing) | 130 | TikTok workflow coverage categories: creative, audience, Spark, Shop, measurement, audit | Unsupported benchmarks and categorical performance claims |
| [Hainrixz/claude-ads](https://github.com/Hainrixz/claude-ads) | 73 | Weighted-audit structure and explicit unknown/not-applicable states | Weightings, fatigue cutoffs, and unattributed uplift claims |
| [novoads/agent-skills](https://github.com/novoads/agent-skills) | 14 | Preflight-before-deploy and paused-first lifecycle separation | Meta-specific API execution and external service coupling |

No TikTok-specific agent-skill repository found in this review had genuinely high-star
adoption. The repository therefore does not label third-party TikTok skills as
authoritative or copy them wholesale.

## Resulting skill design

Every active skill now includes:

- a discriminating purpose and evidence preconditions;
- a domain-specific decision tree or dependency chain;
- exact use of installed host tools;
- metric, attribution, and completeness rules;
- unsupported-cause and missing-data behavior;
- an output contract that exposes scope, confidence, and next action.

Staged skills contain a realistic workflow and activation gate, but remain invisible to
the runtime until their required typed tools exist. `sandbox-lifecycle` is additionally
capability-gated: it appears only when the composed AdBackend implements `Creator`.

## Deliberate exclusions

- benchmark tables without geography, objective, attribution, period, and source;
- “best practice” presented as a guaranteed causal result;
- raw API or arbitrary-JSON tools embedded in instructions;
- credential collection, direct publishing, autonomous optimization, or chat approval;
- copying a third-party skill as a dependency or silently inheriting its licenses and
  external integrations.
