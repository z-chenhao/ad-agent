package sandbox

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestHierarchyRollups(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var canonical ads.Metrics
	for _, level := range []ads.Level{ads.Ad, ads.AdGroup, ads.Campaign, ads.Advertiser} {
		r, err := b.Report(ctx, ads.ReportQuery{Level: level, Start: "2026-08-28", End: "2026-09-03"})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Complete {
			t.Fatal("incomplete sandbox")
		}
		if !r.Totals.Spend.IsPositive() || r.Totals.Revenue == nil || !r.Totals.Revenue.IsPositive() {
			t.Fatalf("bad sums at %s: %+v", level, r.Totals)
		}
		if level == ads.Ad {
			canonical = r.Totals
		}
		a, _ := json.Marshal(canonical)
		v, _ := json.Marshal(r.Totals)
		if string(a) != string(v) {
			t.Fatalf("cross-level mismatch at %s", level)
		}
		c, err := ads.Analyze(r)
		if err != nil {
			t.Fatal(err)
		}
		if c.ROAS == nil || !c.ROAS.Equal(*r.Totals.ROAS()) {
			t.Fatal("weighted ratio incorrect")
		}
	}
	prev, _ := b.Report(ctx, ads.ReportQuery{Level: ads.Campaign, Start: "2026-08-21", End: "2026-08-27"})
	cur, _ := b.Report(ctx, ads.ReportQuery{Level: ads.Campaign, Start: "2026-08-28", End: "2026-09-03"})
	comp, err := ads.Compare(prev, cur)
	if err != nil {
		t.Fatal(err)
	}
	if comp.PreviousROAS == nil || comp.CurrentROAS == nil || len(comp.Contributions) != 3 {
		t.Fatalf("unexpected comparison %+v", comp)
	}
	sum := mustDecimal("0")
	for _, c := range comp.Contributions {
		sum = sum.Add(*c.ROASPoints)
	}
	if sum.Sub(*comp.DeltaROAS).Abs().GreaterThan(decimal.RequireFromString("0.000000000000001")) {
		t.Fatalf("contributions do not sum: contributions=%s delta=%s difference=%s", sum, comp.DeltaROAS, sum.Sub(*comp.DeltaROAS).Abs())
	}
	for _, level := range []ads.Level{ads.Campaign, ads.AdGroup} {
		entities, _ := b.List(ctx, ads.EntityQuery{Level: level})
		for _, e := range entities {
			children := ads.AdGroup
			if level == ads.AdGroup {
				children = ads.Ad
			}
			list, _ := b.List(ctx, ads.EntityQuery{Level: children, ParentID: e.ID})
			if len(list) == 0 {
				t.Fatal("orphan parent")
			}
		}
	}
}

