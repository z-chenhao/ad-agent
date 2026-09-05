import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";

test("advancing the shared clock refreshes campaign and creative reports within a day", async ({
  page,
}) => {
  await page.goto("/");
  const dir =
    process.env.AD_AGENT_E2E_DATA_DIR ??
    (process.env.AD_AGENT_E2E_RUNTIME === "codex" ? "e2e-codex" : "e2e");
  const key = (
    await readFile(
      new URL(`../../.data/${dir}/operator-key`, import.meta.url),
      "utf8",
    )
  ).trim();
  await page.getByLabel("Local operator key").fill(key);
  await page.getByRole("button", { name: "Enter workspace" }).click();
  await expect(
    page.getByRole("heading", { name: "Today", exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Performance metrics").first()).toContainText(
    "Spend",
  );
  const original = (await (await page.request.get("/api/v1/settings")).json())
    .settings;
  const { csrf } = await (await page.request.get("/api/v1/auth")).json();
  const save = async (settings: unknown) => {
    // Wait for observable application state above, not global network idleness:
    // media and unrelated browser connections need not finish to save settings.
    const response = await page.request.post("/api/v1/settings", {
      headers: { Origin: new URL(page.url()).origin, "X-CSRF-Token": csrf },
      data: { settings },
    });
    expect(response.status()).toBe(200);
    const { session_id } = await response.json();
    await page.evaluate(
      (id) => localStorage.setItem("ad-agent.session", id),
      session_id,
    );
  };
  await save({
    ...original,
    backend: { kind: "sandbox", environment: "clock-regression" },
  });
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Today", exact: true }),
  ).toBeVisible();
  try {
    // Advance into a partial current day without consuming any model quota.
    await page
      .getByRole("button", { name: "Sandbox clock", exact: true })
      .click();
    await page
      .getByRole("menuitem", { name: "Advance 1 hour", exact: true })
      .click();
    await expect(
      page.getByLabel("Performance metrics").first(),
    ).not.toContainText("Unavailable");
    await expect(
      page.getByText("Partial report · reported to date"),
    ).toBeVisible();
    await page.getByRole("button", { name: "Campaigns", exact: true }).click();
    await expect(page.locator("main tbody tr")).toHaveCount(3);
    const advanceOnPage = async (level: string) => {
      await page
        .getByRole("button", { name: "Sandbox clock", exact: true })
        .click();
      const report = page.waitForResponse(
        (r) =>
          r.url().includes("/api/v1/report?") &&
          new URL(r.url()).searchParams.get("level") === level &&
          r.status() === 200,
      );
      await page
        .getByRole("menuitem", { name: "Advance 1 hour", exact: true })
        .click();
      await report;
    };
    await advanceOnPage("campaign");
    await page.getByRole("button", { name: "Creatives", exact: true }).click();
    await expect(page.locator("main tbody tr")).toHaveCount(12);
    await advanceOnPage("ad");
    await expect(page.locator("main tbody tr")).toHaveCount(12);
    const state = await (await page.request.get("/api/v1/sandbox")).json();
    expect(state.current_time).toBeTruthy();
  } finally {
    await save(original);
  }
});
