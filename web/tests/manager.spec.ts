import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

test("manager scope shows isolated advertisers and scoped routes", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await expect(page).toHaveTitle("Ad Desk · Advertising Workspace");
  const dataDir = process.env.AD_AGENT_MANAGER_E2E_DATA_DIR ?? "e2e-manager";
  const key = (
    await readFile(
      new URL(`../../.data/${dataDir}/operator-key`, import.meta.url),
      "utf8",
    )
  ).trim();
  await page.getByLabel("Local operator key").fill(key);
  await page.getByRole("button", { name: "Enter workspace" }).click();
  await expect(page.getByRole("heading", { name: "Accounts" })).toBeVisible();
  await expect(
    page.getByLabel("Workspace toolbar").getByLabel("Account context"),
  ).toHaveCount(1);
  await expect(
    page.getByLabel("Report period").getByLabel("Account context"),
  ).toHaveCount(0);
  await expect(
    page.getByLabel("Workspace toolbar").getByText("Accounts", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText("Ad Desk", { exact: true })).toBeVisible();
  await expect(
    page
      .locator(".assistant-panel header")
      .getByText("Ad Agent", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText(`Manager · ${dataDir}`)).toBeVisible();
  await expect(page.locator("tbody tr")).toHaveCount(3);
  await expect(page.getByText("no cross-currency total").first()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Campaigns", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Open account" }).first().click();
  await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
  await expect(page.getByLabel("Account context")).not.toHaveValue("");
  await expect(page.locator("main tbody tr")).toHaveCount(3);
  await page.getByRole("button", { name: "Back to accounts" }).click();
  await expect(page.getByRole("heading", { name: "Accounts" })).toBeVisible();
  expect((await page.request.get("/api/v1/advertisers/current")).status()).toBe(
    404,
  );
  await page.getByRole("button", { name: "Open activity and memory" }).click();
  await expect(page.getByRole("heading", { name: "Activity" })).toBeVisible();
  await expect(page.getByText("Business memory")).toHaveCount(0);
  expect(errors).toEqual([]);
});