func TestProductReportsKeepSourceAndEvidenceGapsWithoutSimulatorDocumentation(t *testing.T) {
	b, err := NewEnvironment("copy-review")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	account, err := b.Account(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if account.Source.Backend != "sandbox" || account.Source.Environment != "copy-review" || len(account.Limitations) != 0 {
		t.Fatalf("source must remain explicit without generic account caveats: %+v", account)
	}
	report, err := b.Report(ctx, ads.ReportQuery{Level: ads.Campaign, Start: "2026-08-28", End: "2026-09-03"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Source != account.Source || report.Attribution == "" || !report.Complete || len(report.Limitations) != 0 {
		t.Fatalf("complete report contains generic caveats or lost metadata: %+v", report)
	}
	partial, err := b.Report(ctx, ads.ReportQuery{Level: ads.Campaign, Start: "2026-01-01", End: "2026-01-02"})
	if err != nil {
		t.Fatal(err)
	}
	if partial.Complete || len(partial.Limitations) == 0 {
		t.Fatal("missing data must still be qualified")
	}
}

func TestOfficialExampleFieldsArePreserved(t *testing.T) {
	raw, err := files.ReadFile("data/official-example.json")
	if err != nil {
		t.Fatal(err)
	}
	var original struct {
		Campaign struct {
			AdvertiserID string `json:"advertiser_id"`
			Name         string `json:"campaign_name"`
			Budget       int64  `json:"budget"`
			Mode         string `json:"budget_mode"`
			Objective    string `json:"objective_type"`
		} `json:"campaign_create"`
	}
	if err = json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	seed, err := files.ReadFile("data/environment-seed.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := FromJSON(seed)
	if err != nil {
		t.Fatal(err)
	}
	entity, err := b.Get(context.Background(), ads.Campaign, "campaign_example_1")
	if err != nil {
		t.Fatal(err)
	}
	if entity.AccountID != original.Campaign.AdvertiserID || entity.Name != original.Campaign.Name || entity.Budget.IntPart() != original.Campaign.Budget || entity.BudgetMode != original.Campaign.Mode || entity.Objective != original.Campaign.Objective {
		t.Fatal("sandbox silently changed official request fields")
	}
}
func TestMissingDayIsNotZero(t *testing.T) {
	raw, _ := files.ReadFile("data/environment-seed.json")
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc.Report.Data.List = doc.Report.Data.List[1:]
	raw, _ = json.Marshal(doc)
	b, err := FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.Report(context.Background(), ads.ReportQuery{Level: ads.Advertiser, Start: "2022-07-04", End: "2022-07-17"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Complete {
		t.Fatal("missing day falsely complete")
	}
	if _, err = ads.Analyze(r); err == nil {
		t.Fatal("ranked incomplete report")
	}
}
func TestSandboxRejectsContradictions(t *testing.T) {
	raw, _ := files.ReadFile("data/environment-seed.json")
	for _, modify := range []func(*Document){
		func(d *Document) { d.Ads.Data.List[0].AdvertiserID = "other" },
		func(d *Document) { d.Ads.Data.List[0].AdGroupID = "missing" },
		func(d *Document) { d.Report.Data.List = append(d.Report.Data.List, d.Report.Data.List[0]) },
		func(d *Document) { d.Report.Data.List[0].Metrics.Clicks = 999999 },
		func(d *Document) { v := mustDecimal("-1"); d.Report.Data.List[0].Metrics.Revenue = &v },
	} {
		var d Document
		json.Unmarshal(raw, &d)
		modify(&d)
		b, _ := json.Marshal(d)
		if _, err := FromJSON(b); err == nil {
			t.Fatal("accepted inconsistent sandbox")
		}
	}
	b, _ := New()
	if _, e := b.Report(context.Background(), ads.ReportQuery{Level: ads.Advertiser, EntityID: "other", Start: "2026-08-28", End: "2026-09-03"}); e == nil {
		t.Fatal("cross-account query accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := b.Account(ctx); e == nil {
		t.Fatal("cancel ignored")
	}
}

func TestCreateEnforcesHierarchyAndLevelFields(t *testing.T) {
	b, err := NewEnvironment("validation")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err = b.Create(ctx, ads.CreateRequest{Level: ads.Ad, ParentID: "adgroup_broad_us", Name: "Invalid budget", Status: "DISABLE", Budget: decimalPtr("10"), BudgetMode: "BUDGET_MODE_DAY"}); err == nil {
		t.Fatal("ad accepted budget fields")
	}
	if _, err = b.Create(ctx, ads.CreateRequest{Level: ads.AdGroup, ParentID: "missing", Name: "Orphan", Status: "DISABLE"}); err == nil {
		t.Fatal("ad group accepted missing parent")
	}
	campaign, err := b.Create(ctx, ads.CreateRequest{Level: ads.Campaign, Name: "Valid", Status: "DISABLE", Objective: "TRAFFIC"})
	if err != nil {
		t.Fatal(err)
	}
	group, err := b.Create(ctx, ads.CreateRequest{Level: ads.AdGroup, ParentID: campaign.ID, Name: "Valid group", Status: "DISABLE"})
	if err != nil {
		t.Fatal(err)
	}
	ad, err := b.Create(ctx, ads.CreateRequest{Level: ads.Ad, ParentID: group.ID, Name: "Valid ad", Status: "DISABLE"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := b.Get(ctx, ads.Ad, ad.ID); err != nil || got.ParentID != group.ID {
		t.Fatalf("created ad=%#v err=%v", got, err)
	}
}

func decimalPtr(value string) *decimal.Decimal {
	v := decimal.RequireFromString(value)
	return &v
}

func TestHourlySimulationObservesDeliveryStateAndCompleteness(t *testing.T) {
	b, err := NewEnvironment("hourly")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, target := range []struct {
		level ads.Level
		id    string
	}{{ads.Campaign, "campaign_prospect_us"}, {ads.AdGroup, "adgroup_broad_us"}, {ads.Ad, "ad_prospect_creator"}} {
		entity, getErr := b.Get(ctx, target.level, target.id)
		if getErr != nil {
			t.Fatal(getErr)
		}
		outcome := b.Write(ctx, ads.WriteRequest{Target: entity, Kind: "status", Status: "ENABLE"})
		if outcome.State != "acknowledged" {
			t.Fatalf("enable %s: %#v", target.id, outcome)
		}
	}
	first, facts, err := b.Advance(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.AdvancedBy != 1 || first.FactsCreated != 12 || len(facts) != 12 || !first.State.CurrentTime.Equal(b.simulationStart.Add(time.Hour)) {
		t.Fatalf("unexpected first advance: %#v facts=%d", first, len(facts))
	}
	report, err := b.Report(ctx, ads.ReportQuery{Level: ads.Advertiser, Start: "2026-09-04", End: "2026-09-04"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Complete || report.Totals.Spend.IsZero() {
		t.Fatalf("partial hour should be incomplete with generated spend: %#v", report)
	}
	second, _, err := b.Advance(ctx, 23)
	if err != nil {
		t.Fatal(err)
	}
	if second.State.GeneratedHours != 24 || second.State.FactCount != 288 {
		t.Fatalf("unexpected simulation counters: %#v", second.State)
	}
	report, err = b.Report(ctx, ads.ReportQuery{Level: ads.Advertiser, Start: "2026-09-04", End: "2026-09-04"})
	if err != nil || !report.Complete || len(report.Rows) != 1 {
		t.Fatalf("full simulated day should be complete: report=%#v err=%v", report, err)
	}
}

func TestHourlySimulationIsDeterministic(t *testing.T) {
	left, _ := NewEnvironment("same")
	right, _ := NewEnvironment("same")
	different, _ := NewEnvironment("different")
	for _, backend := range []*Backend{left, right, different} {
		enableDeliveryPath(t, backend)
	}
	_, leftFacts, leftErr := left.Advance(context.Background(), 3)
	_, rightFacts, rightErr := right.Advance(context.Background(), 3)
	_, differentFacts, differentErr := different.Advance(context.Background(), 3)
	leftJSON, _ := json.Marshal(leftFacts)
	rightJSON, _ := json.Marshal(rightFacts)
	if leftErr != nil || rightErr != nil || string(leftJSON) != string(rightJSON) {
		t.Fatalf("same environment diverged: %v %v", leftErr, rightErr)
	}
	differentJSON, _ := json.Marshal(differentFacts)
	if differentErr != nil || string(leftJSON) == string(differentJSON) {
		t.Fatalf("different environment reused delivery stream: %v", differentErr)
	}
}

func TestSimulationChangesAffectOnlyFutureFacts(t *testing.T) {
	b, err := NewEnvironment("future-only")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	enableDeliveryPath(t, b)
	if _, _, err = b.Advance(ctx, 24); err != nil {
		t.Fatal(err)
	}
	dayOneBefore := sandboxDailyReport(t, b, "2026-09-04")
	beforeGroup, err := b.Report(ctx, ads.ReportQuery{Level: ads.AdGroup, EntityID: "adgroup_broad_us", Start: "2026-09-04", End: "2026-09-04"})
	if err != nil {
		t.Fatal(err)
	}
	group, err := b.Get(ctx, ads.AdGroup, "adgroup_broad_us")
	if err != nil {
		t.Fatal(err)
	}
	lowerBudget := decimal.RequireFromString("2")
	if outcome := b.Write(ctx, ads.WriteRequest{Target: group, Kind: "budget", Budget: &lowerBudget}); outcome.State != "acknowledged" {
		t.Fatalf("budget change=%#v", outcome)
	}
	if _, _, err = b.Advance(ctx, 24); err != nil {
		t.Fatal(err)
	}
	dayOneAfter := sandboxDailyReport(t, b, "2026-09-04")
	dayTwo, err := b.Report(ctx, ads.ReportQuery{Level: ads.AdGroup, EntityID: group.ID, Start: "2026-09-05", End: "2026-09-05"})
	if err != nil {
		t.Fatal(err)
	}
	if !dayOneBefore.Totals.Spend.Equal(dayOneAfter.Totals.Spend) || dayOneBefore.Totals.Impressions != dayOneAfter.Totals.Impressions || dayOneBefore.Totals.Clicks != dayOneAfter.Totals.Clicks {
		t.Fatalf("immutable delivery facts changed: before=%#v after=%#v", dayOneBefore.Totals, dayOneAfter.Totals)
	}
	if dayOneBefore.Totals.Conversions == nil || dayOneAfter.Totals.Conversions == nil || dayOneAfter.Totals.Conversions.LessThan(*dayOneBefore.Totals.Conversions) {
		t.Fatalf("attribution backfill regressed: before=%#v after=%#v", dayOneBefore.Totals, dayOneAfter.Totals)
	}
	// Other groups may win the opportunities this group no longer enters.
	// A group budget constrains that group, not the entire account's spend.
	if dayTwo.Totals.Spend.GreaterThan(lowerBudget) || !dayTwo.Totals.Spend.LessThan(beforeGroup.Totals.Spend) {
		t.Fatalf("lower group budget did not constrain spend: day1=%s day2=%s", beforeGroup.Totals.Spend, dayTwo.Totals.Spend)
	}
	ad, err := b.Get(ctx, ads.Ad, "ad_prospect_creator")
	if err != nil {
		t.Fatal(err)
	}
	if outcome := b.Write(ctx, ads.WriteRequest{Target: ad, Kind: "status", Status: "DISABLE"}); outcome.State != "acknowledged" {
		t.Fatalf("pause=%#v", outcome)
	}
	_, pausedFacts, err := b.Advance(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range pausedFacts {
		if fact.AdID != ad.ID {
			continue
		}
		if !fact.Metrics.Spend.IsZero() || fact.Metrics.Impressions != 0 || fact.Metrics.Clicks != 0 || !fact.Metrics.Conversions.IsZero() || !fact.Metrics.Revenue.IsZero() {
			t.Fatalf("paused hierarchy delivered: %#v", fact)
		}
	}
}

func TestSimulationAdvanceBoundsAndCancellation(t *testing.T) {
	b, err := NewEnvironment("bounds")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx   context.Context
		hours int
	}{{context.Background(), 0}, {context.Background(), MaxAdvanceHours + 1}, {ctx, 1}} {
		before, stateErr := b.SimulationState(context.Background())
		if stateErr != nil {
			t.Fatal(stateErr)
		}
		if _, _, err = b.Advance(tc.ctx, tc.hours); err == nil {
			t.Fatalf("advance accepted: hours=%d", tc.hours)
		}
		after, stateErr := b.SimulationState(context.Background())
		if stateErr != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("rejected advance mutated state: before=%#v after=%#v err=%v", before, after, stateErr)
		}
	}
}

func TestSimulationEnforcesAnnualGeneratedHourLimit(t *testing.T) {
	b, err := NewEnvironment("annual-limit")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	remaining := MaxGeneratedHours
	for remaining > 0 {
		hours := remaining
		if hours > MaxAdvanceHours {
			hours = MaxAdvanceHours
		}
		if _, _, err = b.Advance(ctx, hours); err != nil {
			t.Fatal(err)
		}
		remaining -= hours
	}
	before, err := b.SimulationState(ctx)
	if err != nil || before.GeneratedHours != MaxGeneratedHours || before.FactCount != 12*MaxGeneratedHours {
		t.Fatalf("annual state=%#v err=%v", before, err)
	}
	if _, _, err = b.Advance(ctx, 1); err == nil {
		t.Fatal("sandbox advanced beyond annual limit")
	}
	after, err := b.SimulationState(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("limit rejection mutated state: before=%#v after=%#v err=%v", before, after, err)
	}
}

func TestSandboxCommonResourcesCoverOperationalFamilies(t *testing.T) {
	b, err := NewAccountEnvironment("resources", "adv_resource", "Resource advertiser")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	identities, identityErr := b.ListIdentities(ctx)
	creative, creativeErr := b.ListCreativeAssets(ctx)
	audiences, audienceErr := b.ListAudiences(ctx)
	targeting, targetingErr := b.ListTargetingOptions(ctx, "")
	events, eventErr := b.ListEventSources(ctx)
	forms, formErr := b.ListLeadForms(ctx)
	catalogs, catalogErr := b.ListCatalogs(ctx)
	rules, ruleErr := b.ListAutomatedRules(ctx)
	if identityErr != nil || creativeErr != nil || audienceErr != nil || targetingErr != nil || eventErr != nil || formErr != nil || catalogErr != nil || ruleErr != nil {
		t.Fatalf("resource errors: %v %v %v %v %v %v %v %v", identityErr, creativeErr, audienceErr, targetingErr, eventErr, formErr, catalogErr, ruleErr)
	}
	if len(identities) == 0 || len(creative) == 0 || len(audiences) == 0 || len(targeting) == 0 || len(events) == 0 || len(forms) == 0 || len(catalogs) == 0 || len(rules) == 0 {
		t.Fatalf("resource family missing: identities=%d creative=%d audiences=%d targeting=%d events=%d forms=%d catalogs=%d rules=%d", len(identities), len(creative), len(audiences), len(targeting), len(events), len(forms), len(catalogs), len(rules))
	}
	for _, accountID := range []string{identities[0].AccountID, creative[0].AccountID, audiences[0].AccountID, events[0].AccountID, forms[0].AccountID, catalogs[0].AccountID, rules[0].AccountID} {
		if accountID != "adv_resource" {
			t.Fatalf("resource escaped advertiser scope: %q", accountID)
		}
	}
	if len(forms[0].FieldNames) == 0 {
		t.Fatal("lead form schema metadata missing")
	}
	if _, err = b.GetEventStats(ctx, events[0].ID, "2022-07-01", "2022-07-31"); err != nil {
		t.Fatal(err)
	}
	if _, err = b.ListProductSets(ctx, catalogs[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = b.ListAutomatedRuleResults(ctx, rules[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxAdDetailsBindLicensedPreviewMedia(t *testing.T) {
	b, err := NewAccountEnvironment("creative-media", "adv_resource", "Resource advertiser")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ad_prospect_creator", "ad_prospect_demo", "ad_interest_room", "ad_launch_collection"} {
		detail, detailErr := b.GetAdDetail(context.Background(), id)
		if detailErr != nil {
			t.Fatalf("detail %s: %v", id, detailErr)
		}
		if detail.Media == nil || detail.Media.PreviewURL == "" || detail.Media.SourceURL == "" || detail.Media.Attribution == "" {
			t.Fatalf("detail %s is missing preview provenance: %#v", id, detail.Media)
		}
		if detail.Media.PreviewURL[0] != '/' {
			t.Fatalf("detail %s uses a non-local preview: %q", id, detail.Media.PreviewURL)
		}
	}
}

func enableDeliveryPath(t *testing.T, b *Backend) {
	t.Helper()
	ctx := context.Background()
	for _, target := range []struct {
		level ads.Level
		id    string
	}{{ads.Campaign, "campaign_prospect_us"}, {ads.AdGroup, "adgroup_broad_us"}, {ads.Ad, "ad_prospect_creator"}} {
		entity, err := b.Get(ctx, target.level, target.id)
		if err != nil {
			t.Fatal(err)
		}
		if outcome := b.Write(ctx, ads.WriteRequest{Target: entity, Kind: "status", Status: "ENABLE"}); outcome.State != "acknowledged" {
			t.Fatalf("enable %s: %#v", target.id, outcome)
		}
	}
}

func sandboxDailyReport(t *testing.T, b *Backend, day string) ads.Report {
	t.Helper()
	report, err := b.Report(context.Background(), ads.ReportQuery{Level: ads.Advertiser, Start: day, End: day})
	if err != nil || !report.Complete {
		t.Fatalf("report %s: complete=%t err=%v", day, report.Complete, err)
	}
	return report
}

func TestHourlySimulationRejectsCorruptPersistedFacts(t *testing.T) {
	b, _ := NewEnvironment("corrupt")
	negative := decimal.RequireFromString("-1")
	zero := decimal.Zero
	err := b.RestoreSimulation(nil, []HourFact{{
		AdID:    "ad_prospect_creator",
		Hour:    b.simulationStart.Add(time.Hour),
		Metrics: ads.Metrics{Spend: negative, Conversions: &zero, Revenue: &zero},
	}})
	if err == nil {
		t.Fatal("corrupt persisted fact was accepted")
	}
}
