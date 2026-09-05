# Sandbox causal auction simulator

Status: experimental, private Sandbox model `causal-auction-v1`.

## Decision summary

The existing Sandbox hourly metric generator is replaced by one seeded causal simulator.
It models aggregate cohorts, eligibility, pacing, sampled competitor populations, a generic
auction, user actions, order value, attribution, and reporting. It does not create a second
simulation system and does not change the stable `AdBackend` report or mutation contracts.

The simulator is a behaviorally realistic abstraction for product and agent evaluation. It
is not a replica of TikTok, Meta, Amazon, or Alibaba delivery, ranking, prediction, or auction
software. Every formula not established by metric identity or budget accounting is a
replaceable modeling assumption with explicit calibration.

## Model boundary

The simulator serves local Advertiser/Manager operation and deterministic regression.
It does not use private platform formulas, reproduce a vendor simulator, or claim predictive
accuracy. Cohort distributions, auction scoring, clearing cost, fatigue and learning are
explicit modeling assumptions. Metric identities and spend caps are invariants.

## Invariants and policies

Stable invariants:

- `CTR = clicks / impressions`, `CVR = conversions / clicks`,
  `CPM = spend / impressions * 1000`, `CPC = spend / clicks`,
  `CPA = spend / conversions`, and `ROAS = revenue / spend`;
- clicks arise only after impressions, click-through conversions arise only after clicks,
  and order value arises only after a conversion;
- spend never exceeds the applicable campaign or ad-group budget;
- true outcomes and report visibility are separate;
- the same initial state, ordered actions, configuration, and seed produce the same result.

Current replaceable policies:

- multiplicative pCTR/pCVR factors;
- `log(1 + bid)` bid utility;
- multiplicative generic auction value;
- sampled competitor distributions and next-best-score clearing;
- exponential fatigue and saturation decay;
- learning exploration and partial reset thresholds;
- log-normal order values, tracking coverage, and reporting delays.

`ModelStatements()` classifies these as `PlatformInvariant`, `ModelingAssumption`, or
`CalibrationParameter`. `SimulationConfig` owns every current calibration value, including
cohorts, per-ad creative quality and affinities, competition, pacing, cost, product and
landing-page quality, learning, seasonality, tracking, attribution delay, and AOV.

## Causal pipeline

```text
Cohort opportunity
  -> eligibility
  -> paced participation
  -> advertiser value + sampled competitor population
  -> ranked auction + next-score clearing approximation
  -> impression + incremental reach
  -> click
  -> click-through or view-through conversion
  -> order value
  -> tracking coverage + attribution delay
  -> reported row
  -> derived metrics
```

Opportunity volume is `population * daily_active_rate * timestep_fraction`, modified by
daypart and a market event's traffic input. Eligibility checks the campaign/ad-group/ad
delivery path, schedule start and end in account time, geo, language, audience inclusion
and exclusion, configured ad-group placement, available placement inventory, creative
review state, product availability, and remaining campaign/ad-group budget. Invalid
persisted schedules fail closed.

Pacing changes only ad-group participation probability. For every timestep it forecasts
eligible cohort opportunities through the end of the account-local day, estimates spend at
full participation, apportions a shared campaign budget across ad groups by remaining
opportunity supply, and applies a smooth spend-lag correction. Budget never enters pCTR,
pCVR, quality, or auction score. Higher budgets can unlock participation, while limited
future supply, auction losses, and saturation create diminishing returns.

For each cohort, pCTR combines audience and creative affinity, creative quality, novelty,
fatigue, saturation, and context. pCVR combines audience affinity, purchase intent, product
quality, price attractiveness, landing-page quality, and availability. Tracking coverage
does not enter either probability.

The generic auction value is:

```text
log(1 + bid * bid_utility_scale)
* objective_predicted_value
* quality
```

For click objectives, pCTR receives an explicit calibration scale so its value is comparable
inside a mixed-objective auction. For conversion objectives, predicted value is
`pCTR * pCVR * expected_order_value`. For each opportunity sample, every eligible ad group
independently passes its pacing gate. One ad is selected within that ad group through a
seeded exploration/exploitation policy. Selected ads from overlapping ad groups and the same
sampled external competitor population are ranked together. This makes additional creatives
share one ad-group entry, while overlapping ad groups can cannibalize one another.

The strongest losing score determines a bounded clearing-cost approximation; the advertiser
does not automatically pay its maximum bid. All sampled wins are resolved before desired
spend is proportionally scaled first by shared ad-group budget and then by shared campaign
budget. Entity iteration order cannot buy inventory. These are simulation assumptions, not
reverse-engineered platform formulas.

Large cohort opportunity sets use a bounded number of explicit sampled auctions. A
largest-remainder allocation scales mutually exclusive sampled winners to the aggregate
cohort without allowing one opportunity to create multiple advertiser impressions. Clicks
and conversions use exact seeded binomial draws for small populations and a bounded normal
approximation for large populations.

## Stateful behavior

The persisted model retains advertiser-audience cohort exposure separately from
creative/cohort exposure, plus creative first-seen time, ad-group learning signals, and
daily and lifetime spend ledgers. Audience frequency determines saturation and incremental reach;
creative frequency determines fatigue and primarily changes pCTR. Replacing creative starts
a fresh creative/cohort state and renewed novelty without erasing prior audience exposure.

