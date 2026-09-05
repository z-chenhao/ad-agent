import { useEffect, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Check, ChevronRight, CircleAlert, Loader2 } from "lucide-react";
import type { Live } from "../reducer";
import type { ViewContext } from "../types";

const toolProblems: Record<string, { label: string; detail: string }> = {
  report_budget_exceeded: {
    label: "Read limit reached",
    detail:
      "This turn has reached its report snapshot limit. Reuse collected evidence or narrow the next question; this is not a model quota or advertising budget limit.",
  },
  budget_delta_exceeded: {
    label: "Blocked",
    detail:
      "The requested budget change exceeds the configured percentage limit. Review Guardrails in Settings before preparing a different draft.",
  },
  budget_outside_limits: {
    label: "Blocked",
    detail:
      "The requested budget is outside the configured range. Review Guardrails in Settings.",
  },
  budget_policy_not_configured: {
    label: "Blocked",
    detail:
      "Budget changes require configured account limits. Review Guardrails in Settings.",
  },
  analysis_timeout: {
    label: "Timed out",
    detail:
      "The analysis deadline elapsed. No completed analysis was returned.",
  },
  analysis_cancelled: {
    label: "Cancelled",
    detail: "The analysis was cancelled. Its result is unconfirmed.",
  },
  analysis_runtime_failed: {
    label: "Failed",
    detail: "The analysis runtime failed. No completed analysis was returned.",
  },
  analysis_budget_exhausted: {
    label: "Incomplete",
    detail: "The analysis exhausted its execution allowance before completing.",
  },
  analysis_missing_submission: {
    label: "Incomplete",
    detail: "The analyst ended without submitting a validated result.",
  },
  analysis_interrupted: {
    label: "Interrupted",
    detail: "The analysis runtime did not finish normally.",
  },
  analysis_incomplete: {
    label: "Incomplete",
    detail:
      "No completed analysis was returned. This older event has no detailed reason.",
  },
  analysis_delegate_limit: {
    label: "Blocked",
    detail: "This turn has already used its analysis delegates.",
  },
};

