import { defineConfig } from "@playwright/test";

const dataDirName = process.env.AD_AGENT_MANAGER_E2E_DATA_DIR ?? "e2e-manager";
const port = process.env.AD_AGENT_MANAGER_E2E_PORT ?? "18482";
if (!/^[a-z0-9_-]+$/i.test(dataDirName) || !/^\d{2,5}$/.test(port)) {
  throw new Error("Invalid Manager E2E data directory or port");
}
const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./tests",
  testMatch: "manager.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  use: {
    baseURL,
    headless: true,
    viewport: { width: 1440, height: 1000 },
    trace: "off",
    screenshot: "only-on-failure",
  },
  webServer: {
    command:
      `../bin/ad-agent serve --root .. --scope manager --data-dir ../.data/${dataDirName} ` +
      `--sandbox-environment ${dataDirName} --addr 127.0.0.1:${port}`,
    url: `${baseURL}/api/v1/health/live`,
    reuseExistingServer: false,
    timeout: 20_000,
  },
});