Learning accumulates clicks and true conversions. Low progress increases auction-score
uncertainty and blends selection toward exploration. Bid, budget, targeting, or other
ad-group fingerprint changes partially reset learning; a creative asset replacement begins
a fresh creative state. Learning does not directly add or subtract ROAS.

The causal model uses the `__simulation_v1` persistence namespace.
Campaigns, operations, approvals, and other environment state keep their existing namespaces.

Market events alter traffic, purchase intent, competitor count, and competitor bid
distributions. They never multiply CPM directly. The downstream auction may therefore show
higher volume, CVR, competition, and clearing prices together.

## True state, attribution, and reporting

Public reach is recomputed for the requested entity and date window from per-cohort
impression occupancy. It is not lifetime incremental reach, and campaign/account totals
do not sum daily or child unique reach. Uniform cohort occupancy is a modeling assumption;
it estimates overlap without reconstructing individual users.

Saved audiences retain their definitions and gate membership; unknown audience IDs never
match all cohorts. The default calibration represents US/en cohorts, not worldwide traffic.
Approved rules evaluate reported data on a half-hour virtual clock, persist targets,
conditions and results, and can pause, notify or set a policy-bounded budget. Notifications
are recorded locally, not delivered through external messaging. Balance is a shared prepaid
delivery constraint and is debited by generated spend. Lifetime budgets never reset daily.

New event sources without linked ad groups have no generated traffic. Purchase statistics
project reported attributed conversions from linked groups; they are explicitly partial,
not all website telemetry. Cart events, offline ingestion and site-wide measurement remain
unmodeled. Creating source metadata does not claim those behaviors exist.

Each new hourly fact contains private true metrics and eventual tracked metrics. Tracking
loss samples which true conversion events become observable and never changes the true
business outcome. Click-through and view-through conversions are recorded separately in the
private attribution breakdown. Recent tracked conversions and revenue remain invisible until
their attribution delay plus reporting latency expires; later report queries backfill them.
Impressions, clicks, and spend remain unchanged during that backfill.

True metrics, cohort purchase intent, competitor state, fatigue factors, and model
calibration never enter `ads.Report`, runtime context, Agent tools, SSE, or React. The full
model snapshot exists only in the Sandbox persistence payload needed for deterministic
continuation.

## Debug trace

The one-shot developer command advances virtual time while capturing an internal tracker:

```sh
./bin/ad-agent simulation-trace \
  --backend sandbox \
  --sandbox-environment simulator-lab \
  --simulation-hours 24
```

Each ad/timestep trace reports opportunities, eligible opportunities, the remaining-supply
forecast, pacing probability, participation, external and internal competition, auction
wins, budget-limited wins, win rate, clearing price, impressions, reach, pCTR, clicks, pCVR,
true and eventual reported conversions, spend, revenue, frequency, saturation, fatigue,
learning, and an eligibility reason. Trace capture is automatically disabled after the
command. The HTTP application and Agent tool registry have no trace endpoint or tool.

## Openness decision

- Appropriate audience: repository developers and deterministic evaluation code.
- Stable contract: existing `AdBackend`, report metrics, approval flow, and virtual clock.
- Experimental seam: typed `SimulationConfig`, model classifications, and developer trace.
- Private implementation: probability functions, competitor sampling, hidden state, true
  outcomes, attribution queue, and calibration values.
- Evolution: change formulas or calibration behind the Sandbox boundary; version persisted
  facts when their meaning changes. A real second simulator consumer is required before
  extracting a generic auction framework.

## Deliberately not generalized

- no TikTok/Meta ranking or delivery claims;
- no learned opportunity generator, private-data calibration, or neural model;
- no 48-agent strategic game, adaptive competitor policies, RL/bandit training, or bidder SDK;
- no arbitrary auction plugin system or public simulator protocol;
- no universal dayparting or frequency-cap contract; provider-specific eligibility semantics
  require a supported buying type and an independently validated API shape;
- no autonomous bid optimization and no new Agent write authority;
- no normal-product access to hidden state or causal traces.

## Tradeoffs and risks

Aggregate sampled auctions are fast enough for year-scale virtual time but do not preserve
individual user journeys or every counterfactual auction. Parameterized competitors create
behavioral competition but not strategic multi-agent adaptation. Whole-fact attribution
availability makes backfill discrete at hourly granularity. These are explicit MVP limits.

Calibration can make output plausible without making it predictive. Before using the
simulator for quantitative planning, parameters need public or controlled experimental
calibration, sensitivity ranges, and held-out behavioral checks. Until then, it validates
directional agent reasoning and product mechanics only.

## Validation

Fixed-seed tests cover bid monotonicity and diminishing returns, opportunity-aware pacing,
campaign/ad-group budget caps, schedule/placement/language eligibility, cross-ad-group
cannibalization, within-group creative selection, input-order independence, competition
effects on win rate and clearing cost, creative quality, landing-page isolation, purchase
intent, independently persisted fatigue and saturation, creative reset, learning reset,
seasonality, tracking loss, reporting delay/backfill, derived metrics, persistence, and
hidden-state non-disclosure. Assertions use long fixed horizons or expected aggregates rather
than one stochastic draw.
