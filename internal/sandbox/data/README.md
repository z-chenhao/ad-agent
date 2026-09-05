# Local sandbox provenance

`official-example.json` transcribes factual request values from TikTok's official SDK example
and official Postman synchronous-report request. Sources are pinned in the JSON. No credentials
or real account records are included. The published examples use placeholder IDs and do not
provide a complete, consistent multi-level delivery dataset.

`environment-seed.json` is a **deterministic compatibility fixture for those examples,
not a verbatim official response**.
SDK-Campaign, APP_PROMOTION, APP_INSTALL, total budget 50 and the report end date originate
from the official examples. IDs, second campaign, group/ad relationships and delivery metrics
are explicitly synthetic. The original hourly query is retained separately; the lab expands
to daily rows for July 4–17, 2022. Purchase values are synthetic and never stand for a verified
TikTok attribution or revenue mapping. All objects are currently paused; reports are historical.

Ad/day records are the only stored performance facts. No campaign/ad-group/account totals are
handwritten: the local sandbox backend derives them from the ad records and tests conservation across
all four levels. Spend remains below the sample total campaign budgets. The date range is fixed;
the host reports the sandbox's latest date instead of pretending it is current production data.

`virtual-account.json` is the product-facing Sandbox seed. It describes Aster & Pine
Home across prospecting, retargeting, and launch work, with twelve bound ads. The simulator
creates a deterministic twenty-eight-day history through cohort opportunities, auction
outcomes, clicks, purchases, and delayed attribution. This supports two equal fourteen-day
windows. CPM, CTR, CVR and ROAS are derived, not independently generated. Cohort conversion
baselines are illustrative calibration assumptions, not measured platform benchmarks.
Creative metadata and ad-to-identity/asset bindings use the same IDs. Room-tour videos and
decor stills have matching creative names and copy, real media dimensions/durations, and
reserved `.test` destinations. Stock room footage is not labeled as customer testimony,
an unboxing, or a before/after claim. New ads reusing known assets retain their previews;
unknown assets never borrow unrelated imagery. Asset update timestamps remain stable as
the clock advances; fatigue still comes from the simulator's exposure history.
The optional operator-facing ad detail attaches local licensed stock previews from
`web/public/sandbox/creatives`; provenance and source links live beside those files.
Preview hosting remains outside CreativeAsset so it does not enter agent resource tools
or become a requirement of TikTok MAPI.

Every environment begins from the virtual account and then stores its own created and
updated entities, virtual clock, and immutable hourly facts in SQLite under the
environment identity. Environments are persistent fictional workspaces, not selectable
behavioral scenarios. Partial, denied, timed-out, rejected, and unknown outcomes are
exercised through test-only fakes and fault injection so product state remains ordinary
and composable.
