import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { readFile } from "node:fs/promises";
// Ordinary UI cases reuse one worker-local authenticated session. Keep credentials
// out of stored fixtures and avoid exhausting the real login rate limit.
let workspaceCookies:
  Awaited<ReturnType<BrowserContext["cookies"]>> | undefined;
async function login(page: Page) {
  if (workspaceCookies) {
    await page.context().addCookies(workspaceCookies);
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: "Today", exact: true }),
    ).toBeVisible();
    return;
  }
  await page.goto("/");
  await expect(page).toHaveTitle("Ad Desk · Advertising Workspace");
  await expect(
    page.getByRole("heading", { name: "Open Ad Desk" }),
  ).toBeVisible();
  const dataDir =
    process.env.AD_AGENT_E2E_DATA_DIR ??
    (process.env.AD_AGENT_E2E_RUNTIME === "codex" ? "e2e-codex" : "e2e");
  const key = (
    await readFile(
      new URL(`../../.data/${dataDir}/operator-key`, import.meta.url),
      "utf8",
    )
  ).trim();
  await page.getByLabel("Local operator key").fill(key);
  await page.getByRole("button", { name: "Enter workspace" }).click();
  await expect(
    page.getByRole("heading", { name: "Today", exact: true }),
  ).toBeVisible();
  workspaceCookies = await page.context().cookies();
  await expect(page.getByText("Ad Desk", { exact: true })).toBeVisible();
  await expect(
    page
      .locator(".assistant-panel header")
      .getByText("Ad Agent", { exact: true }),
  ).toBeVisible();
}

test("login rate limiting is explained without retaining the entered key", async ({
  page,
}) => {
  await page.route("**/api/v1/login", (route) =>
    route.fulfill({ status: 429, json: { error: "login_rate_limited" } }),
  );
  await page.goto("/");
  await page.getByLabel("Local operator key").fill("not-a-real-operator-key");
  await page.getByRole("button", { name: "Enter workspace" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Too many login attempts",
  );
  await expect(page.getByLabel("Local operator key")).toHaveValue("");
});

test("page titles, account identity and report controls have separate locations", async ({
  page,
}, info) => {
  await login(page);
  const account = await (
    await page.request.get("/api/v1/advertisers/current")
  ).json();
  const toolbar = page.getByLabel("Workspace toolbar");
  await expect(toolbar.getByText(account.name, { exact: true })).toHaveCount(1);
  await expect(toolbar).toContainText(account.currency);
  await expect(toolbar).toContainText(account.timezone);
  await expect(page.getByText("Today", { exact: true })).toHaveCount(2);
  await expect(page.getByLabel("Report period")).not.toContainText(
    account.name,
  );
  for (const name of ["Today", "Campaigns", "Creatives", "Changes"]) {
    await page.getByRole("button", { name, exact: true }).click();
    await expect(page.locator("main h1")).toHaveText(name);
    await expect(toolbar.getByText(name, { exact: true })).toHaveCount(0);
    await expect(page.getByLabel("Workspace account")).toContainText(
      account.name,
    );
    await expect(page.getByLabel("Date range")).toHaveCount(1);
  }
  await page.getByRole("button", { name: "Today", exact: true }).click();
  await expect(page.locator(".current-context")).toContainText(
    "Account briefing",
  );
  await page.screenshot({ path: info.outputPath("page-hierarchy.png") });
});

test("business pages omit simulator explanations but retain source and actual data caveats", async ({
  page,
}) => {
  await login(page);
  await expect(page.getByLabel("Workspace toolbar")).toContainText("Sandbox ·");
  await expect(page.locator("main")).not.toContainText("Data and attribution");
  await expect(page.locator("main")).not.toContainText("fictional advertiser");
  await expect(page.locator("main")).not.toContainText("modeling assumptions");
  const account = await (
    await page.request.get("/api/v1/advertisers/current")
  ).json();
  expect(account.source.backend).toBe("sandbox");
  expect(account.limitations).toEqual([]);

  // An evidence-specific warning must survive; do not suppress the limitations
  // field, perform text-based filtering, or silently turn partial data into zero.
  await page.route("**/api/v1/advertisers/current", async (route) => {
    const response = await route.fetch();
    const value = await response.json();
    value.limitations = ["Purchase value is unavailable for this account."];
    await route.fulfill({ json: value });
  });
  await page.reload();
  const notes = page.locator("main details").filter({ hasText: "Data notes" });
  await notes.locator("summary").click();
  await expect(notes).toContainText(
    "Purchase value is unavailable for this account.",
  );
});

test("visual hierarchy and action ownership stay consistent across workspace and assistant", async ({
  page,
}, info) => {
  await login(page);
  const rail = page.locator(".assistant-panel");
  await expect(
    rail.getByRole("button", { name: /Diagnose|Analyze/ }),
  ).toHaveCount(0);
  await expect(rail.getByText("Next action", { exact: true })).toHaveCount(0);
  await expect(rail.getByText("Conversation", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Sandbox clock" })).toHaveCount(
    1,
  );
  await expect(
    page.locator("main").getByRole("button", { name: /hour/ }),
  ).toHaveCount(0);
  const fontSize = async (selector: string) =>
    page
      .locator(selector)
      .first()
      .evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
  expect(await fontSize("main h1")).toBe(24);
  expect(await fontSize("main .ui-section-title")).toBe(16);
  expect(await fontSize("main .metric-value")).toBe(24);
  expect(await fontSize(".assistant-panel summary")).toBe(14);
  expect(
    await rail
      .getByRole("button", { name: "Compare periods" })
      .evaluate((el) => getComputedStyle(el).fontSize),
  ).toBe("12px");
  await page.screenshot({ path: info.outputPath("today-hierarchy.png") });

  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await page.locator("main tbody tr").first().click();
  await expect(
    page.getByRole("tab", { name: "Overview", exact: true }),
  ).toBeVisible();
  await expect(
    page.locator("main").getByRole("button", { name: "Diagnose", exact: true }),
  ).toHaveCount(1);
  await expect(
    rail.getByRole("button", { name: /Diagnose|Analyze/ }),
  ).toHaveCount(0);
  await page.screenshot({ path: info.outputPath("campaign-hierarchy.png") });

  await page.getByRole("button", { name: "Creatives", exact: true }).click();
  await expect(page.locator("main tbody tr").first()).toBeVisible();
  await page.screenshot({ path: info.outputPath("creative-hierarchy.png") });
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  for (const tab of [
    "Model",
    "Runtime",
    "Skills",
    "Ad connection",
    "Guardrails",
  ]) {
    await page.getByRole("tab", { name: tab, exact: true }).click();
    await expect(page.getByRole("tabpanel")).toBeVisible();
    await page.screenshot({
      path: info.outputPath(`settings-${tab.replaceAll(" ", "-")}.png`),
    });
  }
});

test("missing comparison data is not presented as healthy performance", async ({
  page,
}) => {
  await page.route("**/api/v1/report?*", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    if (data.calculation) data.calculation.ranking = [];
    await route.fulfill({ json: data });
  });
  await login(page);
  await expect(page.getByLabel("Performance metrics").first()).toBeVisible();
  await expect(page.locator("main")).toContainText(
    "Campaign comparison unavailable",
  );
  await expect(page.locator("main")).not.toContainText(
    "No campaign decline established",
  );
});

test("a slow previous date selection cannot overwrite the selected report", async ({
  page,
}) => {
  await login(page);
  await expect(page.getByLabel("Performance metrics").first()).toBeVisible();
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  let delayed = 0;
  await page.route("**/api/v1/report?*", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const url = new URL(route.request().url());
    const days =
      (Date.parse(url.searchParams.get("end_date")!) -
        Date.parse(url.searchParams.get("start_date")!)) /
        86400000 +
      1;
    data.report.totals.spend = days === 14 ? "14141" : "777";
    if (days === 14) {
      delayed++;
      await gate;
    }
    await route.fulfill({ json: data });
  });
  await page.getByLabel("Date range").selectOption("14");
  await expect.poll(() => delayed).toBe(2);
  await page.getByLabel("Date range").selectOption("7");
  await expect(page.getByLabel("Performance metrics").first()).toContainText(
    "777",
  );
  const completed = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/v1/report" &&
      response.url().includes("start_date="),
  );
  release();
  await completed;
  await expect(page.getByLabel("Performance metrics").first()).toContainText(
    "777",
  );
  await expect(
    page.getByLabel("Performance metrics").first(),
  ).not.toContainText("14,141");
});

