import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

test("portfolio scope shows isolated advertisers and scoped routes", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  const key = (
    await readFile(
      new URL("../../.data/e2e-portfolio/operator-key", import.meta.url),
      "utf8",
    )
  ).trim();
  await page.getByLabel("Local operator key").fill(key);
  await page.getByRole("button", { name: "Enter workspace" }).click();
  await expect(
    page.getByRole("heading", { name: "Advertiser portfolio" }),
  ).toBeVisible();
  await expect(page.getByText("Portfolio · e2e-portfolio")).toBeVisible();
  await expect(page.locator("tbody tr")).toHaveCount(3);
  await expect(page.getByText("no cross-currency total").first()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Campaigns", exact: true }),
  ).toHaveCount(0);
  expect((await page.request.get("/api/v1/advertisers/current")).status()).toBe(
    404,
  );
  await page.getByRole("button", { name: "Open activity and memory" }).click();
  await expect(page.getByRole("heading", { name: "Activity" })).toBeVisible();
  await expect(page.getByText("Business memory")).toHaveCount(0);
  expect(errors).toEqual([]);
});
