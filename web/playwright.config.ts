import { defineConfig } from "@playwright/test";
const runtime = process.env.AD_AGENT_E2E_RUNTIME === "j" ? "j" : "pi";
const dataDir = runtime === "j" ? "../.data/e2e-j" : "../.data/e2e";
export default defineConfig({
  testDir: "./tests",
  testIgnore: "portfolio.spec.ts",
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
    command: `../bin/ad-agent serve --root .. --runtime ${runtime} --data-dir ${dataDir} --addr 127.0.0.1:18481`,
    url: "http://127.0.0.1:18481/api/v1/health/live",
    reuseExistingServer: false,
    timeout: 20_000,
  },
});
