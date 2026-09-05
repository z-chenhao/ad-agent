import { useEffect, useState } from "react";
import { api } from "../api";
import type { ModelSelection, RuntimeConfig } from "../types";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./ui/dialog";
import { cn } from "../lib/utils";

interface CustomSkill {
  name: string;
  description: string;
  content: string;
  required_tools: string[];
  scopes: string[];
  enabled: boolean;
}
interface Settings {
  runtime: string;
  model: ModelSelection;
  connection: "chatgpt_oauth" | "openrouter_oauth" | "http";
  backend: {
    kind: string;
    environment: string;
    advertiser_id?: string;
  };
  guardrails: {
    min_budget: string;
    max_budget: string;
    max_delta_percent: string;
  };
  skills: CustomSkill[];
}
interface SettingsResponse {
  settings: Settings;
  runtimes: string[];
  key_ready: boolean;
  openrouter_ready: boolean;
  live_writes: boolean;
  tools: string[];
}
const tabs = [
  "Model",
  "Runtime",
  "Skills",
  "Ad connection",
  "Guardrails",
] as const;
const selectClass =
  "mt-1.5 h-9 w-full rounded-md border border-input bg-background px-3 text-sm";
const directModel = (provider: string): ModelSelection => ({
  provider,
  model: "",
  auth_mode: "api_key",
  reasoning: "medium",
  api:
    provider === "anthropic"
      ? "anthropic-messages"
      : provider === "openai"
        ? "openai-responses"
        : "openai-completions",
  base_url:
    (
      {
        openai: "https://api.openai.com/v1",
        anthropic: "https://api.anthropic.com",
        deepseek: "https://api.deepseek.com",
        openrouter: "https://openrouter.ai/api/v1",
      } as Record<string, string>
    )[provider] ?? "",
  api_key_env: "AD_AGENT_WEB_MODEL_KEY",
  context_window: 131072,
  max_output_tokens: 8192,
});

