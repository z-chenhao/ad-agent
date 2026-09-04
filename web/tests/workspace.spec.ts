import { test, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
async function login(page: Page) {
  await page.goto("/");
  const dataDir = process.env.AD_AGENT_E2E_RUNTIME === "j" ? "e2e-j" : "e2e";
  const key = (
    await readFile(
      new URL(`../../.data/${dataDir}/operator-key`, import.meta.url),
      "utf8",
    )
  ).trim();
  await page.getByLabel("Local operator key").fill(key);
  await page.getByRole("button", { name: "Enter workspace" }).click();
  await expect(
    page.getByRole("heading", { name: "Account overview", exact: true }),
  ).toBeVisible();
}
test("authenticated overview and consistent hierarchy", async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  await expect(
    page.getByText(
      process.env.AD_AGENT_E2E_RUNTIME === "j" ? "J-agent + Luna" : "Pi + Luna",
      { exact: true },
    ),
  ).toBeVisible();
  const metrics = page.getByLabel("Performance metrics").first();
  await expect(metrics).toContainText("21");
  await expect(metrics).toContainText("35");
  await expect(metrics).toContainText("1.667");
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
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await page.getByText("SDK-Campaign", { exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByText("App install · Prospecting", { exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(2);
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
    headers: { Origin: "http://127.0.0.1:18481" },
    data: { session_id: "web", message: "read" },
  });
  expect(response.status()).toBe(403);
  expect((await page.request.get("/AGENT.md")).status()).toBe(404);
});
test("mobile layout stays inside viewport", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  await expect(page.getByLabel("Performance metrics").first()).toContainText(
    "21",
  );
  await page.getByRole("button", { name: "Open navigation" }).click();
  await page.getByRole("button", { name: "Campaigns", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Campaign hierarchy" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Open assistant" }).click();
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
    "Opt-in: consumes ChatGPT quota, fixture only",
  );
  test.setTimeout(240_000);
  await login(page);
  await page.getByRole("button", { name: "New session", exact: true }).click();
  const entities = (await (
    await page.request.get("/api/v1/entities/campaign")
  ).json()) as { id: string; budget: string }[];
  const entity = entities.find((e) => e.id === "campaign_example_1")!;
  const after = String(Number(entity.budget) + 1);
  await page
    .getByLabel("Your advertising question")
    .fill(
      `Read campaign_example_1 and change its total budget from ${entity.budget} USD to ${after} USD. Create exactly one draft and show its approval preview. Do not apply it.`,
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
