import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "portfolio.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:18482",
    headless: true,
    viewport: { width: 1440, height: 1000 },
    trace: "off",
    screenshot: "only-on-failure",
  },
  webServer: {
    command:
      "../bin/ad-agent serve --root .. --scope portfolio --data-dir ../.data/e2e-portfolio --sandbox-environment e2e-portfolio --addr 127.0.0.1:18482",
    url: "http://127.0.0.1:18482/api/v1/health/live",
    reuseExistingServer: false,
    timeout: 20_000,
  },
});
