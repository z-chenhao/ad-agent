// Credential-free source-install acceptance. No chat, provider or live-ad calls.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { chmodSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const temporary = mkdtempSync(join(tmpdir(), "ad-agent-smoke-"));
chmodSync(temporary, 0o700);
// Do not inherit provider credentials, proxy settings, or the operator's home.
const env = {
  PATH: process.env.PATH,
  HOME: temporary,
  TMPDIR: temporary,
  LANG: "C.UTF-8",
};

function cli(command, extra = []) {
  const output = execFileSync(
    join(root, "bin", "ad-agent"),
    [
      command,
      "--root",
      root,
      "--backend",
      "sandbox",
      "--data-dir",
      join(temporary, "state"),
      "--sandbox-environment",
      "source-install",
      ...extra,
    ],
    {
      cwd: root,
      env,
      encoding: "utf8",
      timeout: 60_000,
      maxBuffer: 8 * 1024 * 1024,
    },
  );
  return JSON.parse(output);
}

try {
  const campaigns = cli("inspect");
  assert.ok(campaigns.length > 0, "Fresh Sandbox must contain campaigns");
  assert.ok(campaigns.every((item) => item.id && item.level === "campaign"));
  const args = ["--start", "2026-08-28", "--end", "2026-09-03"];
  const report = cli("report", args);
  assert.equal(report.source.backend, "sandbox");
  assert.equal(report.source.environment, "source-install");
  assert.ok(report.currency && report.timezone && report.attribution);
  assert.ok(
    report.rows.length > 0,
    "Seed window must contain reported evidence",
  );
  assert.ok(Number(report.totals.spend) > 0);
  assert.ok(report.totals.impressions >= report.totals.clicks);
  assert.ok(
    report.rows.every((row) =>
      campaigns.some((item) => item.id === row.entity_id),
    ),
  );
  // A separate CLI process reopens the same store. Ignore volatile fetch metadata.
  const reopened = cli("report", args);
  assert.deepEqual(
    reopened.rows,
    report.rows,
    "Restart must preserve report facts",
  );
  assert.deepEqual(reopened.totals, report.totals);
  assert.equal(
    (cli("changes") ?? []).length,
    0,
    "Browsing must not stage changes",
  );
  const harness = cli("verify");
  assert.equal(harness.ok, true);
  assert.equal(harness.final_status, "completed");
  assert.ok(harness.tool_calls > 0 && harness.cards > 0);
  console.log(
    "PASS: fresh Sandbox, report provenance, restart, empty change ledger, harness events/cards.",
  );
  console.log(
    "No model credentials or provider quota used. Browser and live integrations are separate gates.",
  );
} finally {
  // Only this invocation's newly created temporary directory is removed.
  rmSync(temporary, { recursive: true, force: true });
}
