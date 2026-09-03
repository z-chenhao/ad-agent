import { test, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
async function login(page: Page) {
  await page.goto("/");
  const key = (
    await readFile(
      new URL("../../.data/e2e/operator-key", import.meta.url),
      "utf8",
    )
  ).trim();
  await page.getByLabel("本机操作员密钥").fill(key);
  await page.getByRole("button", { name: "进入工作台" }).click();
  await expect(
    page.getByRole("heading", { name: "账户概览", exact: true }),
  ).toBeVisible();
}
test("authenticated overview and consistent hierarchy", async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  await expect(page.locator(".metrics strong").first()).toHaveText("21");
  await expect(page.locator(".metrics strong").nth(1)).toHaveText("35");
  await expect(page.locator(".metrics strong").nth(2)).toHaveText("1.667");
  await page.screenshot({
    path: info.outputPath("overview.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "广告层级", exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await page.getByRole("button", { name: "SDK-Campaign", exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: /App install · Prospecting/ }).click();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  expect(errors).toEqual([]);
});
test("login, CSRF and source file boundary", async ({ page }) => {
  await page.goto("/");
  expect((await page.request.get("/api/v1/advertisers/current")).status()).toBe(
    401,
  );
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
  await expect(page.locator(".metrics strong").first()).toHaveText("21");
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
  await page.getByRole("button", { name: "分析助手", exact: true }).click();
  await page.getByRole("button", { name: "新会话", exact: true }).click();
  const entities = (await (
    await page.request.get("/api/v1/entities/campaign")
  ).json()) as { id: string; budget: string }[];
  const entity = entities.find((e) => e.id === "campaign_example_1")!;
  const after = String(Number(entity.budget) + 1);
  await page
    .getByLabel("你的广告问题")
    .fill(
      `读取 campaign_example_1，把总预算从 ${entity.budget} USD 改为 ${after} USD。仅创建一个草案并展示审批预览，不要执行。`,
    );
  await page.getByRole("button", { name: "发送 ↑" }).click();
  await expect(
    page.getByRole("heading", { name: "变更预览", exact: true }),
  ).toBeVisible({ timeout: 180_000 });
  await expect(page.getByRole("button", { name: "发送 ↑" })).toBeVisible({
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
  await page.getByRole("button", { name: "查看审批队列", exact: true }).click();
  await page.getByRole("button", { name: "审批此变更", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page
    .getByRole("button", { name: "确认并执行此变更", exact: true })
    .click();
  await expect(page.getByRole("dialog")).not.toBeVisible();
  await expect(page.locator(".change-item .status")).toHaveText("已核对");
  const changed = (await (
    await page.request.get("/api/v1/entities/campaign")
  ).json()) as { id: string; budget: string }[];
  expect(changed.find((e) => e.id === entity.id)?.budget).toBe(after);
  await page.screenshot({
    path: info.outputPath("approval-confirmed.png"),
    fullPage: true,
  });
  await page.reload();
  await page.getByRole("button", { name: "变更审批", exact: true }).click();
  await expect(page.locator(".change-item .status")).toHaveText("已核对");
});