export function WorkspaceSettings({
  open,
  onOpenChange,
  config,
  busy,
  sessionId,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  config?: RuntimeConfig;
  busy: boolean;
  sessionId: string;
  onSaved: (session: string) => Promise<void>;
}) {
  const [tab, setTab] = useState<(typeof tabs)[number]>("Model");
  const [info, setInfo] = useState<SettingsResponse>();
  const [draft, setDraft] = useState<Settings>();
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [skillContent, setSkillContent] = useState("");
  const [skillTools, setSkillTools] = useState<string[]>([]);
  useEffect(() => {
    if (!open) {
      setKey("");
      return;
    }
    setError("");
    void api<SettingsResponse>("/settings")
      .then((value) => {
        setInfo(value);
        setDraft(value.settings);
        if (sessionStorage.getItem("ad-agent.openrouter-connected") === "1") {
          sessionStorage.removeItem("ad-agent.openrouter-connected");
          if (value.openrouter_ready)
            setDraft({
              ...value.settings,
              connection: "openrouter_oauth",
              model: directModel("openrouter"),
            });
        }
      })
      .catch((reason) => setError(String(reason)));
  }, [open]);
  const model = (patch: Partial<ModelSelection>) =>
    draft && setDraft({ ...draft, model: { ...draft.model, ...patch } });
  const save = async () => {
    if (!draft || busy || saving) return;
    setSaving(true);
    setError("");
    try {
      const result = await api<{ session_id: string }>("/settings", {
        settings: draft,
        session_id: sessionId,
        ...(key ? { api_key: key } : {}),
      });
      setKey("");
      await onSaved(result.session_id);
      onOpenChange(false);
    } catch (reason) {
      setError(String(reason));
    } finally {
      setSaving(false);
    }
  };
  const connect = async () => {
    try {
      const result = await api<{ url: string }>(
        "/settings/openrouter/start",
        {},
      );
      window.location.assign(result.url);
    } catch (reason) {
      setError(String(reason));
    }
  };
  const install = async () => {
    if (!skillContent) return;
    setSaving(true);
    setError("");
    try {
      const installed = await api<CustomSkill>("/settings/skills/preview", {
        content: skillContent,
        required_tools: skillTools,
        scopes: [config?.mode === "manager" ? "manager" : "advertiser"],
      });
      if (draft?.skills.some((skill) => skill.name === installed.name)) {
        setError(
          `A skill named "${installed.name}" already exists. Use a different name to add another skill.`,
        );
        return;
      }
      setDraft((current) =>
        current
          ? { ...current, skills: [...current.skills, installed] }
          : current,
      );
      setSkillContent("");
      setSkillTools([]);
    } catch (reason) {
      setError(String(reason));
    } finally {
      setSaving(false);
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl p-0">
        <header className="border-b border-border px-6 py-5">
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription className="sr-only">
            Configure models, runtimes, skills, ad connections and guardrails.
          </DialogDescription>
        </header>
        <div className="flex min-h-[430px] flex-col sm:flex-row">
          <nav
            className="flex shrink-0 gap-1 overflow-x-auto border-b border-border bg-muted/30 p-3 sm:w-44 sm:flex-col sm:border-r sm:border-b-0"
            role="tablist"
            aria-label="Settings sections"
          >
            {tabs.map((name) => (
              <Button
                key={name}
                role="tab"
                aria-selected={tab === name}
                size="sm"
                variant="ghost"
                className={cn(
                  "justify-start",
                  tab === name && "bg-background shadow-sm",
                )}
                onClick={() => setTab(name)}
              >
                {name}
              </Button>
            ))}
          </nav>
          <div
            className="min-w-0 flex-1 overflow-y-auto px-6 py-5 sm:max-h-[65vh]"
            role="tabpanel"
          >
            {error && (
              <p
                role="alert"
                className="mb-4 rounded-md bg-red-50 p-3 text-xs text-red-800"
              >
                {error.replaceAll("_", " ")}
              </p>
            )}
            {!draft && !error && (
              <p className="text-sm text-muted-foreground">
                Loading workspace settings…
              </p>
            )}
            {draft && tab === "Model" && (
              <div className="space-y-4">
                <Field title="Connection method">
                  <select
                    aria-label="Connection method"
                    className={selectClass}
                    value={draft.connection}
                    onChange={(e) => {
                      const connection = e.target
                        .value as Settings["connection"];
                      setDraft({
                        ...draft,
                        connection,
                        model:
                          connection === "chatgpt_oauth"
                            ? {
                                provider: "openai-codex",
                                model: "gpt-5.6-luna",
                                reasoning: "medium",
                                auth_mode: "chatgpt_oauth",
                              }
                            : directModel(
                                connection === "openrouter_oauth"
                                  ? "openrouter"
                                  : "openai",
                              ),
                      });
                      setKey("");
                    }}
                  >
                    <option value="chatgpt_oauth">OAuth · ChatGPT</option>
                    <option value="openrouter_oauth">OAuth · OpenRouter</option>
                    <option value="http">HTTP · Bring your API key</option>
                  </select>
                </Field>
                {draft.connection === "chatgpt_oauth" ? (
                  <>
                    <Field title="Model">
                      <select
                        aria-label="Model"
                        className={selectClass}
                        value={draft.model.model}
                        onChange={(e) => model({ model: e.target.value })}
                      >
                        {config?.model.options
                          .filter((o) => o.auth_mode === "chatgpt_oauth")
                          .map((option) => (
                            <option key={option.model} value={option.model}>
                              {option.label}
                            </option>
                          ))}
                      </select>
                    </Field>
                    <p className="text-xs leading-relaxed text-muted-foreground">
                      Uses your existing local Pi ChatGPT login. If it has
                      expired, sign in through Pi's /login command. No API key
                      is needed.
                    </p>
                  </>
                ) : (
                  <>
                    {draft.connection === "openrouter_oauth" ? (
                      <div className="flex items-center justify-between rounded-lg border border-border p-3">
                        <span className="text-xs">
                          {info?.openrouter_ready
                            ? "OpenRouter authorized for this server session"
                            : "Authorize OpenRouter with PKCE"}
                        </span>
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={busy || saving}
                          onClick={() => void connect()}
                        >
                          Connect OpenRouter
                        </Button>
                      </div>
                    ) : (
                      <>
                        <Field title="Provider">
                          <select
                            aria-label="Provider"
                            className={selectClass}
                            value={
                              [
                                "openai",
                                "anthropic",
                                "deepseek",
                                "openrouter",
                              ].includes(draft.model.provider)
                                ? draft.model.provider
                                : "custom"
                            }
                            onChange={(e) => {
                              setDraft({
                                ...draft,
                                model: directModel(e.target.value),
                              });
                              setKey("");
                            }}
                          >
                            {[
                              "openai",
                              "anthropic",
                              "deepseek",
                              "openrouter",
                              "custom",
                            ].map((value) => (
                              <option key={value} value={value}>
                                {value}
                              </option>
                            ))}
                          </select>
                        </Field>
                        <div className="grid gap-3 sm:grid-cols-2">
                          <Field title="Provider identifier">
                            <Input
                              aria-label="Provider identifier"
                              className="mt-1.5"
                              value={draft.model.provider}
                              onChange={(e) =>
                                model({ provider: e.target.value })
                              }
                            />
                          </Field>
                          <Field title="Protocol">
                            <select
                              aria-label="Protocol"
                              className={selectClass}
                              value={draft.model.api}
                              onChange={(e) =>
                                model({
                                  api: e.target.value as ModelSelection["api"],
                                })
                              }
                            >
                              <option value="openai-responses">
                                OpenAI Responses
                              </option>
                              <option value="openai-completions">
                                Chat Completions
                              </option>
                              <option value="anthropic-messages">
                                Anthropic Messages
                              </option>
                            </select>
                          </Field>
                        </div>
                        <Field title="Base URL">
                          <Input
                            aria-label="Base URL"
                            className="mt-1.5"
                            value={draft.model.base_url ?? ""}
                            onChange={(e) =>
                              model({ base_url: e.target.value })
                            }
                          />
                        </Field>
                        <Field title="API key">
                          <Input
                            aria-label="API key"
                            type="password"
                            autoComplete="off"
                            className="mt-1.5"
                            value={key}
                            onChange={(e) => setKey(e.target.value)}
                            placeholder={
                              info?.key_ready
                                ? "Leave blank to keep the current connection key"
                                : "Enter a provider API key"
                            }
                          />
                        </Field>
                        <details className="text-xs text-muted-foreground">
                          <summary className="cursor-pointer">
                            Use a server environment variable instead
                          </summary>
                          <Input
                            aria-label="API key environment variable"
                            className="mt-2"
                            value={draft.model.api_key_env ?? ""}
                            onChange={(e) =>
                              model({ api_key_env: e.target.value })
                            }
                          />
                        </details>
                      </>
                    )}
                    <Field title="Model ID">
                      <Input
                        aria-label="Model ID"
                        className="mt-1.5"
                        value={draft.model.model}
                        onChange={(e) => model({ model: e.target.value })}
                        placeholder="Exact model identifier from your provider"
                      />
                    </Field>
                    <details className="text-xs text-muted-foreground">
                      <summary className="cursor-pointer">
                        Declared token limits
                      </summary>
                      <div className="mt-3 grid grid-cols-2 gap-3">
                        <Field title="Context tokens">
                          <Input
                            aria-label="Context tokens"
                            type="number"
                            value={draft.model.context_window}
                            onChange={(e) =>
                              model({ context_window: Number(e.target.value) })
                            }
                          />
                        </Field>
                        <Field title="Max output tokens">
                          <Input
                            aria-label="Max output tokens"
                            type="number"
                            value={draft.model.max_output_tokens}
                            onChange={(e) =>
                              model({
                                max_output_tokens: Number(e.target.value),
                              })
                            }
                          />
                        </Field>
                      </div>
                      <p className="mt-2">
                        {draft.runtime === "codex"
                          ? "Codex applies its native output-token policy; the declared output limit is not enforced by this runtime."
                          : "Use the limits published for your selected model."}
                      </p>
                    </details>
                    <p className="text-xs leading-relaxed text-muted-foreground">
                      Keys are kept in server memory. Reconnect after a server
                      restart, or use a preconfigured environment variable.
                    </p>
                  </>
                )}
              </div>
            )}
            {draft && tab === "Runtime" && (
              <div className="space-y-4">
                <Field title="Agent runtime">
                  <select
                    aria-label="Agent runtime"
                    className={selectClass}
                    value={draft.runtime}
                    onChange={(e) =>
                      setDraft({ ...draft, runtime: e.target.value })
                    }
                  >
                    {info?.runtimes.map((name) => (
                      <option key={name} value={name}>
                        {(
                          {
                            pi: "Pi SDK",
                            builtin: "Built-in Runtime",
                            codex: "Codex App Server",
                            claude: "Claude Agent SDK",
                          } as Record<string, string>
                        )[name] ?? name}
                      </option>
                    ))}
                  </select>
                </Field>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  Built-in Runtime and Pi support all listed connections. Codex
                  supports ChatGPT OAuth and Responses API-key connections.
                  Claude Agent SDK requires an Anthropic Messages API-key
                  connection.
                </p>
                {draft.runtime === "codex" &&
                  draft.model.auth_mode === "api_key" &&
                  draft.model.api !== "openai-responses" && (
                    <p className="rounded-lg bg-amber-50 p-3 text-xs text-amber-900">
                      Choose an OpenAI Responses connection in Model settings,
                      or use Pi for this protocol.
                    </p>
                  )}
                {draft.runtime === "claude" &&
                  draft.model.api !== "anthropic-messages" && (
                    <p className="rounded-lg bg-amber-50 p-3 text-xs text-amber-900">
                      Choose HTTP → Anthropic in Model settings before saving.
                    </p>
                  )}
              </div>
            )}
            {draft && tab === "Skills" && (
              <div className="space-y-5">
                <div>
                  <h3 className="text-sm font-semibold">
                    Your business skills
                  </h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Import one SKILL.md with name and description frontmatter.
                    Markdown guidance only; no scripts, tools, or credentials
                    are installed.
                  </p>
                </div>
                {draft.skills.map((skill) => (
                  <div
                    key={skill.name}
                    className="rounded-lg border border-border p-3"
                  >
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={skill.enabled}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            skills: draft.skills.map((s) =>
                              s.name === skill.name
                                ? { ...s, enabled: e.target.checked }
                                : s,
                            ),
                          })
                        }
                      />
                      {skill.name}
                    </label>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {skill.description}
                    </p>
                  </div>
                ))}
                <Field title="Upload SKILL.md">
                  <input
                    aria-label="Upload SKILL.md"
                    type="file"
                    accept=".md,text/markdown,text/plain"
                    className="mt-2 block w-full text-xs"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) {
                        if (file.size > 32000) {
                          setError("Skill must be at most 32 KB");
                          return;
                        }
                        void file.text().then(setSkillContent);
                      }
                    }}
                  />
                </Field>
                {skillContent && (
                  <>
                    <pre className="max-h-32 overflow-auto rounded-md bg-muted p-3 text-xs">
                      {skillContent}
                    </pre>
                    <Field title="Required installed tools">
                      <select
                        aria-label="Required installed tools"
                        multiple
                        className="mt-2 h-24 w-full rounded-md border border-input p-2 text-xs"
                        value={skillTools}
                        onChange={(e) =>
                          setSkillTools(
                            [...e.target.selectedOptions].map((o) => o.value),
                          )
                        }
                      >
                        {info?.tools.map((tool) => (
                          <option key={tool}>{tool}</option>
                        ))}
                      </select>
                    </Field>
                    <Button
                      size="sm"
                      disabled={busy || saving}
                      onClick={() => void install()}
                    >
                      Add skill to settings
                    </Button>
                  </>
                )}
                <details className="border-t border-border pt-4">
                  <summary className="cursor-pointer text-sm font-medium">
                    Built-in business skills ·{" "}
                    {config?.harness.capabilities.length}
                  </summary>
                  <div className="mt-3 space-y-3">
                    {config?.harness.capabilities.map((skill) => (
                      <div key={skill.name}>
                        <h4 className="text-xs font-medium">{skill.name}</h4>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {skill.description}
                        </p>
                      </div>
                    ))}
                  </div>
                </details>
              </div>
            )}
            {draft &&
              tab === "Ad connection" &&
              draft.backend.kind === "manager" && (
                <p className="text-sm leading-relaxed">
                  Account connections are managed in local configuration. Open
                  an advertiser workspace to view its connection.
                </p>
              )}
            {draft &&
              tab === "Ad connection" &&
              draft.backend.kind !== "manager" && (
                <div className="space-y-4">
                  <Field title="Ad backend">
                    <select
                      aria-label="Ad backend"
                      className={selectClass}
                      value={draft.backend.kind}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          backend: {
                            kind: e.target.value,
                            environment:
                              e.target.value === "sandbox"
                                ? "default"
                                : "sandbox",
                          },
                        })
                      }
                    >
                      <option value="sandbox">
                        Ad Sandbox · simulated data
                      </option>
                      <option value="tiktok">TikTok MAPI</option>
                      <option value="meta" disabled>
                        Meta Ads · unavailable
                      </option>
                    </select>
                  </Field>
                  {draft.backend.kind === "sandbox" ? (
                    <Field title="Sandbox environment">
                      <Input
                        aria-label="Sandbox environment"
                        className="mt-1.5"
                        value={draft.backend.environment}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            backend: {
                              ...draft.backend,
                              environment: e.target.value,
                            },
                          })
                        }
                      />
                      <p className="mt-2 text-xs text-muted-foreground">
                        A new name creates a separate environment. Existing
                        environments are kept.
                      </p>
                    </Field>
                  ) : (
                    <>
                      <Field title="TikTok environment">
                        <select
                          aria-label="TikTok environment"
                          className={selectClass}
                          value={draft.backend.environment}
                          onChange={(e) =>
                            setDraft({
                              ...draft,
                              backend: {
                                ...draft.backend,
                                environment: e.target.value,
                              },
                            })
                          }
                        >
                          <option value="sandbox">
                            TikTok platform sandbox
                          </option>
                          <option value="live">
                            Live advertiser · read only
                          </option>
                        </select>
                      </Field>
                      <Field title="Authorized advertiser ID">
                        <Input
                          aria-label="Authorized advertiser ID"
                          className="mt-1.5"
                          value={draft.backend.advertiser_id ?? ""}
                          onChange={(e) =>
                            setDraft({
                              ...draft,
                              backend: {
                                ...draft.backend,
                                advertiser_id: e.target.value,
                              },
                            })
                          }
                        />
                      </Field>
                      <p className="text-xs leading-relaxed text-muted-foreground">
                        Requires developer approval and advertiser
                        authorization. New connections are read-only.
                      </p>
                    </>
                  )}
                </div>
              )}
            {draft &&
              tab === "Guardrails" &&
              draft.backend.kind === "manager" && (
                <p className="text-sm leading-relaxed">
                  Budget limits belong to each advertiser, not the Manager
                  scope. Configure them in the advertiser workspace. Approval,
                  account isolation, and write verification always apply.
                </p>
              )}
            {draft &&
              tab === "Guardrails" &&
              draft.backend.kind !== "manager" && (
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold">
                    Business budget limits
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    Account currency. Applied at staging, approval execution,
                    and Sandbox scheduled-rule execution.
                  </p>
                  {(
                    [
                      ["min_budget", "Minimum budget"],
                      ["max_budget", "Maximum budget"],
                      ["max_delta_percent", "Maximum change (%)"],
                    ] as const
                  ).map(([field, label]) => (
                    <Field key={field} title={label}>
                      <Input
                        aria-label={label}
                        className="mt-1.5"
                        type="number"
                        min="0.01"
                        step="any"
                        value={draft.guardrails[field]}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            guardrails: {
                              ...draft.guardrails,
                              [field]: e.target.value,
                            },
                          })
                        }
                      />
                    </Field>
                  ))}
                  <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs leading-relaxed">
                    <strong>Always enforced</strong>
                    <p className="mt-1">
                      Explicit approval · account isolation · exact change
                      preview · read-back verification · no blind retry of
                      unknown writes.
                    </p>
                    <p className="mt-2">
                      Live writes are{" "}
                      {info?.live_writes
                        ? "deployment-enabled and approval-gated"
                        : "disabled"}
                      . This page cannot enable them or change platform
                      permissions.
                    </p>
                  </div>
                </div>
              )}
          </div>
        </div>
        <footer className="flex items-center justify-between gap-4 border-t border-border px-6 py-4">
          <p className="text-xs text-muted-foreground">
            Model and runtime changes keep this conversation. Changing the ad
            connection starts a new one.
          </p>
          <Button
            disabled={!draft || saving || busy}
            onClick={() => void save()}
          >
            {saving ? "Saving…" : "Save settings"}
          </Button>
        </footer>
      </DialogContent>
    </Dialog>
  );
}

function Field({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="text-xs font-medium">{title}</div>
      {children}
    </div>
  );
}
