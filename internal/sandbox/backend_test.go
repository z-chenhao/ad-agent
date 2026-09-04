package sandbox

import (
	"context"
	"encoding/json"
	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"testing"
)

func TestHierarchyRollups(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var canonical ads.Metrics
	for _, level := range []ads.Level{ads.Ad, ads.AdGroup, ads.Campaign, ads.Advertiser} {
		r, err := b.Report(ctx, ads.ReportQuery{Level: level, Start: "2022-07-11", End: "2022-07-17"})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Complete {
			t.Fatal("incomplete sandbox")
		}
		if !r.Totals.Spend.Equal(mustDecimal("21")) || r.Totals.Revenue == nil || !r.Totals.Revenue.Equal(mustDecimal("35")) {
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
		if c.ROAS == nil || !c.ROAS.Equal(mustDecimal("35").Div(mustDecimal("21"))) {
			t.Fatal("weighted ratio incorrect")
		}
	}
	prev, _ := b.Report(ctx, ads.ReportQuery{Level: ads.Campaign, Start: "2022-07-04", End: "2022-07-10"})
	cur, _ := b.Report(ctx, ads.ReportQuery{Level: ads.Campaign, Start: "2022-07-11", End: "2022-07-17"})
	comp, err := ads.Compare(prev, cur)
	if err != nil {
		t.Fatal(err)
	}
	if comp.PreviousROAS.String() != "3" || comp.Contributions[1].ROASPoints.String() != "0" {
		t.Fatalf("unexpected comparison %+v", comp)
	}
	sum := mustDecimal("0")
	for _, c := range comp.Contributions {
		sum = sum.Add(*c.ROASPoints)
	}
	if !sum.Equal(*comp.DeltaROAS) {
		t.Fatal("contributions do not sum")
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
	b, err := New()
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
	if _, e := b.Report(context.Background(), ads.ReportQuery{Level: ads.Advertiser, EntityID: "other", Start: "2022-07-11", End: "2022-07-17"}); e == nil {
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
	if _, err = b.Create(ctx, ads.CreateRequest{Level: ads.Ad, ParentID: "adgroup_example_1", Name: "Invalid budget", Status: "DISABLE", Budget: decimalPtr("10"), BudgetMode: "BUDGET_MODE_DAY"}); err == nil {
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
