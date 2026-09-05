// Single-user readiness probe: sends fictional data only, does not connect to TikTok, and does not change default model settings.
// Usage: node scripts/check-pi-readiness.mjs /absolute/path/to/pi-coding-agent/dist/index.js
import { randomUUID } from "node:crypto";
import { isAbsolute } from "node:path";
import { pathToFileURL } from "node:url";
import { homedir } from "node:os";
import { readFile } from "node:fs/promises";

function safeError(error) {
  return String(error?.message ?? error ?? "unknown error")
    .replace(/https?:\/\/[^\s]+/g, "[URL_REDACTED]")
    .replace(/(?:Bearer\s+)?eyJ[A-Za-z0-9_.-]+/g, "[TOKEN_REDACTED]")
    .replace(/\b(?:sk-|sk_|sess-)[A-Za-z0-9_-]+/g, "[TOKEN_REDACTED]")
    .replace(
      /(?:access_token|refresh_token|api_key|authorization|auth_code|state)\s*[:=]\s*[^\s,;]+/gi,
      "[CREDENTIAL_REDACTED]",
    )
    .slice(0, 350);
}

let session;
let deadline;
let hardDeadline;
let timedOut = false;
try {
  const entry = process.argv[2];
  if (!entry || !isAbsolute(entry))
    throw new Error("Provide the absolute installed Pi SDK entry path.");
  const entryURL = pathToFileURL(entry);
  const pkg = JSON.parse(
    await readFile(new URL("../package.json", entryURL), "utf8"),
  );
  if (
    pkg.name !== "@earendil-works/pi-coding-agent" ||
    pkg.version !== "0.84.4"
  ) {
    throw new Error(
      "This readiness probe is verified only against Pi 0.84.4; review bootstrap before upgrading.",
    );
  }
  // Pi CLI performs this initialization, but importing the SDK does not. It configures both
  // the proxy dispatcher and matching undici fetch. This probe preserves the existing proxy
  // environment and does not change shell, system proxy, or user model settings.
  const { configureHttpDispatcher } = await import(
    new URL("./core/http-dispatcher.js", entryURL).href
  );
  configureHttpDispatcher();
  const {
    createAgentSession,
    DefaultResourceLoader,
    ModelRuntime,
    SessionManager,
    SettingsManager,
  } = await import(entryURL.href);
  const cwd = process.cwd();
  const agentDir = `${homedir()}/.pi/agent`;
  const runtime = await ModelRuntime.create({
    modelsPath: null,
    allowModelNetwork: false,
    signal: AbortSignal.timeout(10000),
  });
  const model = runtime.getModel("openai-codex", "gpt-5.6-luna");
  if (!model || !runtime.isUsingOAuth("openai-codex"))
    throw new Error("Requested model or OAuth is missing.");
  const settingsManager = SettingsManager.inMemory({
    transport: "sse",
    compaction: { enabled: false },
    retry: {
      enabled: true,
      maxRetries: 1,
      baseDelayMs: 500,
      provider: { maxRetries: 1, maxRetryDelayMs: 2000 },
    },
  });
  const loader = new DefaultResourceLoader({
    cwd,
    agentDir,
    settingsManager,
    noExtensions: true,
    noSkills: true,
    noPromptTemplates: true,
    noThemes: true,
    noContextFiles: true,
    systemPromptOverride: () =>
      "You are a readiness probe. Call get_sandbox_report exactly once. Then reply with exactly the marker from the tool result. All data is fictional. Do nothing else.",
  });
  await loader.reload();
  const marker = `AD_BACKEND_${randomUUID()}`;
  let calls = 0;
  const tool = {
    name: "get_sandbox_report",
    label: "Sandbox report",
    description: "Return a tiny fictional report and a readiness marker.",
    parameters: {
      type: "object",
      properties: {},
      required: [],
      additionalProperties: false,
    },
    execute: async () => {
      calls++;
      return {
        content: [
          {
            type: "text",
            text: JSON.stringify({
              source: "sandbox",
              spend: 100,
              revenue: 200,
              marker,
            }),
          },
        ],
        details: { source: "sandbox" },
      };
    },
  };
  ({ session } = await createAgentSession({
    cwd,
    agentDir,
    model,
    modelRuntime: runtime,
    thinkingLevel: "medium",
    tools: [tool.name],
    customTools: [tool],
    resourceLoader: loader,
    settingsManager,
    sessionManager: SessionManager.inMemory(cwd),
  }));
  const active = session.getActiveToolNames();
  const contextFiles = loader.getAgentsFiles().agentsFiles.length;
  const extensions = loader.getExtensions().extensions.length;
  if (
    active.length !== 1 ||
    active[0] !== tool.name ||
    contextFiles ||
    extensions
  ) {
    throw new Error("Unexpected active tools or discovered resources.");
  }
  deadline = setTimeout(() => {
    timedOut = true;
    void session.abort();
  }, 45000);
  hardDeadline = setTimeout(() => {
    console.error("Probe deadline exceeded.");
    process.exit(124);
  }, 55000);
  await session.prompt("Run the single read-only tool check now.");
  const assistant = session.messages
    .filter((m) => m.role === "assistant")
    .at(-1);
  const answer = (assistant?.content ?? [])
    .filter((b) => b.type === "text")
    .map((b) => b.text)
    .join("")
    .trim();
  const results = session.messages.filter((m) => m.role === "toolResult");
  const callIDs = session.messages
    .filter((m) => m.role === "assistant")
    .flatMap((m) =>
      m.content.filter((b) => b.type === "toolCall").map((b) => b.id),
    );
  const paired =
    callIDs.length === 1 &&
    results.length === 1 &&
    results[0].toolCallId === callIDs[0];
  const passed =
    !timedOut &&
    calls === 1 &&
    paired &&
    answer === marker &&
    assistant?.stopReason === "stop";
  console.log(
    JSON.stringify(
      {
        piVersion: pkg.version,
        httpBootstrap: "pi-cli-dispatcher",
        transport: "sse",
        provider: session.model?.provider,
        model: session.model?.id,
        oauth: true,
        activeTools: active,
        discoveredContextFiles: contextFiles,
        discoveredExtensions: extensions,
        toolCalls: calls,
        toolResults: results.length,
        paired,
        roundTripPassed: passed,
        stopReason: assistant?.stopReason,
        timedOut,
        ...(!passed && assistant?.errorMessage
          ? { error: safeError(assistant.errorMessage) }
          : {}),
      },
      null,
      2,
    ),
  );
  if (!passed) process.exitCode = 1;
} catch (error) {
  console.error(safeError(error));
  process.exitCode = 1;
} finally {
  clearTimeout(deadline);
  clearTimeout(hardDeadline);
  session?.dispose();
}
