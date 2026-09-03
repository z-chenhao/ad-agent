import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:18481",
    headless: true,
    viewport: { width: 1440, height: 1000 },
    trace: "off",
    screenshot: "only-on-failure",
  },
  webServer: {
    command:
      "../bin/ad-agent serve --root .. --data-dir ../.data/e2e --addr 127.0.0.1:18481",
    url: "http://127.0.0.1:18481/api/v1/health/live",
    reuseExistingServer: false,
    timeout: 20_000,
  },
});