for (const pageName of ["Campaigns", "Creatives"]) {
  test(`${pageName} explains a failed read and retries without stale metrics`, async ({
    page,
  }) => {
    await login(page);
    await expect(page.getByLabel("Performance metrics").first()).toBeVisible();
    await page.route("**/api/v1/report?*", (route) =>
      route.fulfill({ status: 503, json: { error: "report_unavailable" } }),
    );
    await page.getByRole("button", { name: pageName, exact: true }).click();
    await expect(page.locator("main").getByRole("alert")).toContainText(
      "Previous metrics are not shown",
    );
    await expect(page.locator("main tbody tr")).toHaveCount(0);
    await page.unroute("**/api/v1/report?*");
    await page.getByRole("button", { name: "Retry data", exact: true }).click();
    await expect(page.locator("main tbody tr").first()).toBeVisible();
    await expect(page.locator("main").getByRole("alert")).toHaveCount(0);
  });
}

test("long current context shares conversation scrolling and leaves the composer accessible", async ({
  page,
}, info) => {
  await login(page);
  const longName = "Home collection / prospecting / ".repeat(24);
  await page.route("**/api/v1/entities/campaign", async (route) => {
    const response = await route.fetch();
    const entities = await response.json();
    entities[0].name = longName;
    await route.fulfill({ json: entities });
  });
  await page.setViewportSize({ width: 1200, height: 720 });
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await page.locator("main tbody tr").first().click();
  const context = page.locator(".current-context");
  await expect(context.locator("summary")).toContainText(longName.trim());
  await context.locator("summary").click();
  await expect(context).toHaveAttribute("open", "");
  const dimensions = await context
    .locator(".context-details")
    .evaluate((el) => ({
      overflow: getComputedStyle(el).overflowY,
      scroll: el.scrollHeight,
      client: el.clientHeight,
    }));
  expect(dimensions.overflow).toBe("visible");
  expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.client + 1);
  await expect(page.getByLabel("Your advertising question")).toBeInViewport();
  await expect(
    page.getByRole("button", { name: "New session" }),
  ).toBeInViewport();
  await expect(page.getByLabel("Conversation")).toHaveCount(1);
  const checkRailWidth = async () => {
    const viewport = page.locator('[aria-label="Conversation"]:visible');
    const size = await viewport.evaluate((el) => ({
      width: el.clientWidth,
      scroll: el.scrollWidth,
    }));
    expect(size.scroll).toBeLessThanOrEqual(size.width + 1);
  };
  await checkRailWidth();
  await page.screenshot({ path: info.outputPath("long-context.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  await page
    .getByRole("button", { name: "Open assistant", exact: true })
    .click();
  await expect(
    page.getByLabel("Your advertising question on mobile"),
  ).toBeInViewport();
  const mobileContext = page.getByRole("dialog").locator(".current-context");
  await mobileContext.locator("summary").click();
  await checkRailWidth();
  await expect(
    page.getByLabel("Your advertising question on mobile"),
  ).toBeInViewport();
  await page.screenshot({ path: info.outputPath("mobile-context.png") });
});

test("completed turns retain tools, context, safe Markdown and compact cards after reload", async ({
  page,
}, info) => {
  let count = 0;
  const turns: Record<string, unknown[]> = {};
  const session = {
    id: "web",
    messages: [] as {
      role: string;
      text: string;
      turn_id: string;
      status: string;
    }[],
  };
  await page.route("**/api/v1/session?*", (route) =>
    route.fulfill({ json: session }),
  );
  await page.route("**/api/v1/turns/*/events?*", (route) => {
    const id = new URL(route.request().url()).pathname.split("/").at(-2)!;
    return route.fulfill({ json: turns[id] ?? [] });
  });
  await page.route("**/api/v1/agent/turn", async (route) => {
    const input = route.request().postDataJSON();
    const id = `turn-${++count}`;
    const text = `## Finding ${count}\n\n**Spend is reported**, not forecast.\n\n- Read the selected period\n- Keep changes approval-gated\n\n| Metric | Value |\n|---|---|\n| ROAS | 2.4 |\n\n[Unsafe](javascript:alert(1))\n\n<script>window.injection = true</script>`;
    const cards = [
      {
        id: `card-${count}`,
        type: "digest",
        digest: {
          title: `Report ${count}`,
          items: [
            {
              kind: "opportunity",
              headline:
                "The seven-day comparison does not establish a scaling winner",
              why: "Seven days of evidence do not establish incremental lift.",
              action:
                "Keep total budget unchanged; compare the last 28 complete days before proposing a reallocation.",
            },
          ],
        },
      },
      {
        id: `suggestion-${count}`,
        type: "suggestions",
        suggestions: [
          "Compare Cart 7D and Viewers 14D over the last 28 complete days.",
        ],
      },
    ];
    const data = [
      ["turn.started", {}],
      ["context.bound", input.view_context],
      ["progress.updated", { message: "Reading the selected report" }],
      [
        "text.delta",
        { text: "I will check delivery before recommending a change." },
      ],
      ["tool.started", { id: `read-${count}`, name: "get_performance_report" }],
      [
        "tool.finished",
        {
          id: `read-${count}`,
          name: "get_performance_report",
          ok: true,
          duration_ms: 125,
        },
      ],
      [
        "tool.finished",
        {
          id: `budget-${count}`,
          name: "stage_budget_change",
          ok: false,
          error: "budget_delta_exceeded",
          duration_ms: 10,
        },
      ],
      [
        "tool.finished",
        {
          id: `analysis-${count}`,
          name: "run_analysis",
          ok: false,
          error: "analysis_missing_submission",
          duration_ms: 2200,
        },
      ],
      ["text.delta", { text }],
      [
        "turn.completed",
        {
          turn_id: id,
          session_id: session.id,
          status: "completed",
          text,
          cards,
          elapsed_ms: 2400,
          usage: { input: 0, output: 0, cache_read: 0, cache_write: 0 },
        },
      ],
    ];
    const events = data.map(([type, data], i) => ({
      v: "1",
      type,
      data,
      seq: i + 1,
      turnId: id,
      at: "2026-09-05T00:00:00Z",
    }));
    turns[id] = events;
    session.messages.push(
      { role: "user", text: input.message, turn_id: id, status: "completed" },
      { role: "assistant", text, turn_id: id, status: "completed" },
    );
    await route.fulfill({
      contentType: "text/event-stream",
      body: events
        .map((event) => `data: ${JSON.stringify(event)}\n\n`)
        .join(""),
    });
  });
  await login(page);
  const context = page.locator(".current-context").first();
  expect(
    await context.evaluate((el) => el.getBoundingClientRect().height),
  ).toBeLessThan(145);
  const composer = page.getByLabel("Your advertising question");
  await composer.fill("Inspect my report");
  expect(
    await composer.evaluate((el) => getComputedStyle(el).outlineStyle),
  ).toBe("none");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Finding 1" })).toBeVisible();
  await expect(page.getByLabel("Agent activity")).toHaveCount(1);
  await page.getByLabel("Agent activity").getByRole("button").click();
  await expect(page.getByLabel("Agent activity")).toContainText("Done");
  await expect(
    page.getByLabel("Agent activity").locator("details"),
  ).toHaveCount(0);
  const messageContext = page.getByLabel("Message context");
  await expect(messageContext).toHaveCount(1);
  await expect(messageContext).not.toHaveAttribute("open", "");
  expect(
    await messageContext.evaluate(
      (el) => el.previousElementSibling?.textContent,
    ),
  ).toBe("Inspect my report");
  await messageContext.locator("summary").click();
  const sentScope = await messageContext.locator("dl").innerText();
  expect(sentScope).toContain("Aster & Pine Home");
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await page.locator("main tbody tr").first().click();
  await expect(messageContext.locator("dl")).toHaveText(sentScope, {
    useInnerText: true,
  });
  await expect(page.getByText("Progress updates", { exact: true })).toHaveCount(
    0,
  );
  await expect(page.getByLabel("Agent activity")).toContainText("125ms");
  await expect(
    page.getByText("I will check delivery before recommending a change.", {
      exact: true,
    }),
  ).toHaveCount(1);
  await expect(page.getByLabel("Agent activity")).toContainText(
    "Review Guardrails in Settings",
  );
  await expect(page.getByLabel("Agent activity")).toContainText(
    "Incomplete · 2.2s",
  );
  await expect(page.getByLabel("Agent activity")).toContainText(
    "without submitting a validated result",
  );
  await expect(page.locator(".agent-markdown strong")).toHaveText(
    "Spend is reported",
  );
  await expect(page.locator(".agent-markdown table")).toBeVisible();
  await expect(
    page.locator('.agent-markdown a[href^="javascript:"]'),
  ).toHaveCount(0);
  await expect(page.locator(".agent-markdown script")).toHaveCount(0);
  const recommendation = page.getByText(
    "Keep total budget unchanged; compare the last 28 complete days before proposing a reallocation.",
    { exact: true },
  );
  await expect(recommendation).toBeVisible();
  expect(await recommendation.evaluate((el) => el.tagName)).toBe("P");
  await composer.fill("My unfinished question");
  await recommendation.click();
  await expect(composer).toHaveValue("My unfinished question");
  await page
    .getByRole("button", {
      name: "Compare Cart 7D and Viewers 14D over the last 28 complete days.",
    })
    .click();
  await expect(composer).toHaveValue(
    "Compare Cart 7D and Viewers 14D over the last 28 complete days.",
  );
  expect(count).toBe(1); // A follow-up edits the composer; it never sends or approves.
  await composer.fill("Read it again");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Finding 2" })).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Agent activity")).toHaveCount(2);
  await expect(messageContext).toHaveCount(2);
  await expect(messageContext.first()).not.toHaveAttribute("open", "");
  await messageContext.first().locator("summary").focus();
  await page.keyboard.press("Enter");
  await expect(messageContext.first().locator("dl")).toHaveText(sentScope, {
    useInnerText: true,
  });
  await expect(
    page.getByLabel("Agent activity").locator(".turn-context"),
  ).toHaveCount(0);
  await page.getByLabel("Agent activity").first().getByRole("button").click();
  await expect(page.getByLabel("Agent activity").first()).toContainText(
    "Incomplete · 2.2s",
  );
  await expect(page.getByLabel("Agent activity").first()).toContainText(
    "I will check delivery before recommending a change.",
  );
  await expect(page.getByRole("heading", { name: "Finding 1" })).toHaveCount(1);
  await expect(page.getByRole("heading", { name: "Finding 2" })).toHaveCount(1);
  await expect(page.getByText("Report 1", { exact: true })).toHaveCount(1);
  await expect(page.getByText("Report 2", { exact: true })).toHaveCount(1);
  await expect(recommendation).toHaveCount(2);
  await expect(
    page.getByRole("button", {
      name: "Keep total budget unchanged; compare the last 28 complete days before proposing a reallocation.",
    }),
  ).toHaveCount(0);
  await page.screenshot({ path: info.outputPath("conversation.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  await page
    .getByRole("button", { name: "Open assistant", exact: true })
    .click();
  const mobileContext = page
    .getByRole("dialog")
    .getByLabel("Message context")
    .first();
  await expect(mobileContext).not.toHaveAttribute("open", "");
  await mobileContext.locator("summary").click();
  await expect(mobileContext.locator("dl")).toHaveText(sentScope, {
    useInnerText: true,
  });
  const viewport = page.locator('[aria-label="Conversation"]:visible');
  const width = await viewport.evaluate((el) => ({
    client: el.clientWidth,
    scroll: el.scrollWidth,
  }));
  expect(width.scroll).toBeLessThanOrEqual(width.client + 1);
  await mobileContext.scrollIntoViewIfNeeded();
  await page.screenshot({
    path: info.outputPath("mobile-message-context.png"),
  });
});

test("briefings separate grounded subjects, judgments, next steps and evidence without action controls", async ({
  page,
}, info) => {
  let requests = 0;
  const session = { id: "web", messages: [] as Record<string, string>[] };
  let savedEvents: unknown[] = [];
  await page.route("**/api/v1/session?*", (route) =>
    route.fulfill({ json: session }),
  );
  await page.route("**/api/v1/turns/*/events?*", (route) =>
    route.fulfill({ json: savedEvents }),
  );
  const entity = {
    id: "ad_cart_reminder",
    account_id: "adv_home",
    level: "ad",
    name: "Cart reminder · Small-space storage collection",
    status: "ENABLE",
  };
  const card = {
    id: "briefing-decisions",
    type: "digest",
    digest: {
      title: "Creative delivery decisions",
      items: [
        {
          kind: "opportunity",
          entity,
          headline:
            "Click traffic weakened while post-click conversion held steady",
          action:
            "Inspect the opening frame and offer; compare ad-level CTR over the last 14 complete days before preparing a replacement.",
          why: "CTR fell from 2.4% to 1.8%; click CVR stayed near 4% in the two complete seven-day windows. This does not establish creative fatigue.",
        },
        {
          kind: "change",
          change: {
            id: "change_review",
            kind: "budget",
            status: "pending",
            before: {
              ...entity,
              id: "adgroup_cart",
              level: "ad_group",
              name: "Cart visitors · 7 days",
            },
          },
          headline: "The budget proposal is still awaiting review",
          action:
            "Review the exact proposed values in Changes before deciding whether to approve; no budget change has been confirmed.",
          why: "The saved draft is pending approval, with no successful execution or read-back record.",
        },
        {
          kind: "measurement",
          headline: "Today's conversion data is not complete",
          action:
            "Recheck tomorrow after the reporting window closes before interpreting today's conversion trend.",
          why: "The current day contains only six reported hours; recent conversions may still backfill.",
        },
      ],
    },
  };
  await page.route("**/api/v1/agent/turn", async (route) => {
    requests++;
    const event = {
      v: "1",
      seq: 2,
      type: "turn.completed",
      turnId: "briefing-turn",
      at: "2026-09-05T00:00:00Z",
      data: {
        turn_id: "briefing-turn",
        session_id: "web",
        status: "completed",
        text: "Creative delivery decisions are ready for review.",
        cards: [card],
      },
    };
    savedEvents = [{ ...event, seq: 1, type: "turn.started", data: {} }, event];
    session.messages.push(
      {
        role: "user",
        text: route.request().postDataJSON().message,
        turn_id: event.turnId,
        status: "completed",
      },
      {
        role: "assistant",
        text: event.data.text,
        turn_id: event.turnId,
        status: "completed",
      },
    );
    await route.fulfill({
      contentType: "text/event-stream",
      body: savedEvents
        .map((item) => `data: ${JSON.stringify(item)}\n\n`)
        .join(""),
    });
  });
  await login(page);
  const composer = page.getByLabel("Your advertising question");
  await composer.fill("Summarize the creative decisions");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  const briefing = page.locator(".briefing-card:visible");
  await expect(briefing).toHaveCount(1);
  const items = briefing.locator(".briefing-item");
  await expect(items).toHaveCount(3);
  await expect(items.first().locator(".briefing-subject")).toHaveText(
    `Ad · ${entity.name}`,
  );
  await expect(items.nth(1).locator(".briefing-subject")).toHaveText(
    "Change · Ad group · Cart visitors · 7 days",
  );
  await expect(items.last().locator(".briefing-subject")).toHaveCount(0);
  await expect(briefing.locator("button, a, svg")).toHaveCount(0);
  await expect(briefing.locator("details[open]")).toHaveCount(0);
  expect(
    await items
      .first()
      .evaluate((el) => Array.from(el.children).map((child) => child.tagName)),
  ).toEqual(["P", "H4", "DIV", "DETAILS"]);
  const type = await items.first().evaluate((el) => {
    const finding = getComputedStyle(el.querySelector(".briefing-finding")!);
    const action = getComputedStyle(
      el.querySelector(".briefing-next-step p:last-child")!,
    );
    const subject = getComputedStyle(el.querySelector(".briefing-subject")!);
    return {
      finding: finding.fontSize,
      action: action.fontSize,
      subject: subject.fontSize,
      weight: +finding.fontWeight,
      actionWeight: +action.fontWeight,
      color: action.color,
      subjectColor: subject.color,
    };
  });
  expect(type.finding).toBe("14px");
  expect(type.action).toBe("14px");
  expect(type.subject).toBe("12px");
  expect(type.weight).toBeGreaterThan(type.actionWeight);
  expect(type.color).not.toBe(type.subjectColor);
  await composer.fill("Keep my draft intact");
  await items.first().locator(".briefing-next-step p:last-child").click();
  await expect(composer).toHaveValue("Keep my draft intact");
  const evidence = items.first().locator("details");
  await evidence.locator("summary").focus();
  await page.keyboard.press("Enter");
  await expect(evidence).toHaveAttribute("open", "");
  await expect(evidence.locator("p")).toHaveText(card.digest.items[0]!.why);
  await page.keyboard.press("Enter");
  await expect(evidence).not.toHaveAttribute("open", "");
  await composer.focus();
  await briefing.screenshot({ path: info.outputPath("briefing-desktop.png") });
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await page.locator("main tbody tr").first().click();
  await page.reload();
  await expect(
    page.locator(".briefing-card .briefing-subject").first(),
  ).toHaveText(`Ad · ${entity.name}`);
  await expect(page.locator(".briefing-card details[open]")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await page
    .getByRole("button", { name: "Open assistant", exact: true })
    .click();
  await expect(
    page.locator(".briefing-card:visible .briefing-item"),
  ).toHaveCount(3);
  const conversation = page.locator('[aria-label="Conversation"]:visible');
  expect(
    await conversation.evaluate((el) => el.scrollWidth - el.clientWidth),
  ).toBeLessThanOrEqual(1);
  await conversation.evaluate((el) => {
    const card = el.querySelector(".briefing-card")!;
    el.scrollTop +=
      card.getBoundingClientRect().top - el.getBoundingClientRect().top - 8;
  });
  await expect(
    page.getByRole("heading", {
      name: "Creative delivery decisions",
      exact: true,
    }),
  ).toBeInViewport();
  await page.screenshot({ path: info.outputPath("briefing-mobile.png") });
  expect(requests).toBe(1); // Presentation and disclosure never send or approve anything.
});

test("metric cards retain evidence scope and recognizable object names after navigation and reload", async ({
  page,
}, info) => {
  const source = {
    backend: "sandbox",
    environment: "default",
    account_id: "account-home",
  };
  const query = {
    level: "ad_group",
    entity_id: "group-cart",
    start_date: "2026-08-28",
    end_date: "2026-09-03",
  };
  const scope = {
    account_id: source.account_id,
    account_name: "Aster & Pine Home",
    level: query.level,
    entity_id: query.entity_id,
    entity_name: "Cart recovery · 7 days",
  };
  const report = {
    source,
    query,
    currency: "USD",
    timezone: "America/Los_Angeles",
    totals: {
      spend: "209.65",
      revenue: "2205.18",
      clicks: 456,
      impressions: 24690,
    },
    limitations: [],
  };
  const cards = [
    { id: "group", type: "metrics", report, metric_scope: scope },
    {
      id: "campaign",
      type: "metrics",
      calculation: {
        ...report,
        query: { ...query, level: "campaign", entity_id: "campaign-retarget" },
      },
      metric_scope: {
        ...scope,
        level: "campaign",
        entity_id: "campaign-retarget",
        entity_name: "US · Returning shoppers",
      },
    },
    {
      id: "all",
      type: "metrics",
      report: { ...report, query: { ...query, entity_id: undefined } },
      metric_scope: { ...scope, entity_id: undefined, entity_name: undefined },
    },
    {
      id: "id-fallback",
      type: "metrics",
      report: {
        ...report,
        query: { ...query, level: "ad", entity_id: "ad-materials" },
      },
    },
    {
      id: "wrong-scope",
      type: "metrics",
      report: { ...report, query: { ...query, entity_id: "group-browse" } },
      metric_scope: {
        ...scope,
        entity_name: "Mismatched label must not appear",
      },
    },
    {
      id: "compare",
      type: "metrics",
      comparison: {
        source,
        current_query: query,
        previous_query: {
          ...query,
          start_date: "2026-08-21",
          end_date: "2026-08-27",
        },
        timezone: report.timezone,
        previous_roas: "9.2",
        current_roas: "10.518",
        delta_roas: "1.318",
        limitations: [],
        method: "Equal reporting periods",
      },
      metric_scope: { ...scope, entity_name: "Cart recovery comparison" },
    },
  ];
  await page.route("**/api/v1/session?*", (route) =>
    route.fulfill({
      json: {
        id: "web",
        messages: [
          {
            role: "user",
            text: "Compare my ad groups",
            turn_id: "scope-turn",
            status: "completed",
          },
          {
            role: "assistant",
            text: "Performance by object.",
            turn_id: "scope-turn",
            status: "completed",
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/turns/*/events?*", (route) =>
    route.fulfill({
      json: [
        {
          v: "1",
          type: "turn.completed",
          seq: 1,
          turnId: "scope-turn",
          at: "2026-09-05T00:00:00Z",
          data: {
            turn_id: "scope-turn",
            status: "completed",
            text: "Performance by object.",
            cards,
            elapsed_ms: 100,
          },
        },
      ],
    }),
  );
  await login(page);
  const conversation = page.getByLabel("Conversation");
  const check = async () => {
    for (const name of [
      "Cart recovery · 7 days",
      "US · Returning shoppers",
      "All ad groups",
      "ad-materials",
      "group-browse",
      "Cart recovery comparison",
    ]) {
      await expect(
        conversation.getByRole("heading", { name, exact: true }),
      ).toHaveCount(1);
    }
    await expect(conversation).toContainText(
      "Ad group · Performance · Aster & Pine Home",
    );
    await expect(conversation).toContainText(
      "Campaign · Performance · Aster & Pine Home",
    );
    await expect(conversation).toContainText(
      "Ad group · Period comparison · Aster & Pine Home",
    );
    await expect(conversation).not.toContainText(
      "Mismatched label must not appear",
    );
    await expect(
      conversation.getByRole("heading", { name: "Performance snapshot" }),
    ).toHaveCount(0);
  };
  await check();
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await check();
  await page.reload();
  await check();
  await conversation
    .getByRole("heading", { name: "Cart recovery · 7 days", exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath("scoped-metric-cards.png") });
});

test("failed runtime keeps its completed tools and explains the connection failure after reload", async ({
  page,
}, info) => {
  const result = {
    turn_id: "failed-runtime",
    session_id: "web",
    status: "failed",
    text: "This turn did not complete.",
    error_code: "provider_auth_failed",
    cards: [],
    elapsed_ms: 1000,
  };
  await page.route("**/api/v1/session?*", (route) =>
    route.fulfill({
      json: {
        id: "web",
        messages: [
          {
            role: "user",
            text: "Review delivery",
            turn_id: "failed-runtime",
            status: "failed",
          },
          {
            role: "assistant",
            text: result.text,
            turn_id: "failed-runtime",
            status: "failed",
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/turns/*/events?*", (route) =>
    route.fulfill({
      json: [
        { type: "turn.started", data: { runtime: "builtin" } },
        {
          type: "text.delta",
          data: { id: "commentary", text: "I will inspect delivery." },
        },
        {
          type: "tool.started",
          data: { id: "read", name: "get_performance_report" },
        },
        {
          type: "tool.finished",
          data: {
            id: "read",
            name: "get_performance_report",
            ok: true,
            duration_ms: 2,
          },
        },
        { type: "turn.completed", data: result },
      ].map((entry, i) => ({
        ...entry,
        v: "1",
        turnId: "failed-runtime",
        seq: i + 1,
        at: "2026-09-05",
      })),
    }),
  );
  await login(page);
  for (let i = 0; i < 2; i++) {
    const activity = page.getByLabel("Agent activity");
    await expect(activity).toContainText("Incomplete · 1 tool calls");
    await expect(activity.getByRole("status")).toContainText(
      "could not authenticate",
    );
    await activity.getByRole("button").click();
    await expect(activity).toContainText("I will inspect delivery.");
    await expect(activity).toContainText("Done · 2ms");
    if (!i) await page.reload();
  }
  await page.screenshot({ path: info.outputPath("runtime-failure.png") });
});

test("running activity interleaves public speech and tools without stale progress or zero-second rounding", async ({
  page,
}, info) => {
  let settled = false;
  const finalText = "## Delivery summary\n\nThe result is ready.";
  const replay: { type: string; data: unknown }[] = [
    { type: "turn.started", data: {} },
    {
      type: "progress.updated",
      data: { message: "Planning the next evidence-backed action" },
    },
  ];
  await page.route("**/api/v1/session?*", (route) =>
    route.fulfill({
      json: {
        id: "web",
        messages: settled
          ? [
              {
                role: "user",
                text: "Review delivery",
                turn_id: "stream-turn",
                status: "completed",
              },
              {
                role: "assistant",
                text: finalText,
                turn_id: "stream-turn",
                status: "completed",
              },
            ]
          : [],
      },
    }),
  );
  await page.route("**/api/v1/turns/*/events?*", (route) =>
    route.fulfill({
      json: replay.map((entry, i) => ({
        ...entry,
        v: "1",
        turnId: "stream-turn",
        seq: i + 1,
        at: new Date().toISOString(),
      })),
    }),
  );
  // A controlled public event stream exercises the real fetch reader and UI;
  // no provider, private reasoning, or operator Sandbox mutations are involved.
  await page.addInitScript(() => {
    const originalFetch = window.fetch.bind(window);
    let controller: ReadableStreamDefaultController<Uint8Array>;
    let seq = 0;
    const streamWindow = window as typeof window & {
      activityEvent: (type: string, data: unknown) => void;
      activityClose: () => void;
    };
    streamWindow.activityEvent = (type, data) =>
      controller.enqueue(
        new TextEncoder().encode(
          `data: ${JSON.stringify({ v: "1", turnId: "stream-turn", seq: ++seq, type, data, at: new Date().toISOString() })}\n\n`,
        ),
      );
    streamWindow.activityClose = () => controller.close();
    window.fetch = async (input, init) => {
      if (String(input).endsWith("/api/v1/agent/turn")) {
        return new Response(
          new ReadableStream<Uint8Array>({
            start(value) {
              controller = value;
              streamWindow.activityEvent("turn.started", {});
              streamWindow.activityEvent("progress.updated", {
                message: "Planning the next evidence-backed action",
              });
            },
          }),
          { headers: { "Content-Type": "text/event-stream" } },
        );
      }
      return originalFetch(input, init);
    };
  });
  await login(page);
  await page.getByLabel("Your advertising question").fill("Review delivery");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  const activity = page.getByLabel("Agent activity");
  await expect(activity.getByRole("button")).toContainText("Working");
  const emit = async (type: string, data: unknown) => {
    replay.push({ type, data });
    return page.evaluate(
      ({ type, data }) => {
        (
          window as unknown as {
            activityEvent: (type: string, data: unknown) => void;
          }
        ).activityEvent(type, data);
      },
      { type, data },
    );
  };
  await expect(page.getByLabel("Message context")).toHaveCount(0);
  await emit("context.bound", {
    page: "campaigns",
    account_name: "Bound advertising account",
    entity_level: "campaign",
    entity_name: "Bound campaign",
    start_date: "2026-08-28",
    end_date: "2026-09-03",
  });
  const boundContext = page.getByLabel("Message context");
  await expect(boundContext).toHaveCount(1);
  await expect(boundContext).not.toHaveAttribute("open", "");
  await boundContext.locator("summary").click();
  await expect(boundContext).toContainText("Bound campaign");
  await expect(boundContext).toContainText("Bound advertising account");
  expect(
    await boundContext.evaluate((el) => el.previousElementSibling?.textContent),
  ).toBe("Review delivery");
  await expect(activity.locator(".turn-context")).toHaveCount(0);
  await emit("text.delta", {
    text: "I will compare delivery with the previous period.",
  });
  await expect(activity.getByRole("button")).toContainText("Responding");
  await emit("tool.started", { id: "read", name: "get_performance_report" });
  await expect(activity.getByRole("button")).toContainText("Read performance");
  await expect(activity).toContainText("Running");
  await emit("tool.finished", {
    id: "read",
    name: "get_performance_report",
    ok: true,
    duration_ms: 3,
  });
  await expect(activity.getByRole("button")).toContainText("Working");
  await emit("text.delta", {
    text: "The first read is complete. I will verify the campaign.",
  });
  await emit("tool.started", { id: "entity", name: "get_entity" });
  await emit("tool.finished", {
    id: "entity",
    name: "get_entity",
    ok: false,
    duration_ms: 0,
  });
  await emit("tool.started", { id: "slow", name: "run_analysis" });
  await expect(activity).toContainText("Done · 3ms");
  await expect(activity).toContainText("Unsuccessful · <1ms");
  await expect(activity).not.toContainText("0.0s");
  await expect(activity).not.toContainText("Planning the next");
  await expect(activity).not.toContainText("Progress updates");
  await expect(
    page.getByText("I will compare delivery with the previous period.", {
      exact: true,
    }),
  ).toHaveCount(1);
  const rows = activity.locator(".border-l").first();
  await expect(rows).toContainText(
    /I will compare.*Read performance.*The first read.*Verify object/s,
  );
  // A running tool timer advances even while no new events arrive.
  await expect(activity.getByText(/Running · [1-9]\.\ds/)).toBeVisible({
    timeout: 4000,
  });
  await emit("tool.started", {
    id: "slice",
    name: "analysis_slice",
    role: "analysis",
    parent_id: "slow",
  });
  await emit("tool.finished", {
    id: "slice",
    name: "analysis_slice",
    role: "analysis",
    parent_id: "slow",
    ok: true,
    duration_ms: 2,
  });
  await emit("tool.started", {
    id: "calc",
    name: "analysis_calculate",
    role: "analysis",
    parent_id: "slow",
  });
  await emit("tool.finished", {
    id: "calc",
    name: "analysis_calculate",
    role: "analysis",
    parent_id: "slow",
    ok: false,
    duration_ms: 1,
  });
  const analysis = activity.locator(".analysis-activity");
  await expect(analysis).toHaveAttribute("open", "");
  await expect(analysis).toContainText("Filter data");
  await expect(analysis).toContainText("Calculate metrics");
  await expect(analysis).toContainText("1 unsuccessful");
  await expect(activity).not.toContainText("analysis_slice");
  await expect(activity).not.toContainText("· analyst");
  await page.screenshot({ path: info.outputPath("running-activity.png") });
  await emit("tool.finished", {
    id: "slow",
    name: "run_analysis",
    ok: true,
    duration_ms: 1250,
  });
  const evidenceCard = {
    id: "stable-evidence",
    type: "digest",
    digest: {
      title: "Delivery decision",
      items: [
        {
          kind: "measurement",
          headline: "The reporting window is incomplete",
          why: "Only six hours are available today.",
          action: "Recheck tomorrow before changing budgets.",
        },
      ],
    },
  };
  await emit("ui.upsert", evidenceCard);
  const card = page.locator(".briefing-card");
  await expect(card).toContainText("Delivery decision");
  const contentTop = () =>
    card.evaluate((el) => {
      const viewport = el.closest("[data-radix-scroll-area-viewport]")!;
      return el.getBoundingClientRect().top + viewport.scrollTop;
    });
  const beforeSpeech = await contentTop();
  await emit("text.delta", {
    id: "last-commentary",
    text: "The checks are complete; I am summarizing the evidence.",
  });
  await emit("text.delta", {
    id: "final-answer",
    text: "## Delivery summary\n\nThe result is ready.",
  });
  await expect(
    page.getByRole("heading", { name: "Delivery summary" }),
  ).toHaveCount(1);
  expect(Math.abs((await contentTop()) - beforeSpeech)).toBeLessThan(2);
  expect(
    await card.evaluate(
      (el) =>
        !!(
          el.compareDocumentPosition(
            document.querySelector(".turn-activity")!,
          ) & Node.DOCUMENT_POSITION_FOLLOWING
        ),
    ),
  ).toBe(true);
  settled = true;
  await emit("turn.completed", {
    turn_id: "stream-turn",
    status: "completed",
    text: "## Delivery summary\n\nThe result is ready.",
    cards: [evidenceCard],
    elapsed_ms: 3400,
  });
  await page.evaluate(() =>
    (window as unknown as { activityClose: () => void }).activityClose(),
  );
  await expect(activity.getByRole("button")).toContainText(
    "5 tool calls · 2 unsuccessful",
  );
  await expect(boundContext).toHaveCount(1);
  await boundContext.locator("summary").click();
  await expect(boundContext).toContainText("Bound campaign");
  await activity.getByRole("button").click();
  await expect(
    page.getByRole("heading", { name: "Delivery summary" }),
  ).toHaveCount(1);
  await expect(activity).toContainText("I will compare delivery");
  await expect(activity).toContainText("The checks are complete");
  await expect(activity).not.toContainText("The result is ready.");
  await expect(activity).toContainText("Done · 1.3s");
  expect(
    await card.evaluate(
      (el) =>
        !!(
          el.compareDocumentPosition(document.querySelector(".agent-answer")!) &
          Node.DOCUMENT_POSITION_FOLLOWING
        ),
    ),
  ).toBe(true);
  await expect(analysis).not.toHaveAttribute("open", "");
  await expect(analysis.locator("summary")).toContainText("1 unsuccessful");
  await analysis.locator("summary").click();
  await expect(
    analysis.getByText("Filter data", { exact: true }),
  ).toBeVisible();
  await expect(analysis).toContainText("Unsuccessful · 1ms");
  await page.screenshot({ path: info.outputPath("settled-activity.png") });
  await page.reload();
  await expect(card).toContainText("Delivery decision");
  expect(
    await card.evaluate(
      (el) =>
        !!(
          el.compareDocumentPosition(document.querySelector(".agent-answer")!) &
          Node.DOCUMENT_POSITION_FOLLOWING
        ),
    ),
  ).toBe(true);
  await activity.getByRole("button").click();
  await analysis.locator("summary").click();
  await expect(
    analysis.getByText("Filter data", { exact: true }),
  ).toBeVisible();
  await expect(analysis).toContainText("Unsuccessful · 1ms");
});

for (const source of ["read", "turn"] as const) {
  test(`expired ${source} session returns to login without retrying and recovers after login`, async ({
    page,
  }) => {
    await login(page);
    await page
      .getByRole("button", { name: "New session", exact: true })
      .click();
    const sessionID = await page.evaluate(() =>
      localStorage.getItem("ad-agent.session"),
    );
    let rejected = 0;
    const path =
      source === "read" ? "**/api/v1/session?*" : "**/api/v1/agent/turn";
    await page.route(path, (route) => {
      rejected++;
      return route.fulfill({
        status: 401,
        json: { error: "authentication_required" },
      });
    });
    if (source === "read") {
      await page.getByRole("button", { name: "Refresh history" }).click();
    } else {
      await page
        .getByLabel("Your advertising question")
        .fill("Prepare a budget draft");
      await page.getByRole("button", { name: "Send", exact: true }).click();
    }
    await expect(page.getByLabel("Local operator key")).toBeVisible();
    await expect(page.getByRole("status")).toContainText(
      "No operation was automatically retried",
    );
    await expect(page.getByLabel("Your advertising question")).toHaveCount(0);
    expect(rejected).toBe(1);
    await page.unroute(path);
    const dataDir =
      process.env.AD_AGENT_E2E_DATA_DIR ??
      (process.env.AD_AGENT_E2E_RUNTIME === "codex" ? "e2e-codex" : "e2e");
    const key = (
      await readFile(
        new URL(`../../.data/${dataDir}/operator-key`, import.meta.url),
        "utf8",
      )
    ).trim();
    await page.getByLabel("Local operator key").fill(key);
    await page.getByRole("button", { name: "Enter workspace" }).click();
    await expect(
      page.getByRole("heading", { name: "Today", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(() => localStorage.getItem("ad-agent.session")),
    ).toBe(sessionID);
    expect(rejected).toBe(1);
  });
}
test("runtime and model settings retain conversation, cards and composer across saves and reload", async ({
  page,
}) => {
  const sessionID = "web-continuity";
  await page.addInitScript(() =>
    localStorage.setItem("ad-agent.session", "web-continuity"),
  );
  const session = { id: sessionID, messages: [] as Record<string, string>[] };
  const saved: Record<string, unknown[]> = {};
  const selections: {
    runtime: string;
    model: { model: string };
    session_id: string;
  }[] = [];
  await page.route("**/api/v1/session?*", (route) =>
    route.fulfill({ json: session }),
  );
  await page.route("**/api/v1/turns/*/events?*", (route) => {
    const id = new URL(route.request().url()).pathname.split("/").at(-2)!;
    return route.fulfill({ json: saved[id] ?? [] });
  });
  await page.route("**/api/v1/agent/turn", async (route) => {
    const input = route.request().postDataJSON();
    selections.push(input);
    const turnID = `continuous-${selections.length}`;
    const text = `Saved answer ${selections.length}`;
    const events = [
      {
        v: "1",
        seq: 1,
        type: "turn.started",
        turnId: turnID,
        at: "2026-09-05T00:00:00Z",
        data: {},
      },
      {
        v: "1",
        seq: 2,
        type: "turn.completed",
        turnId: turnID,
        at: "2026-09-05T00:00:01Z",
        data: {
          turn_id: turnID,
          session_id: sessionID,
          status: "completed",
          text,
          cards: [
            {
              id: `card-${turnID}`,
              type: "digest",
              digest: {
                title: `Decision ${selections.length}`,
                items: [
                  {
                    kind: "measurement",
                    headline: "The report is incomplete",
                    why: "Only six hours are reported.",
                    action: "Recheck after the current day closes.",
                  },
                ],
              },
            },
          ],
        },
      },
    ];
    saved[turnID] = events;
    session.messages.push(
      {
        role: "user",
        text: input.message,
        turn_id: turnID,
        status: "completed",
      },
      { role: "assistant", text, turn_id: turnID, status: "completed" },
    );
    await route.fulfill({
      contentType: "text/event-stream",
      body: events
        .map((event) => `data: ${JSON.stringify(event)}\n\n`)
        .join(""),
    });
  });
  await login(page);
  const composer = page.getByLabel("Your advertising question");
  await composer.fill("Remember the campaign under review");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(page.getByText("Saved answer 1", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  for (const [runtime, model] of [
    ["builtin", "gpt-5.4-mini"],
    ["codex", "gpt-5.6-luna"],
    ["pi", "gpt-5.6-luna"],
  ]) {
    await composer.fill("My unfinished follow-up");
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    await page.getByRole("tab", { name: "Model", exact: true }).click();
    await page.getByLabel("Connection method").selectOption("chatgpt_oauth");
    await page.getByLabel("Model", { exact: true }).selectOption(model!);
    await page.getByRole("tab", { name: "Runtime", exact: true }).click();
    await page.getByLabel("Agent runtime").selectOption(runtime!);
    await page
      .getByRole("button", { name: "Save settings", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(composer).toHaveValue("My unfinished follow-up");
    await expect(
      page.getByRole("heading", { name: "Campaigns", exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Decision 1", { exact: true })).toHaveCount(1);
    expect(
      await page.evaluate(() => localStorage.getItem("ad-agent.session")),
    ).toBe(sessionID);
    await page.reload();
    await expect(
      page.getByText("Saved answer 1", { exact: true }),
    ).toBeVisible();
    await composer.fill("Continue with the same campaign");
    await page.getByRole("button", { name: "Send", exact: true }).click();
    await expect(
      page.getByText(`Saved answer ${selections.length}`, { exact: true }),
    ).toBeVisible();
    expect(selections.at(-1)?.runtime).toBe(runtime);
    expect(selections.at(-1)?.model.model).toBe(model);
    expect(selections.at(-1)?.session_id).toBe(sessionID);
    await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  }
  await expect(page.locator(".briefing-card")).toHaveCount(4);
});

test("settings separate connection, runtime, skills and immutable safety", async ({
  page,
}, info) => {
  await login(page);
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByLabel("Connection method").selectOption("http");
  await page.getByLabel("Provider", { exact: true }).selectOption("deepseek");
  await expect(page.getByLabel("Protocol")).toHaveValue("openai-completions");
  await expect(page.getByLabel("Base URL")).toHaveValue(
    "https://api.deepseek.com",
  );
  await page.getByLabel("Model ID").fill("operator-test-model");
  await page
    .getByLabel("API key", { exact: true })
    .fill("browser-test-key-not-a-real-credential");
  await page.getByRole("tab", { name: "Runtime", exact: true }).click();
  await expect(page.getByLabel("Agent runtime").locator("option")).toHaveCount(
    4,
  );
  await page.getByLabel("Agent runtime").selectOption("codex");
  await expect(
    page.getByText("Choose an OpenAI Responses connection", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("tab", { name: "Model", exact: true }).click();
  await page.getByLabel("Provider", { exact: true }).selectOption("openai");
  await expect(page.getByLabel("Protocol")).toHaveValue("openai-responses");
  await page.getByLabel("Model ID").fill("operator-test-model");
  await page
    .getByLabel("API key", { exact: true })
    .fill("browser-test-key-not-a-real-credential");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("tab", { name: "Model", exact: true }).click();
  await expect(page.getByLabel("Connection method")).toHaveValue("http");
  await expect(page.getByLabel("API key", { exact: true })).toHaveValue("");
  await page.getByRole("tab", { name: "Runtime", exact: true }).click();
  await page.getByLabel("Agent runtime").selectOption("builtin");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("tab", { name: "Runtime", exact: true }).click();
  await expect(page.getByLabel("Agent runtime")).toHaveValue("builtin");
  await expect(
    page.getByRole("option", { name: "Built-in Runtime", exact: true }),
  ).toHaveCount(1);
  await page.getByRole("tab", { name: "Model", exact: true }).click();
  expect(
    await page.evaluate(() =>
      JSON.stringify({ ...localStorage, ...sessionStorage }),
    ),
  ).not.toContain("browser-test-key");
  await page.getByLabel("Connection method").selectOption("chatgpt_oauth");
  await page.getByRole("tab", { name: "Runtime", exact: true }).click();
  await page
    .getByLabel("Agent runtime")
    .selectOption(
      process.env.AD_AGENT_E2E_RUNTIME === "codex" ? "codex" : "pi",
    );
  await page.getByRole("tab", { name: "Ad connection", exact: true }).click();
  await expect(
    page.getByLabel("Ad backend").locator('option[value="meta"]'),
  ).toBeDisabled();
  await page.getByRole("tab", { name: "Guardrails", exact: true }).click();
  await expect(
    page.getByText("Always enforced", { exact: true }),
  ).toBeVisible();
  await page.getByLabel("Maximum change (%)").fill("25");
  await page.screenshot({ path: info.outputPath("settings.png") });
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("tab", { name: "Guardrails", exact: true }).click();
  await expect(page.getByLabel("Maximum change (%)")).toHaveValue("25");
  await page.getByLabel("Maximum change (%)").fill("26");
  await page.getByRole("tab", { name: "Skills", exact: true }).click();
  const skillAlreadyInstalled =
    (await page.getByLabel("campaign-reading", { exact: true }).count()) > 0;
  await page.getByLabel("Upload SKILL.md").setInputFiles({
    name: "SKILL.md",
    mimeType: "text/markdown",
    buffer: Buffer.from(
      "---\nname: campaign-reading\ndescription: Inspect account evidence before proposing a change.\n---\nUse the selected dates and preserve attribution limitations.",
    ),
  });
  await page
    .getByRole("button", { name: "Add skill to settings", exact: true })
    .click();
  if (skillAlreadyInstalled) {
    await expect(page.getByRole("alert")).toContainText("already exists");
  }
  await expect(
    page.getByLabel("campaign-reading", { exact: true }),
  ).toHaveCount(1);
  await expect(
    page.getByLabel("campaign-reading", { exact: true }),
  ).toBeChecked();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("tab", { name: "Skills", exact: true }).click();
  await expect(
    page.getByLabel("campaign-reading", { exact: true }),
  ).toBeChecked();
  await page.getByRole("tab", { name: "Guardrails", exact: true }).click();
  await expect(page.getByLabel("Maximum change (%)")).toHaveValue("26");
  await page.getByRole("button", { name: "Close", exact: true }).click();
});

test("authenticated operating flow and consistent drill-down", async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  await expect(page.locator(".assistant-panel header")).toContainText(
    "GPT-5.6 Luna",
  );
  await page.getByRole("button", { name: "Settings" }).click();
  await expect(page.getByLabel("Model")).toHaveValue("gpt-5.6-luna");
  await expect(page.getByLabel("Model").locator("option")).toHaveCount(7);
  await expect(page.getByLabel("Connection method")).toHaveValue(
    "chatgpt_oauth",
  );
  await page.getByRole("tab", { name: "Skills" }).click();
  await page.getByText(/Built-in business skills/).click();
  await expect(
    page.getByText("account-operations", { exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Upload SKILL.md")).toBeVisible();
  await page.getByRole("button", { name: "Close" }).click();
  await expect(page.getByRole("button", { name: "Sandbox clock" })).toHaveCount(
    1,
  );
  await expect(page.getByRole("button", { name: "+1 hour" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "+24 hours" })).toHaveCount(0);
  await expect(page.getByLabel("Date range")).toHaveValue("7");
  await expect(page.getByText("Compare: previous 7d")).toBeVisible();
  const metrics = page.getByLabel("Performance metrics").first();
  await expect(metrics).toContainText("Spend");
  await expect(metrics).toContainText("Purchase value");
  await expect(metrics).toContainText("ROAS");
  await page.getByRole("button", { name: "Open activity and memory" }).click();
  await expect(
    page.getByRole("heading", { name: "Activity and memory" }),
  ).toBeVisible();
  await expect(
    page.getByText("Nothing saved yet.", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Close" }).click();
  await page.screenshot({
    path: info.outputPath("overview.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
  await page.getByRole("button", { name: "Analyze", exact: true }).click();
  await expect(
    page.getByRole("menuitem", { name: "Audit structure" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("tab", { name: "Performance" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.getByRole("tab", { name: "Structure" }).click();
  await expect(
    page.getByLabel("Campaigns hierarchy").getByRole("button"),
  ).toHaveCount(3);
  await page
    .getByText("US | Home Decor | Prospecting | Purchase", { exact: true })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "US | Home Decor | Prospecting | Purchase",
    }),
  ).toBeVisible();
  const chart = page.getByLabel(/Daily ROAS chart/);
  await expect(chart).toBeVisible();
  await chart.hover({ position: { x: 320, y: 100 } });
  await expect(page.getByRole("status")).toContainText("ROAS");
  await expect(page.getByRole("status")).toContainText("Spend");
  await expect(page.getByRole("status")).toContainText("Value");
  await page.screenshot({
    path: info.outputPath("campaign-overview.png"),
    fullPage: true,
  });
  await page.getByRole("tab", { name: "Ad Groups" }).click();
  await expect(page.locator("main tbody tr")).toHaveCount(3);
  await page
    .getByText("Broad | US | 18-54 | Purchase", { exact: true })
    .click();
  await expect(page.getByRole("tab", { name: "Ads" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.locator("main tbody tr")).toHaveCount(2);
  await page
    .getByText("Room tour | Vintage shelves | Vertical", { exact: true })
    .click();
  await expect(
    page.getByText("Vintage shelf styling · room tour"),
  ).toBeVisible();
  await expect(page.locator("main video")).toBeVisible();
  expect(
    (
      await page.request.get("/sandbox/creatives/vintage-shelves.mp4", {
        headers: { Range: "bytes=0-1023" },
      })
    ).status(),
  ).toBe(206);
  await page.screenshot({
    path: info.outputPath("campaign-detail.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Creatives", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Creatives" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Under-tested" }),
  ).toBeVisible();
  await expect(page.locator("main tbody tr")).toHaveCount(12);
  await expect(page.locator("main tbody img")).toHaveCount(12);
  await page.locator("main tbody tr").first().click();
  await expect(
    page.getByText("Illustrative stock media", { exact: true }),
  ).toBeVisible();
  await page
    .getByText("Illustrative stock media", { exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: info.outputPath("creative-detail.png"),
    fullPage: true,
  });
  const separator = page.getByRole("separator", { name: "Resize assistant" });
  await expect(separator).toBeVisible();
  await page.screenshot({
    path: info.outputPath("creatives.png"),
    fullPage: true,
  });
  const initialWidth = Number(await separator.getAttribute("aria-valuenow"));
  await separator.press("ArrowLeft");
  await expect(separator).toHaveAttribute(
    "aria-valuenow",
    String(initialWidth + 24),
  );
  await page.reload();
  await expect(
    page.getByRole("separator", { name: "Resize assistant" }),
  ).toHaveAttribute("aria-valuenow", String(initialWidth + 24));
  expect(errors).toEqual([]);
});
test("login, CSRF and source file boundary", async ({ page }) => {
  await page.goto("/");
  expect((await page.request.get("/api/v1/advertisers/current")).status()).toBe(
    401,
  );
  expect((await page.request.get("/api/v1/memories")).status()).toBe(401);
  await login(page);
  const response = await page.request.post("/api/v1/agent/turn", {
    headers: { Origin: new URL(page.url()).origin },
    data: { session_id: "web", message: "read" },
  });
  expect(response.status()).toBe(403);
  expect((await page.request.get("/prompts/ad-agent-system.md")).status()).toBe(
    404,
  );
});
test("selected object and equal periods are sent as bounded turn context", async ({
  page,
}) => {
  await login(page);
  const account = await (
    await page.request.get("/api/v1/advertisers/current")
  ).json();
  const offset = (days: number) =>
    new Date(Date.parse(account.latest_date + "T00:00:00Z") + days * 86400000)
      .toISOString()
      .slice(0, 10);
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await page
    .getByText("US | Home Decor | Prospecting | Purchase", { exact: true })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "US | Home Decor | Prospecting | Purchase",
    }),
  ).toBeVisible();

  let submitted: Record<string, unknown> | undefined;
  await page.route("**/api/v1/agent/turn", async (route) => {
    submitted = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: `data: ${JSON.stringify({
        v: "1",
        type: "turn.completed",
        turnId: "turn-context-test",
        seq: 1,
        at: "2026-09-04T00:00:00Z",
        data: {
          turn_id: "turn-context-test",
          session_id: "web",
          status: "completed",
          text: "Context received.",
          cards: [],
          elapsed_ms: 1,
          usage: { input: 0, output: 0, cache_read: 0, cache_write: 0 },
        },
      })}\n\n`,
    });
  });
  await page.getByLabel("Your advertising question").fill("What about this?");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect.poll(() => submitted).toBeTruthy();
  expect(submitted?.view_context).toMatchObject({
    page: "campaigns",
    account_id: "adv_aurora_us",
    entity_level: "campaign",
    entity_id: "campaign_prospect_us",
    start_date: offset(-6),
    end_date: offset(0),
    compare_start: offset(-13),
    compare_end: offset(-7),
  });
  expect(submitted?.message).toBe("What about this?");
});
test("mobile layout stays inside viewport", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  await expect(page.getByLabel("Performance metrics").first()).toContainText(
    "Spend",
  );
  await page.getByRole("button", { name: "Open navigation" }).click();
  await expect(
    page
      .getByRole("dialog")
      .getByRole("heading", { name: "Ad Desk", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await expect(
    page.locator("h1").filter({ hasText: "Campaigns" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Open assistant" }).click();
  await expect(
    page.getByRole("dialog").getByText("Ad Agent", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Your advertising question on mobile"),
  ).toBeVisible();
  await page.getByRole("button", { name: "Close assistant" }).click();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("mobile-overview.png"),
    fullPage: true,
  });
});
test("real Luna: stream, staged preview, explicit approval and reload", async ({
  page,
}, info) => {
  test.skip(
    process.env.AD_AGENT_LIVE_E2E !== "1",
    "Opt-in: consumes ChatGPT quota, local sandbox only",
  );
  test.setTimeout(240_000);
  await login(page);
  await page.getByRole("button", { name: "New session", exact: true }).click();
  const entities = (await (
    await page.request.get("/api/v1/entities/campaign")
  ).json()) as { id: string; budget: string; budget_mode: string }[];
  const entity = entities.find((e) => e.id === "campaign_prospect_us")!;
  const after = String(Number(entity.budget) + 1);
  await page
    .getByLabel("Your advertising question")
    .fill(
      `Read campaign_prospect_us and change its budget amount from ${entity.budget} USD to ${after} USD, preserving the current budget mode. Create exactly one draft and show its approval preview. Do not apply it.`,
    );
  const streamResponse = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/agent/turn") &&
      response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Approve", exact: true }),
  ).toBeVisible({ timeout: 180_000 });
  await expect(
    page.getByRole("button", { name: "Send", exact: true }),
  ).toBeVisible({
    timeout: 180_000,
  });
  const stream = await streamResponse;
  expect(stream.status()).toBe(200);
  expect(stream.headers()["x-request-id"]).toMatch(/^request_/);
  // Chromium may discard a consumed fetch-stream body. Read the authenticated event
  // ledger used by reload/replay instead of relying on Network.getResponseBody.
  const sessionID = stream.request().postDataJSON().session_id as string;
  const session = await (
    await page.request.get(
      `/api/v1/session?session_id=${encodeURIComponent(sessionID)}`,
    )
  ).json();
  expect(session.model.provider).toBe("openai-codex");
  expect(session.model.model).toBe("gpt-5.6-luna");
  const turnID = session.messages.at(-1).turn_id;
  const events: {
    v: string;
    type: string;
    seq: number;
    data: { status?: string; id?: string; name?: string; ok?: boolean };
  }[] = await (
    await page.request.get(
      `/api/v1/turns/${turnID}/events?session_id=${encodeURIComponent(sessionID)}`,
    )
  ).json();
  expect(events[0]?.type).toBe("turn.started");
  const terminal = events.filter((event) => event.type === "turn.completed");
  expect(terminal).toHaveLength(1);
  expect(terminal[0]?.data.status).toBe("completed");
  expect(events.every((event) => event.v === "1")).toBe(true);
  const starts = events.filter((event) => event.type === "tool.started");
  const finishes = events.filter((event) => event.type === "tool.finished");
  expect(starts.length).toBeGreaterThan(0);
  for (const started of starts) {
    expect(
      finishes.filter((event) => event.data.id === started.data.id),
    ).toHaveLength(1);
  }
  expect(
    finishes.some(
      (event) => event.data.name === "stage_budget_change" && event.data.ok,
    ),
  ).toBe(true);
  expect(new Set(events.map((event) => event.seq)).size).toBe(events.length);
  await expect(
    page.getByRole("button", { name: "Approve", exact: true }),
  ).toHaveCount(1);
  const unchanged = (await (
    await page.request.get("/api/v1/entities/campaign")
  ).json()) as { id: string; budget: string }[];
  expect(unchanged.find((e) => e.id === entity.id)?.budget).toBe(entity.budget);
  await page.screenshot({
    path: info.outputPath("assistant-staged.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Approve", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page
    .getByRole("button", { name: "Confirm and apply", exact: true })
    .click();
  await expect(page.getByRole("dialog")).not.toBeVisible();
  await expect(
    page.getByText("Verified", { exact: true }).first(),
  ).toBeVisible();
  const changed = (await (
    await page.request.get("/api/v1/entities/campaign")
  ).json()) as { id: string; budget: string }[];
  expect(changed.find((e) => e.id === entity.id)?.budget).toBe(after);
  await page.screenshot({
    path: info.outputPath("approval-confirmed.png"),
    fullPage: true,
  });
  await page.reload();
  await page.getByRole("button", { name: "Changes", exact: true }).click();
  await expect(
    page.getByText("Verified", { exact: true }).first(),
  ).toBeVisible();
});

test("real Luna: exact creative review and typed operation read-back", async ({
  page,
}, info) => {
  test.skip(
    process.env.AD_AGENT_LIVE_E2E !== "1",
    "Opt-in: consumes ChatGPT quota, local sandbox only",
  );
  test.setTimeout(240_000);
  await login(page);
  await page.getByRole("button", { name: "New session", exact: true }).click();
  const detailURL = "/api/v1/ads/ad_prospect_creator/detail";
  const before = await (await page.request.get(detailURL)).json();
  const copy = "Make room for a calmer corner with a modular shelf.";
  await page
    .getByLabel("Your advertising question")
    .fill(
      `Read ad_prospect_creator and prepare exactly one creative-update draft changing only its primary_text to "${copy}". Preserve identity, asset, CTA, destination and delivery status. Do not apply it.`,
    );
  await page.getByRole("button", { name: "Send", exact: true }).click();
  const review = page.getByLabel("Exact operation changes");
  await expect(review).toBeVisible({ timeout: 180_000 });
  await expect(
    page.getByRole("button", { name: "Send", exact: true }),
  ).toBeVisible({ timeout: 180_000 });
  await expect(review).toContainText(before.primary_text);
  await expect(review).toContainText(copy);
  await expect(review).toContainText("ad_prospect_creator");
  await expect(
    page.getByRole("button", { name: "Approve", exact: true }),
  ).toHaveCount(1);
  expect((await (await page.request.get(detailURL)).json()).primary_text).toBe(
    before.primary_text,
  );
  await page.getByRole("button", { name: "Approve", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByLabel("Exact operation changes")).toContainText(
    before.primary_text,
  );
  await expect(dialog.getByLabel("Exact operation changes")).toContainText(
    copy,
  );
  await page.screenshot({
    path: info.outputPath("creative-approval.png"),
    fullPage: true,
  });
  await dialog
    .getByRole("button", { name: "Confirm and apply", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  await expect(
    page.getByText("Verified", { exact: true }).first(),
  ).toBeVisible();
  const after = await (await page.request.get(detailURL)).json();
  expect(after.primary_text).toBe(copy);
  expect(after.destination_url).toBe(before.destination_url);
  expect(after.call_to_action).toBe(before.call_to_action);
  expect(after.identity).toEqual(before.identity);
  expect(after.creative).toEqual(before.creative);
  expect(after.ad.status).toBe(before.ad.status);
  await page.reload();
  await page.getByRole("button", { name: "Changes", exact: true }).click();
  await expect(
    page.getByText("Verified", { exact: true }).first(),
  ).toBeVisible();
});