export function formatToolDuration(ms: number): string {
  if (ms < 1) return "<1ms";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60_000)}m ${Math.floor((ms % 60_000) / 1000)}s`;
}

function toolLabel(name: string, names: Record<string, string>) {
  return (
    names[name] ??
    name.replaceAll("_", " ").replace(/^./, (letter) => letter.toUpperCase())
  );
}

export function ToolActivityRow({
  tool,
  names,
  running = false,
  nested = false,
}: {
  tool: Live["tools"][number];
  names: Record<string, string>;
  running?: boolean;
  nested?: boolean;
}) {
  const problem =
    tool.ok === false ? toolProblems[tool.error ?? ""] : undefined;
  const status =
    tool.ok === undefined
      ? running
        ? "Running"
        : "Unconfirmed"
      : tool.ok
        ? "Done"
        : (problem?.label ?? "Unsuccessful");
  const duration =
    tool.duration_ms !== undefined
      ? formatToolDuration(tool.duration_ms)
      : running && tool.ok === undefined && tool.started_at
        ? formatToolDuration(
            Math.max(0, Date.now() - Date.parse(tool.started_at)),
          )
        : undefined;
  return (
    <div>
      <div className="flex items-start justify-between gap-3">
        <span className="break-words">
          {toolLabel(tool.name, names)}
          {tool.role === "analysis" && !nested && (
            <span className="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
              Analysis
            </span>
          )}
        </span>
        <span
          className="shrink-0 tabular-nums text-muted-foreground"
          title="Tool execution time; excludes model response time"
        >
          {status}
          {duration ? ` · ${duration}` : ""}
        </span>
      </div>
      {tool.ok === false && (
        <p className="mt-1 leading-relaxed text-muted-foreground">
          {problem?.detail ??
            "This call failed. The agent may correct the request or use other evidence."}
        </p>
      )}
    </div>
  );
}

export function AgentMarkdown({ text }: { text: string }) {
  return (
    <div className="agent-markdown">
      <Markdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          a: ({ children, href }) => (
            <a href={href} target="_blank" rel="noopener noreferrer">
              {children}
            </a>
          ),
          img: ({ alt }) => (
            <span className="text-muted-foreground">
              {alt || "Image omitted"}
            </span>
          ),
          table: ({ children }) => (
            <div className="overflow-x-auto">
              <table>{children}</table>
            </div>
          ),
        }}
      >
        {text}
      </Markdown>
    </div>
  );
}

export function TurnContext({ context }: { context?: ViewContext }) {
  if (!context) return null;
  const levels: Record<string, string> = {
    campaign: "Campaign",
    ad_group: "Ad group",
    ad: "Ad",
    advertiser: "Account",
  };
  const rows = [
    ["Account", context.account_name || context.account_id],
    [
      levels[context.entity_level ?? ""] || "Object",
      context.entity_name || context.entity_id,
    ],
    [
      "Period",
      [context.start_date, context.end_date].filter(Boolean).join(" — "),
    ],
    [
      "Compare",
      [context.compare_start, context.compare_end].filter(Boolean).join(" — "),
    ],
  ].filter(([, value]) => value);
  if (!rows.length) return null;
  return (
    <details
      className="turn-context mt-1.5 text-xs text-muted-foreground"
      aria-label="Message context"
    >
      <summary className="ml-auto w-fit cursor-pointer rounded-sm py-1 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring">
        Context
      </summary>
      <dl className="mt-1 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 rounded-lg bg-muted/40 px-3 py-2 text-left leading-relaxed">
        {rows.map(([label, value], index) => (
          <div key={`${label}-${index}`} className="contents">
            <dt>{label}</dt>
            <dd className="min-w-0 break-words text-foreground">{value}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

export function TurnActivity({
  turn,
  names,
  running = false,
  seconds = 0,
}: {
  turn: Live;
  names: Record<string, string>;
  running?: boolean;
  seconds?: number;
}) {
  const [open, setOpen] = useState(running);
  useEffect(() => setOpen(running), [running]);
  const failedTurn = ["failed", "cancelled", "budget_exhausted"].includes(
    turn.status,
  );
  if (
    !turn.tools.length &&
    !turn.progress.length &&
    !turn.activity.length &&
    !failedTurn &&
    !running
  )
    return null;
  const failed = turn.tools.filter((tool) => tool.ok === false).length;
  const activeTool = turn.tools.findLast((tool) => tool.ok === undefined);
  const last = turn.activity.at(-1);
  // Public text is interleaved with tool calls. Only the final answer is rendered
  // outside activity after settlement; earlier commentary survives event replay.
  const activity = turn.activity.filter(
    (item, index) =>
      item.kind !== "message" ||
      running ||
      index !== turn.activity.length - 1 ||
      item.text.trim() !== turn.text.trim(),
  );
  const phase = activeTool
    ? toolLabel(activeTool.name, names)
    : last?.kind === "message"
      ? "Responding"
      : "Working";
  return (
    <div className="turn-activity" aria-label="Agent activity">
      <button
        type="button"
        className="flex w-full items-center gap-2 py-2 text-left text-xs text-muted-foreground"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        {running ? (
          <Loader2 className="size-3.5 animate-spin" />
        ) : failed || failedTurn ? (
          <CircleAlert className="size-3.5" />
        ) : (
          <Check className="size-3.5" />
        )}
        <span className="min-w-0 flex-1">
          {running
            ? phase
            : `${failedTurn ? "Incomplete · " : ""}${turn.tools.length} tool calls${failed ? ` · ${failed} unsuccessful` : ""}`}
        </span>
        <span className="shrink-0 tabular-nums">
          {running ? seconds : Math.round((turn.elapsed ?? 0) / 1000)}s
        </span>
        <ChevronRight
          className={`size-3.5 shrink-0 transition-transform ${open ? "rotate-90" : ""}`}
        />
      </button>
      {failedTurn && turn.error_code && (
        <p
          className="mb-2 text-xs leading-relaxed text-muted-foreground"
          role="status"
        >
          {runtimeFailureText(turn.error_code)}
        </p>
      )}
      {open && (
        <div className="mb-2 ml-1.5 space-y-2 border-l border-border pl-4 text-xs">
          {activity.map((item) => {
            if (item.kind === "message")
              return (
                <div
                  key={item.id}
                  className="activity-commentary py-1 text-sm leading-relaxed"
                >
                  <AgentMarkdown text={item.text} />
                </div>
              );
            const tool = turn.tools.find((tool) => tool.id === item.id);
            if (
              tool?.parent_id &&
              turn.tools.some((parent) => parent.id === tool.parent_id)
            )
              return null;
            const children = turn.tools.filter(
              (child) => child.parent_id === item.id,
            );
            return tool ? (
              <div key={item.id}>
                <ToolActivityRow tool={tool} names={names} running={running} />
                {children.length > 0 && (
                  <details
                    className="analysis-activity mt-2 ml-1 border-l border-border pl-3"
                    open={running && tool.ok === undefined}
                  >
                    <summary className="cursor-pointer text-muted-foreground">
                      Analysis steps · {children.length}
                      {children.some((child) => child.ok === false)
                        ? ` · ${children.filter((child) => child.ok === false).length} unsuccessful`
                        : ""}
                    </summary>
                    <div className="mt-2 space-y-2">
                      {children.map((child) => (
                        <ToolActivityRow
                          key={child.id}
                          tool={child}
                          names={names}
                          running={running}
                          nested
                        />
                      ))}
                    </div>
                  </details>
                )}
              </div>
            ) : null;
          })}
        </div>
      )}
    </div>
  );
}

function runtimeFailureText(code: string): string {
  if (code === "runtime_checkpoint_invalid")
    return "The saved runtime state could not be restored. The conversation is retained; the next request rebuilds context from saved public records.";
  if (
    [
      "chatgpt_oauth_required",
      "oauth_or_model_missing",
      "provider_auth_failed",
      "api_key_missing",
      "oauth_account_changed",
    ].includes(code)
  )
    return "The selected runtime could not authenticate. Check the model connection in Settings.";
  if (code === "provider_rate_limited")
    return "The model provider reached a usage or rate limit. Check your provider allowance before retrying.";
  if (
    [
      "provider_timeout",
      "runtime_timeout",
      "provider_transport_failed",
    ].includes(code)
  )
    return "The model connection timed out or was interrupted. This turn was not automatically retried.";
  if (code === "runtime_cancelled")
    return "Execution was cancelled. Review completed tool activity before starting another request.";
  return "The runtime could not finish the model exchange. The failure category is saved in the turn trace.";
}
