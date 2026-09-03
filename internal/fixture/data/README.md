# Fixture provenance

`official-example.json` transcribes factual request values from TikTok's official SDK example
and official Postman synchronous-report request. Sources are pinned in the JSON. No credentials
or real account records are included. The published examples use placeholder IDs and do not
provide a complete, consistent multi-level delivery dataset.

`mock.json` is a **deterministic completion of those examples, not a verbatim official response**.
SDK-Campaign, APP_PROMOTION, APP_INSTALL, total budget 50 and the report end date originate
from the official examples. IDs, second campaign, group/ad relationships and delivery metrics
are explicitly synthetic. The original hourly query is retained separately; the lab expands
to daily rows for July 4–17, 2022. Purchase values are synthetic and never stand for a verified
TikTok attribution or revenue mapping. All objects are currently paused; reports are historical.

Ad/day records are the only stored performance facts. No campaign/ad-group/account totals are
handwritten: the fixture backend derives them from the ad records and tests conservation across
all four levels. Spend remains below the sample total campaign budgets. The date range is fixed;
the host reports the fixture's latest date instead of pretending it is current production data.
