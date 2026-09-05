import React, { useEffect, useReducer, useRef, useState } from "react";
import {
  Activity,
  ArrowLeft,
  BarChart3,
  Bot,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  CircleAlert,
  Clock3,
  Command,
  FileCheck2,
  Gauge,
  ImageIcon,
  Layers3,
  LayoutList,
  Menu,
  MessageSquare,
  PanelRightClose,
  PanelRightOpen,
  Plus,
  RefreshCw,
  Send,
  Settings2,
  ShieldCheck,
  Sparkles,
  Square,
  TrendingDown,
  TrendingUp,
  X,
} from "lucide-react";
import { api, onSessionExpired, setCSRF, streamTurn } from "./api";
import { emptyLive, reduceEvent } from "./reducer";
import type {
  Account,
  AdDetail,
  Calculation,
  Card as CardRecord,
  Change,
  Entity,
  Event,
  Memory,
  Metrics,
  Report,
  RuntimeConfig,
  SandboxAdvance,
  SandboxState,
  ModelSelection,
  ManagerScope,
  ManagerReport,
  Session,
  ViewContext,
} from "./types";
import { Badge } from "./components/ui/badge";
import { Button } from "./components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "./components/ui/dialog";
import { Input } from "./components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./components/ui/dropdown-menu";
import { ScrollArea } from "./components/ui/scroll-area";
import { Separator } from "./components/ui/separator";
import { Textarea } from "./components/ui/textarea";
import { cn } from "./lib/utils";
import {
  OperationReview,
  operationTarget,
} from "./components/operation-review";
import {
  format,
  MetricStrip,
  PresentationCard,
  stateText,
} from "./components/presentation";
import { CreativePreview } from "./components/creative-preview";
import { RoasTrendChart } from "./components/trend-chart";
import {
  AgentMarkdown,
  TurnActivity,
  TurnContext,
  ToolActivityRow,
} from "./components/agent-message";
import { WorkspaceSettings } from "./components/workspace-settings";

type Page = "today" | "accounts" | "campaigns" | "creatives" | "changes";
type ChangeAction = "apply" | "discard" | "reconcile";

const directPages: {
  id: Page;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}[] = [
  { id: "today", label: "Today", icon: Gauge },
  { id: "campaigns", label: "Campaigns", icon: LayoutList },
  { id: "creatives", label: "Creatives", icon: ImageIcon },
  { id: "changes", label: "Changes", icon: FileCheck2 },
];
const managerPages: typeof directPages = [
  { id: "accounts", label: "Accounts", icon: LayoutList },
  { id: "campaigns", label: "Campaigns", icon: Layers3 },
  { id: "creatives", label: "Creatives", icon: ImageIcon },
  { id: "changes", label: "Changes", icon: FileCheck2 },
];

const toolNames: Record<string, string> = {
  get_advertiser_context: "Read account",
  list_campaigns: "Read campaigns",
  list_ad_groups: "Read ad groups",
  list_ads: "Read ads",
  get_entity: "Verify object",
  list_identities: "Read identities",
  list_creative_assets: "Read creative assets",
  get_creative_review: "Read creative review",
  list_audiences: "Read audiences",
  get_audience: "Read audience",
  get_audience_overlap: "Read audience overlap",
  get_targeting_options: "Read targeting options",
  list_event_sources: "Read event sources",
  get_event_stats: "Read event activity",
  get_optimization_events: "Read optimization events",
  get_attribution_settings: "Read attribution settings",
  list_lead_forms: "Read lead forms",
  get_lead_form: "Read lead form",
  list_catalogs: "Read catalogs",
  get_catalog_feed_health: "Read catalog feed health",
  get_catalog_product_health: "Read catalog product health",
  list_automated_rules: "Read automated rules",
  get_automated_rule_results: "Read rule history",
  list_comments: "Read ad comments",
  get_billing_balance: "Read billing balance",
  list_billing_transactions: "Read billing transactions",
  get_performance_report: "Read performance",
  read_conversation: "Read conversation",
  run_analysis: "Analyze performance",
  analysis_calculate: "Calculate metrics",
  analysis_get_dataset: "Read snapshot",
  analysis_slice: "Filter data",
  report_progress: "Report progress",
  submit_analysis: "Submit analysis",
  present_metrics: "Present metrics",
  present_entities: "Present objects",
  present_digest: "Present briefing",
  present_change_preview: "Present change",
  present_suggestions: "Offer next steps",
  load_skill: "Read guidance",
  stage_budget_change: "Stage budget change",
  stage_status_change: "Stage status change",
  stage_entity_create: "Stage object creation",
  stage_campaign_bundle: "Stage campaign bundle",
  stage_ad_group_update: "Stage ad group update",
  stage_ad_creative_update: "Stage creative update",
  stage_audience_create: "Stage audience creation",
  stage_automated_rule_create: "Stage automated rule",
  stage_comment_action: "Stage comment action",
  stage_event_source_create: "Stage event source",
  get_pending_changes: "Read changes",
  list_advertisers: "Read advertisers",
  get_manager_performance: "Compare account performance",
  run_manager_analysis: "Delegate multi-account analysis",
  list_account_entities: "Read account objects",
  get_account_entity: "Verify account object",
  stage_account_budget_change: "Stage account budget change",
  stage_account_status_change: "Stage account status change",
  stage_account_entity_create: "Stage account object creation",
};

function modelSelectionKey(
  model: Pick<ModelSelection, "provider" | "model" | "auth_mode" | "api">,
) {
  return [model.provider, model.model, model.auth_mode, model.api ?? ""].join(
    "|",
  );
}

const titleCase = (value: string) =>
  value
    .split(/[-_]/)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");

const changeTarget = (change: Change) =>
  change.before?.name ??
  change.created?.name ??
  change.create?.name ??
  change.operation?.lines[0]?.name ??
  change.operation?.lines[0]?.id ??
  operationTarget(change);

const changeKindLabel = (change: Change) =>
  change.kind === "operation"
    ? titleCase(change.operation?.request.kind ?? "advertising operation")
    : change.kind === "create"
      ? "Create"
      : change.kind === "budget"
        ? "Budget"
        : "Delivery status";

const changeSummary = (change: Change) =>
  change.kind === "budget"
    ? `${format(change.before?.budget)} → ${format(change.after?.budget)} ${change.currency}`
    : change.kind === "status"
      ? `${change.before?.status} → ${change.after?.status}`
      : change.kind === "operation"
        ? changeKindLabel(change)
        : `Create ${change.create?.level ?? "object"}`;

type ReportBundle = { report: Report; calculation: Calculation | null };

type Period = {
  days: 7 | 14;
  start: string;
  end: string;
  compareStart: string;
  compareEnd: string;
};

type OverviewData = {
  current: ReportBundle;
  previous: ReportBundle;
  period: Period;
};

type AgentSurface = {
  page: Page;
  title: string;
  subtitle: string;
  accountId?: string;
  entityLevel?: "campaign" | "ad_group" | "ad";
  entityId?: string;
  finding?: {
    label: string;
    value: string;
    detail: string;
    tone: "warning" | "success" | "muted";
  };
  drivers?: { label: string; value: string; magnitude: number }[];
  confidence?: { label: string; detail: string };
  recommendation?: string;
  actionLabel?: string;
  actionPrompt?: string;
};

const metricNumber = (value: string | null | undefined) =>
  value == null ? null : Number(value);

const sumMetrics = (rows: Report["rows"]): Metrics => {
  let spend = 0;
  let impressions = 0;
  let clicks = 0;
  let conversions = 0;
  let revenue = 0;
  let hasConversions = true;
  let hasRevenue = true;
  for (const row of rows) {
    spend += Number(row.metrics.spend);
    impressions += row.metrics.impressions;
    clicks += row.metrics.clicks;
    if (row.metrics.conversions == null) hasConversions = false;
    else conversions += Number(row.metrics.conversions);
    if (row.metrics.revenue == null) hasRevenue = false;
    else revenue += Number(row.metrics.revenue);
  }
  return {
    spend: spend.toFixed(2),
    impressions,
    clicks,
    conversions: hasConversions ? conversions.toFixed(1) : null,
    revenue: hasRevenue ? revenue.toFixed(2) : null,
  };
};

const entityMetrics = (report: Report | undefined, entityId: string) =>
  report
    ? sumMetrics(report.rows.filter((row) => row.entity_id === entityId))
    : undefined;

const ratio = (numerator: number | null, denominator: number | null) =>
  numerator == null || denominator == null || denominator === 0
    ? null
    : numerator / denominator;

const roas = (metrics?: Metrics) =>
  metrics
    ? ratio(metricNumber(metrics.revenue), metricNumber(metrics.spend))
    : null;

const cpa = (metrics?: Metrics) =>
  metrics
    ? ratio(metricNumber(metrics.spend), metricNumber(metrics.conversions))
    : null;

const ctr = (metrics?: Metrics) =>
  metrics ? ratio(metrics.clicks, metrics.impressions) : null;

const cvr = (metrics?: Metrics) =>
  metrics ? ratio(metricNumber(metrics.conversions), metrics.clicks) : null;

const cpm = (metrics?: Metrics) =>
  metrics
    ? ratio(metricNumber(metrics.spend), metrics.impressions / 1000)
    : null;

const deltaPercent = (current: number | null, previous: number | null) =>
  current == null || previous == null || previous === 0
    ? null
    : ((current - previous) / previous) * 100;

const signedPercent = (value: number | null) =>
  value == null ? "Unavailable" : `${value > 0 ? "+" : ""}${format(value, 1)}%`;

function periodFor(latest: string, days: 7 | 14): Period {
  const end = new Date(latest + "T00:00:00Z");
  const start = new Date(end);
  start.setUTCDate(start.getUTCDate() - (days - 1));
  const compareEnd = new Date(start);
  compareEnd.setUTCDate(compareEnd.getUTCDate() - 1);
  const compareStart = new Date(compareEnd);
  compareStart.setUTCDate(compareStart.getUTCDate() - (days - 1));
  return {
    days,
    start: start.toISOString().slice(0, 10),
    end: latest,
    compareStart: compareStart.toISOString().slice(0, 10),
    compareEnd: compareEnd.toISOString().slice(0, 10),
  };
}

const reportPath = (
  base: string,
  level: string,
  period: Pick<Period, "start" | "end">,
  entityId?: string,
) =>
  `${base}/report?level=${level}&start_date=${period.start}&end_date=${period.end}${entityId ? `&entity_id=${encodeURIComponent(entityId)}` : ""}`;

function Login({
  onReady,
  expired = false,
}: {
  onReady: () => void;
  expired?: boolean;
}) {
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <main className="grid min-h-screen bg-background lg:grid-cols-[1.1fr_.9fr]">
      <section className="hidden border-r border-border bg-muted/30 p-12 lg:flex lg:flex-col lg:justify-between">
        <Brand />
        <div className="max-w-xl">
          <p className="mb-5 text-xs font-medium uppercase tracking-[.18em] text-muted-foreground">
            Local advertising workspace
          </p>
          <h1 className="text-5xl font-semibold leading-[1.08] tracking-[-.045em]">
            Evidence first.
            <br />
            Every change reviewed.
          </h1>
          <p className="mt-6 max-w-md text-base leading-relaxed text-muted-foreground">
            Work with Ad Agent to analyze advertising performance, prepare
            bounded changes, and keep approval with the operator.
          </p>
        </div>
        <p className="text-xs text-muted-foreground">
          Single user · local first · model credentials stay on this machine
        </p>
      </section>
      <section className="flex items-center justify-center p-6">
        <div className="w-full max-w-sm">
          <div className="mb-12 lg:hidden">
            <Brand />
          </div>
          <p className="text-xs font-medium uppercase tracking-[.16em] text-muted-foreground">
            Sign in
          </p>
          <h2 className="mt-3 text-2xl font-semibold tracking-tight">
            Open Ad Desk
          </h2>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
            Use the operator key created by the local server. Model and TikTok
            credentials never enter this page.
          </p>
          <form
            className="mt-8 space-y-4"
            onSubmit={async (event) => {
              event.preventDefault();
              setBusy(true);
              setError("");
              try {
                const value = await api<{ csrf: string }>("/login", { key });
                setCSRF(value.csrf);
                setKey("");
                onReady();
              } catch (reason) {
                setKey("");
                setError(
                  reason instanceof Error &&
                    reason.message === "login_rate_limited"
                    ? "Too many login attempts. Wait a minute before trying again."
                    : "Login failed. Check the key in the local operator-key file.",
                );
              } finally {
                setBusy(false);
              }
            }}
          >
            {expired && (
              <p role="status" className="text-sm text-muted-foreground">
                Your local session expired. Sign in again to recover saved
                conversations. No operation was automatically retried. Review
                saved changes before retrying.
              </p>
            )}
            <div>
              <label
                className="mb-2 block text-sm font-medium"
                htmlFor="operator-key"
              >
                Local operator key
              </label>
              <Input
                id="operator-key"
                type="password"
                value={key}
                onChange={(event) => setKey(event.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
            <Button className="w-full" size="lg" disabled={busy}>
              {busy ? "Verifying…" : "Enter workspace"}
              <ChevronRight />
            </Button>
          </form>
          <p className="mt-5 text-xs leading-relaxed text-muted-foreground">
            The startup terminal shows the key file path. Never send the key to
            chat or commit it.
          </p>
        </div>
      </section>
    </main>
  );
}

function Brand() {
  return (
    <div className="flex items-center gap-2.5">
      <div className="flex size-8 items-center justify-center rounded-lg bg-foreground text-background">
        <Command className="size-4" />
      </div>
      <div>
        <div className="text-sm font-semibold tracking-tight">Ad Desk</div>
        <div className="text-xs uppercase tracking-[.16em] text-muted-foreground">
          Workspace
        </div>
      </div>
    </div>
  );
}

// Context identifies the next turn's scope. Findings and actions belong to the
// workspace or the conversation, not to a second dashboard in the assistant rail.
function ContextIntelligence({
  surface,
  period,
}: {
  surface: AgentSurface;
  period?: Period;
}) {
  return (
    <details className="current-context border-b border-border bg-muted/30 px-4 py-3">
      <summary className="cursor-pointer list-none">
        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span>Current context</span>
          <ChevronDown className="size-3.5 shrink-0 transition-transform" />
        </div>
        <div
          className="mt-1 truncate text-sm font-medium"
          title={surface.title}
        >
          {surface.title}
        </div>
      </summary>
      <div className="context-details mt-3 space-y-2 text-xs text-muted-foreground">
        <p className="break-words">{surface.subtitle}</p>
        {surface.entityId && (
          <p className="break-all font-mono">{surface.entityId}</p>
        )}
        {period && (
          <p>
            {period.start} — {period.end}
          </p>
        )}
        <p>Included with your next message.</p>
      </div>
    </details>
  );
}

function AssistantPanel({
  sessionId,
  history,
  live,
  turns,
  message,
  setMessage,
  busy,
  controller,
  send,
  newSession,
  refresh,
  changes,
  onChange,
  onOpenInspector,
  modelLabel,
  mode,
  surface,
  period,
  onClose,
}: {
  sessionId: string;
  history: Session;
  live: typeof emptyLive;
  turns: Record<string, typeof emptyLive>;
  message: string;
  setMessage: (value: string) => void;
  busy: boolean;
  controller?: AbortController;
  send: (text: string) => Promise<void>;
  newSession: () => void;
  refresh: () => void;
  changes: Change[];
  onChange: (change: Change, action: ChangeAction) => void;
  onOpenInspector: () => void;
  modelLabel: string;
  mode: RuntimeConfig["mode"];
  surface: AgentSurface;
  period?: Period;
  onClose?: () => void;
}) {
  const end = useRef<HTMLDivElement>(null);
  const followOutput = useRef(true);
  const [waitingSeconds, setWaitingSeconds] = useState(0);
  const composerID = onClose ? "message-mobile" : "message-desktop";
  const composerLabel = onClose
    ? "Your advertising question on mobile"
    : "Your advertising question";
  useEffect(() => {
    if (followOutput.current) end.current?.scrollIntoView({ block: "end" });
  }, [
    history.messages.length,
    live.text,
    live.cards.length,
    live.tools.length,
  ]);
  useEffect(() => {
    if (!busy) {
      setWaitingSeconds(0);
      return;
    }
    const started = Date.now();
    const timer = window.setInterval(
      () => setWaitingSeconds(Math.floor((Date.now() - started) / 1000)),
      1000,
    );
    return () => window.clearInterval(timer);
  }, [busy, live.turnId]);
  const cards = live.cards.map((card) =>
    card.change
      ? {
          ...card,
          change:
            changes.find((change) => change.id === card.change?.id) ??
            card.change,
        }
      : card,
  );
  const suggestions = surface.entityId
    ? [
        {
          label: "Check delivery",
          prompt: `Check the delivery and setup of ${surface.entityLevel} ${surface.entityId}.`,
        },
        {
          label: "Review changes",
          prompt: `Show pending changes for ${surface.entityLevel} ${surface.entityId}. Do not stage anything.`,
        },
      ]
    : mode === "manager"
      ? [
          {
            label: "Compare accounts",
            prompt:
              "Compare account-level performance without combining currencies.",
          },
          {
            label: "Review changes",
            prompt: "Show pending changes grouped by advertiser.",
          },
        ]
      : [
          {
            label: "Compare periods",
            prompt:
              "Compare the selected period with the previous equal period.",
          },
          {
            label: "Review changes",
            prompt:
              "Show pending changes for this advertiser. Do not stage anything.",
          },
        ];
  const liveTextAlreadySaved = history.messages.some(
    (item) => item.role === "assistant" && item.turn_id === live.turnId,
  );
  const visibleLiveText = liveTextAlreadySaved ? "" : live.text;
  return (
    <section className="assistant-panel flex h-full min-h-0 flex-col bg-background">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border px-4">
        <div className="flex size-8 items-center justify-center rounded-lg bg-foreground text-background">
          <Bot className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold">Ad Agent</div>
          <div
            className="truncate text-xs text-muted-foreground"
            title={`Session ${sessionId}`}
          >
            {modelLabel}
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          aria-label="Open activity and memory"
          title="Activity and memory"
          onClick={onOpenInspector}
        >
          <Activity />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          aria-label="New session"
          title="New session"
          disabled={busy}
          onClick={newSession}
        >
          <Plus />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          aria-label="Refresh history"
          title="Refresh history"
          disabled={busy}
          onClick={refresh}
        >
          <RefreshCw />
        </Button>
        {onClose && (
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Close assistant"
            onClick={onClose}
          >
            <X />
          </Button>
        )}
      </header>
      <ScrollArea
        className="min-h-0 flex-1"
        viewportProps={{
          "aria-label": "Conversation",
          onScroll: (event) => {
            const node = event.currentTarget;
            followOutput.current =
              node.scrollHeight - node.scrollTop - node.clientHeight < 80;
          },
        }}
      >
        <ContextIntelligence surface={surface} period={period} />
        <div className="mx-auto flex w-full max-w-xl flex-col gap-4 px-4 py-4">
          {!history.messages.length && !live.text && (
            <div className="py-5">
              <h3 className="text-sm font-medium">How can I help?</h3>
              <div className="mt-3 flex flex-wrap gap-2">
                {suggestions.map(({ label, prompt }) => (
                  <Button
                    key={label}
                    variant="secondary"
                    size="sm"
                    className="h-auto justify-start whitespace-normal py-2 text-left"
                    onClick={() => {
                      followOutput.current = true;
                      void send(prompt);
                    }}
                  >
                    {label}
                  </Button>
                ))}
              </div>
            </div>
          )}
          {history.messages.map((item, index) => (
            <div
              key={`${item.turn_id}-${index}`}
              className={cn(
                "min-w-0 text-sm leading-relaxed",
                item.role === "user" ? "ml-auto max-w-full" : "w-full",
              )}
            >
              {item.role === "assistant" ? (
                <>
                  <div className="agent-cards space-y-3">
                    {(turns[item.turn_id]?.cards ?? [])
                      .filter((card) => card.type !== "suggestions")
                      .map((card) => (
                        <PresentationCard
                          key={card.id}
                          card={
                            card.change
                              ? {
                                  ...card,
                                  change:
                                    changes.find(
                                      (change) => change.id === card.change?.id,
                                    ) ?? card.change,
                                }
                              : card
                          }
                          onSuggest={setMessage}
                          onAction={onChange}
                          busy={busy}
                        />
                      ))}
                  </div>
                  {turns[item.turn_id] && (
                    <TurnActivity
                      turn={turns[item.turn_id]!}
                      names={toolNames}
                    />
                  )}
                  <div className="agent-answer mt-2">
                    <AgentMarkdown text={item.text} />
                  </div>
                  {(turns[item.turn_id]?.cards ?? [])
                    .filter((card) => card.type === "suggestions")
                    .map((card) => (
                      <PresentationCard
                        key={card.id}
                        card={card}
                        onSuggest={setMessage}
                        onAction={onChange}
                        busy={busy}
                      />
                    ))}
                </>
              ) : (
                <>
                  <p className="whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-foreground px-3.5 py-2.5 text-background">
                    {item.text}
                  </p>
                  <TurnContext
                    context={
                      turns[item.turn_id]?.context ??
                      (item.turn_id === live.turnId ||
                      (item.turn_id === "pending" &&
                        index === history.messages.length - 1)
                        ? live.context
                        : undefined)
                    }
                  />
                </>
              )}
              {item.role === "assistant" &&
                item.status !== "completed" &&
                item.status !== "running" && (
                  <Badge tone="warning" className="mt-2">
                    {stateText(item.status)}
                  </Badge>
                )}
            </div>
          ))}
          {!liveTextAlreadySaved &&
            cards.some((card) => card.type !== "suggestions") && (
              <div className="agent-cards space-y-3">
                {cards
                  .filter((card) => card.type !== "suggestions")
                  .map((card) => (
                    <PresentationCard
                      key={card.id}
                      card={card}
                      onSuggest={setMessage}
                      onAction={onChange}
                      busy={busy}
                    />
                  ))}
              </div>
            )}
          {(busy || (!liveTextAlreadySaved && live.turnId)) && (
            <TurnActivity
              turn={live}
              names={toolNames}
              running={busy}
              seconds={waitingSeconds}
            />
          )}
          {visibleLiveText && !busy && (
            <div className="agent-answer w-full min-w-0 text-sm leading-relaxed">
              <AgentMarkdown text={visibleLiveText} />
            </div>
          )}
          <div className="agent-cards space-y-3">
            {!liveTextAlreadySaved &&
              cards
                .filter((card) => card.type === "suggestions")
                .map((card) => (
                  <PresentationCard
                    key={card.id}
                    card={card}
                    onSuggest={(text) => {
                      setMessage(text);
                    }}
                    onAction={onChange}
                    busy={busy}
                  />
                ))}
          </div>
          <div ref={end} />
        </div>
      </ScrollArea>
      <div className="shrink-0 border-t border-border bg-background p-3">
        <form
          className="agent-composer rounded-xl border border-border bg-background shadow-sm transition-colors focus-within:border-ring"
          onSubmit={(event) => {
            event.preventDefault();
            followOutput.current = true;
            void send(message);
          }}
        >
          <label htmlFor={composerID} className="sr-only">
            {composerLabel}
          </label>
          <Textarea
            id={composerID}
            placeholder="Ask about performance or prepare a change…"
            value={message}
            onChange={(event) => setMessage(event.target.value)}
            rows={2}
            maxLength={8000}
            onKeyDown={(event) => {
              if (
                event.key === "Enter" &&
                !event.shiftKey &&
                !event.nativeEvent.isComposing
              ) {
                event.preventDefault();
                followOutput.current = true;
                void send(message);
              }
            }}
          />
          <div className="flex items-center justify-between px-2 pb-2">
            <span className="text-xs text-muted-foreground">
              Changes require approval
            </span>
            {busy ? (
              <Button
                type="button"
                variant="secondary"
                size="icon"
                className="size-7"
                onClick={() => controller?.abort()}
                aria-label="Cancel turn"
              >
                <Square className="size-3 fill-current" />
              </Button>
            ) : (
              <Button
                size="icon"
                className="size-7"
                disabled={!message.trim()}
                aria-label="Send"
              >
                <Send className="size-3.5" />
              </Button>
            )}
          </div>
        </form>
      </div>
    </section>
  );
}

function HomeView({
  account,
  overview,
  changes,
  onAsk,
  onNavigate,
}: {
  account?: Account;
  overview?: OverviewData;
  changes: Change[];
  onAsk: (text: string) => void;
  onNavigate: (page: Page) => void;
}) {
  const staged = changes.filter((change) => change.state === "staged");
  const [campaigns, setCampaigns] = useState<Entity[]>([]);
  useEffect(() => {
    void api<Entity[]>("/entities/campaign")
      .then(setCampaigns)
      .catch(() => {});
  }, []);
  const names = new Map(
    campaigns.map((campaign) => [campaign.id, campaign.name]),
  );
  const movements = (overview?.current.calculation?.ranking ?? [])
    .map((item) => {
      const previous = overview?.previous.calculation?.ranking.find(
        (candidate) => candidate.entity_id === item.entity_id,
      );
      return {
        id: item.entity_id,
        name: names.get(item.entity_id) ?? item.entity_id,
        current: item,
        delta: deltaPercent(
          metricNumber(item.roas),
          metricNumber(previous?.roas),
        ),
      };
    })
    .sort((a, b) => (a.delta ?? 0) - (b.delta ?? 0));
  const largestDecline = movements.find(
    (item) => item.delta != null && item.delta < 0,
  );
  const hasComparison = movements.some((item) => item.delta != null);
  const weakest = movements
    .filter((item) => metricNumber(item.current.roas) != null)
    .sort(
      (a, b) =>
        Number(a.current.roas ?? Infinity) - Number(b.current.roas ?? Infinity),
    )[0];
  const dataNotes = [
    ...new Set([
      ...(account?.limitations ?? []),
      ...(overview?.current.report.limitations ?? []),
    ]),
  ];
  return (
    <div className="space-y-8">
      <PageHeading
        title="Today"
        action={
          <Button
            variant="default"
            onClick={() =>
              onAsk(
                "Give me today's prioritized account briefing for the period on screen. Lead with findings and show only decision-relevant evidence.",
              )
            }
          >
            <Bot />
            Analyze today
          </Button>
        }
      />
      <Card>
        <CardHeader className="flex-row items-end justify-between">
          <div>
            <CardTitle>Account health</CardTitle>
            <CardDescription className="mt-1">
              {!overview
                ? "Awaiting report"
                : overview.current.report.complete
                  ? "Complete report"
                  : "Partial report · reported to date"}
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          {overview ? (
            <MetricStrip
              metrics={overview.current.report.totals}
              roas={overview.current.calculation?.roas}
              currency={overview.current.report.currency}
            />
          ) : (
            <div className="h-20 animate-pulse rounded-lg bg-muted" />
          )}
          {dataNotes.length > 0 && (
            <details className="mt-4 border-t border-border pt-3 text-xs text-muted-foreground">
              <summary className="w-fit cursor-pointer">Data notes</summary>
              <ul className="mt-2 list-disc space-y-1 pl-4">
                {dataNotes.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </details>
          )}
        </CardContent>
      </Card>
      <div
        className={cn(
          "grid gap-6",
          changes.length > 0 && "lg:grid-cols-[1.25fr_.75fr]",
        )}
      >
        <Card>
          <CardHeader>
            <CardTitle>What needs attention</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 p-2 pt-3">
            <DecisionRow
              icon={
                largestDecline
                  ? TrendingDown
                  : hasComparison
                    ? CheckCircle2
                    : CircleAlert
              }
              title={
                largestDecline
                  ? largestDecline.name
                  : hasComparison
                    ? "No campaign decline established"
                    : "Campaign comparison unavailable"
              }
              description={
                largestDecline
                  ? `ROAS ${signedPercent(largestDecline.delta)} versus the previous equal period.`
                  : hasComparison
                    ? "Available campaign comparisons show no negative ROAS movement. Check report coverage before acting."
                    : "Two periods with valid purchase value and spend are needed to assess ROAS movement."
              }
              action={
                largestDecline ? "Diagnose movement" : "Review performance"
              }
              onClick={() =>
                onAsk(
                  largestDecline
                    ? `Diagnose campaign ${largestDecline.id} for the selected period. Identify observed funnel drivers and counter-evidence before recommending a change.`
                    : "Review campaign performance for the selected period and identify any decision-relevant movement.",
                )
              }
            />
            {weakest && weakest.id !== largestDecline?.id && (
              <DecisionRow
                icon={CircleAlert}
                title={weakest.name}
                description={`Lowest campaign ROAS: ${format(weakest.current.roas, 2)}×. Cause not established.`}
                action="Inspect campaign"
                onClick={() =>
                  onAsk(
                    `Inspect campaign ${weakest.id}. Explain its spend, results, CPA, ROAS, and what evidence is missing before action.`,
                  )
                }
              />
            )}
            {staged.length > 0 && (
              <DecisionRow
                icon={ShieldCheck}
                title={`${staged.length} change${staged.length === 1 ? "" : "s"} awaiting review`}
                description="Drafts are not applied until you approve."
                action="Review changes"
                onClick={() => onNavigate("changes")}
              />
            )}
          </CardContent>
        </Card>
        {changes.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Staged and recent</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {changes.length ? (
                changes.slice(0, 4).map((change) => (
                  <button
                    key={change.id}
                    onClick={() => onNavigate("changes")}
                    className="flex w-full items-start justify-between gap-3 border-b border-border pb-3 text-left last:border-0 last:pb-0"
                  >
                    <span className="min-w-0">
                      <strong className="block truncate text-sm font-medium">
                        {changeTarget(change)}
                      </strong>
                      <span className="text-xs text-muted-foreground">
                        {changeKindLabel(change)}
                      </span>
                    </span>
                    <Badge
                      tone={change.state === "applied" ? "success" : "muted"}
                    >
                      {stateText(change.state)}
                    </Badge>
                  </button>
                ))
              ) : (
                <div className="py-6 text-center text-xs text-muted-foreground">
                  No change activity in this session.
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}

function AccountsView({
  manager,
  report,
  changes,
  onAsk,
  onNavigate,
  onSelectAccount,
}: {
  manager?: ManagerScope;
  report?: ManagerReport;
  changes: Change[];
  onAsk: (text: string) => void;
  onNavigate: (page: Page) => void;
  onSelectAccount: (account: Account) => void;
}) {
  const staged = changes.filter((change) => change.state === "staged");
  return (
    <div className="space-y-8">
      <PageHeading
        title="Accounts"
        description={`${manager?.accounts.length ?? 0} independently bound advertisers`}
        action={
          <Button
            variant="default"
            onClick={() =>
              onAsk(
                "Triage the authorized advertiser accounts. Rank only accounts with comparable evidence, preserve currency and attribution boundaries, and propose the next drill-down.",
              )
            }
          >
            <Bot />
            Triage accounts
          </Button>
        }
      />
      <Card className="overflow-hidden">
        <CardHeader className="flex-row items-end justify-between">
          <div>
            <CardTitle>Client account performance</CardTitle>
            <CardDescription className="mt-1">
              Separate account totals · no cross-currency total
            </CardDescription>
          </div>
          {staged.length > 0 && (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onNavigate("changes")}
            >
              {staged.length} awaiting review
            </Button>
          )}
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-y border-border bg-muted/35 text-xs text-muted-foreground">
                <tr>
                  <th className="px-5 py-3 font-medium">Advertiser</th>
                  <th className="px-4 py-3 font-medium">Spend</th>
                  <th className="px-4 py-3 font-medium">ROAS</th>
                  <th className="px-4 py-3 font-medium">Coverage</th>
                  <th className="px-5 py-3 text-right font-medium">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {(report?.accounts ?? []).map((item) => (
                  <tr
                    key={item.account.id}
                    className="cursor-pointer hover:bg-muted/30"
                    onClick={() => onSelectAccount(item.account)}
                  >
                    <td className="px-5 py-4">
                      <strong className="block font-medium">
                        {item.account.name}
                      </strong>
                      <span className="text-xs text-muted-foreground">
                        {item.account.id} · {item.account.timezone}
                      </span>
                    </td>
                    <td className="px-4 py-4 tabular-nums">
                      {format(item.metrics.spend)} {item.account.currency}
                    </td>
                    <td className="px-4 py-4 tabular-nums">
                      {item.roas == null ? "—" : `${format(item.roas)}×`}
                    </td>
                    <td className="px-4 py-4">
                      <Badge tone={item.complete ? "success" : "warning"}>
                        {item.complete ? "Complete" : "Limited"}
                      </Badge>
                    </td>
                    <td className="px-5 py-4 text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={(event) => {
                          event.stopPropagation();
                          onSelectAccount(item.account);
                        }}
                      >
                        Open account
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!report && (
              <div className="m-5 h-24 animate-pulse rounded-lg bg-muted" />
            )}
          </div>
        </CardContent>
      </Card>
      {report?.limitations.map((limitation) => (
        <p key={limitation} className="text-xs text-muted-foreground">
          {limitation}
        </p>
      ))}
    </div>
  );
}

function DecisionRow({
  icon: Icon,
  title,
  description,
  action,
  onClick,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
  action: string;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="group flex w-full gap-3 rounded-lg px-3 py-3 text-left hover:bg-muted"
    >
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-background">
        <Icon className="size-4" />
      </div>
      <span className="min-w-0 flex-1">
        <strong className="block text-sm font-medium">{title}</strong>
        <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">
          {description}
        </span>
        <span className="mt-1.5 inline-flex items-center gap-1 text-xs font-medium">
          {action}
          <ChevronRight className="size-3 transition-transform group-hover:translate-x-0.5" />
        </span>
      </span>
    </button>
  );
}

function PageHeading({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
      <div>
        <h1 className="ui-page-title">{title}</h1>
        {description && (
          <p className="mt-1 text-xs text-muted-foreground">{description}</p>
        )}
      </div>
      {action}
    </div>
  );
}

function HierarchyColumn({
  label,
  entities,
  selected,
  empty,
  onSelect,
}: {
  label: string;
  entities: Entity[];
  selected?: string;
  empty: string;
  onSelect: (entity: Entity) => void;
}) {
  return (
    <section
      aria-label={`${label} hierarchy`}
      className="min-w-0 border-b border-border last:border-b-0 lg:border-b-0 lg:border-r lg:last:border-r-0"
    >
      <div className="flex h-11 items-center justify-between border-b border-border bg-muted/35 px-3">
        <h3 className="text-xs font-semibold uppercase tracking-[.1em] text-muted-foreground">
          {label}
        </h3>
        <Badge tone="muted">{entities.length}</Badge>
      </div>
      <div className="max-h-80 overflow-y-auto p-2">
        {entities.length ? (
          entities.map((entity) => (
            <button
              key={entity.id}
              className={cn(
                "mb-1 flex w-full items-start gap-2 rounded-lg border border-transparent px-2.5 py-2.5 text-left last:mb-0 hover:bg-muted/60",
                selected === entity.id &&
                  "border-border bg-background shadow-sm",
              )}
              onClick={() => onSelect(entity)}
            >
              <span
                className={cn(
                  "mt-1 size-1.5 shrink-0 rounded-full",
                  entity.status === "ENABLE" ? "bg-emerald-500" : "bg-zinc-300",
                )}
              />
              <span className="min-w-0 flex-1">
                <strong className="block truncate text-xs font-medium">
                  {entity.name}
                </strong>
                <span className="mt-0.5 block truncate font-mono text-xs text-muted-foreground">
                  {entity.id}
                </span>
              </span>
              <ChevronRight className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
            </button>
          ))
        ) : (
          <p className="px-3 py-10 text-center text-xs leading-relaxed text-muted-foreground">
            {empty}
          </p>
        )}
      </div>
    </section>
  );
}

function TrendMark({ value }: { value: number | null }) {
  if (value == null)
    return <span className="text-muted-foreground">Unavailable</span>;
  const down = value < -0.5;
  const up = value > 0.5;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 tabular-nums",
        down
          ? "text-red-700"
          : up
            ? "text-emerald-700"
            : "text-muted-foreground",
      )}
    >
      {down ? (
        <TrendingDown className="size-3.5" />
      ) : up ? (
        <TrendingUp className="size-3.5" />
      ) : null}
      {signedPercent(value)}
    </span>
  );
}

function PerformanceTable({
  level,
  entities,
  current,
  previous,
  period,
  onSelect,
}: {
  level: "campaign" | "ad_group" | "ad";
  entities: Entity[];
  current?: Report;
  previous?: Report;
  period: Period;
  onSelect?: (entity: Entity) => void;
}) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[900px] text-left text-sm">
        <thead className="border-b border-border bg-muted/35 text-xs text-muted-foreground">
          <tr>
            <th className="px-4 py-3 font-medium">
              {level === "campaign"
                ? "Campaign"
                : level === "ad_group"
                  ? "Ad group"
                  : "Ad"}
            </th>
            <th className="px-3 py-3 font-medium">Status</th>
            <th className="px-3 py-3 font-medium">Spend</th>
            <th className="px-3 py-3 font-medium">Results</th>
            <th className="px-3 py-3 font-medium">CPA</th>
            <th className="px-3 py-3 font-medium">ROAS</th>
            {level !== "ad" && (
              <th className="px-3 py-3 font-medium">Budget</th>
            )}
            {level === "campaign" && (
              <th className="px-3 py-3 font-medium">Pacing</th>
            )}
            <th className="px-3 py-3 font-medium">Trend</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {entities.map((entity) => {
            const metrics = entityMetrics(current, entity.id);
            const prior = entityMetrics(previous, entity.id);
            const currentROAS = roas(metrics);
            const movement = deltaPercent(currentROAS, roas(prior));
            const dailyBudget =
              entity.budget_mode === "BUDGET_MODE_DAY"
                ? Number(entity.budget)
                : null;
            const pacing =
              dailyBudget && metrics
                ? (Number(metrics.spend) / (dailyBudget * period.days)) * 100
                : null;
            return (
              <tr
                key={entity.id}
                className={cn(onSelect && "cursor-pointer hover:bg-muted/30")}
                onClick={() => onSelect?.(entity)}
              >
                <td className="max-w-[280px] px-4 py-3">
                  <strong className="block truncate font-medium">
                    {entity.name}
                  </strong>
                  <span className="block truncate font-mono text-xs text-muted-foreground">
                    {entity.id}
                  </span>
                </td>
                <td className="px-3 py-3">
                  <Badge
                    tone={entity.status === "ENABLE" ? "success" : "muted"}
                  >
                    {entity.status === "ENABLE" ? "Active" : "Disabled"}
                  </Badge>
                </td>
                <td className="px-3 py-3 tabular-nums">
                  {format(metrics?.spend)}
                </td>
                <td className="px-3 py-3 tabular-nums">
                  {format(metrics?.conversions, 1)}
                </td>
                <td className="px-3 py-3 tabular-nums">
                  {cpa(metrics) == null ? "—" : format(cpa(metrics), 2)}
                </td>
                <td className="px-3 py-3 font-medium tabular-nums">
                  {currentROAS == null ? "—" : `${format(currentROAS, 2)}×`}
                </td>
                {level !== "ad" && (
                  <td className="px-3 py-3 tabular-nums">
                    {entity.budget
                      ? `${format(entity.budget)}${entity.budget_mode === "BUDGET_MODE_DAY" ? "/d" : " total"}`
                      : "—"}
                  </td>
                )}
                {level === "campaign" && (
                  <td className="px-3 py-3 tabular-nums">
                    {pacing == null ? "—" : `${format(pacing, 0)}%`}
                  </td>
                )}
                <td className="px-3 py-3">
                  <TrendMark value={movement} />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {!entities.length && (
        <div className="px-5 py-12 text-center text-sm text-muted-foreground">
          No objects returned for this scope.
        </div>
      )}
    </div>
  );
}

function AnalysisMenu({
  entity,
  onAsk,
}: {
  entity?: Entity;
  onAsk: (text: string) => void;
}) {
  const target = entity
    ? `campaign ${entity.id}`
    : "campaigns in the selected period";
  const actions = [
    {
      label: "Analyze performance",
      prompt: `Analyze ${target}. Compare the selected period with the previous equal period and identify the largest measured movement.`,
    },
    {
      label: "Check delivery",
      prompt: `Check delivery for ${target}. Separate status, spend, pacing, and data limitations from causal hypotheses.`,
    },
    {
      label: "Review budget",
      prompt: `Review budget evidence for ${target}. Identify constraints or opportunities, but do not stage a change.`,
    },
    {
      label: "Audit structure",
      prompt: `Audit the structure of ${target}, including parent relationships and overlapping budget controls. Do not treat structure as a performance cause without evidence.`,
    },
    {
      label: "Check tracking",
      prompt: `Check measurement and tracking readiness for ${target}. Report verified settings and missing evidence.`,
    },
    {
      label: "Analyze creative",
      prompt: `Analyze ad and creative contribution for ${target}. Do not claim fatigue without frequency and asset-age evidence.`,
    },
  ];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button>
          <Sparkles />
          {entity ? "Diagnose" : "Analyze"}
          <ChevronDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Analysis lens</DropdownMenuLabel>
        {actions.map((action, index) => (
          <React.Fragment key={action.label}>
            {index === 3 && <DropdownMenuSeparator />}
            <DropdownMenuItem onSelect={() => onAsk(action.prompt)}>
              {action.label}
            </DropdownMenuItem>
          </React.Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function CampaignWorkspace({
  account,
  period,
  revision,
  changes,
  basePath,
  onAsk,
  onSurface,
  onBack,
}: {
  account?: Account;
  period: Period;
  revision: number;
  changes: Change[];
  basePath: string;
  onAsk: (text: string) => void;
  onSurface: (surface: AgentSurface) => void;
  onBack?: () => void;
}) {
  const [view, setView] = useState<"performance" | "structure" | "changes">(
    "performance",
  );
  const [filter, setFilter] = useState<
    "all" | "attention" | "active" | "disabled"
  >("all");
  const [detailTab, setDetailTab] = useState<
    "overview" | "adgroups" | "ads" | "history"
  >("overview");
  const [campaigns, setCampaigns] = useState<Entity[]>([]);
  const [adGroups, setAdGroups] = useState<Entity[]>([]);
  const [ads, setAds] = useState<Entity[]>([]);
  const [selectedCampaign, setSelectedCampaign] = useState<Entity>();
  const [selectedAdGroup, setSelectedAdGroup] = useState<Entity>();
  const [selectedAd, setSelectedAd] = useState<Entity>();
  const [detail, setDetail] = useState<AdDetail>();
  const [reports, setReports] = useState<{
    campaign?: ReportBundle;
    campaignPrevious?: ReportBundle;
    groups?: ReportBundle;
    groupsPrevious?: ReportBundle;
    ads?: ReportBundle;
    adsPrevious?: ReportBundle;
  }>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [reload, setReload] = useState(0);

  const prior = { start: period.compareStart, end: period.compareEnd };
  const chooseAd = async (ad: Entity) => {
    setSelectedAd(ad);
    setDetail(undefined);
    try {
      setDetail(
        await api<AdDetail>(
          `${basePath}/ads/${encodeURIComponent(ad.id)}/detail`,
        ),
      );
    } catch {
      setDetail(undefined);
    }
  };
  const chooseAdGroup = async (group: Entity) => {
    setSelectedAdGroup(group);
    setSelectedAd(undefined);
    setDetail(undefined);
    const values = await api<Entity[]>(
      `${basePath}/entities/ad?parent_id=${encodeURIComponent(group.id)}`,
    );
    setAds(values);
    if (values[0]) await chooseAd(values[0]);
  };
  const chooseCampaign = async (campaign: Entity) => {
    setSelectedCampaign(campaign);
    setSelectedAdGroup(undefined);
    setSelectedAd(undefined);
    setAds([]);
    setDetail(undefined);
    const values = await api<Entity[]>(
      `${basePath}/entities/ad_group?parent_id=${encodeURIComponent(campaign.id)}`,
    );
    setAdGroups(values);
    if (values[0]) await chooseAdGroup(values[0]);
  };

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(false);
    setReports({});
    void Promise.all([
      api<Entity[]>(`${basePath}/entities/campaign`),
      api<ReportBundle>(reportPath(basePath, "campaign", period)),
      api<ReportBundle>(reportPath(basePath, "campaign", prior)),
      api<ReportBundle>(reportPath(basePath, "ad_group", period)),
      api<ReportBundle>(reportPath(basePath, "ad_group", prior)),
      api<ReportBundle>(reportPath(basePath, "ad", period)),
      api<ReportBundle>(reportPath(basePath, "ad", prior)),
    ])
      .then(
        ([
          entities,
          campaign,
          campaignPrevious,
          groups,
          groupsPrevious,
          adReport,
          adsPrevious,
        ]) => {
          if (!active) return;
          setCampaigns(entities);
          setReports({
            campaign,
            campaignPrevious,
            groups,
            groupsPrevious,
            ads: adReport,
            adsPrevious,
          });
        },
      )
      .catch(() => active && setLoadError(true))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [basePath, period.start, period.end, revision, reload]);

  useEffect(() => {
    if (!selectedCampaign) {
      const ranked = campaigns
        .map((campaign) => ({
          campaign,
          movement: deltaPercent(
            roas(entityMetrics(reports.campaign?.report, campaign.id)),
            roas(entityMetrics(reports.campaignPrevious?.report, campaign.id)),
          ),
        }))
        .sort((a, b) => (a.movement ?? 0) - (b.movement ?? 0));
      const lead = ranked[0];
      onSurface({
        page: "campaigns",
        title: "Campaign performance",
        subtitle: `${account?.name ?? "Advertiser"} · ${campaigns.length} campaigns`,
        accountId: account?.id,
        finding:
          lead?.movement != null
            ? {
                label: "Largest movement",
                value: `${lead.campaign.name} ${signedPercent(lead.movement)}`,
                detail:
                  "ROAS change versus the previous equal period. Select the campaign to inspect observed funnel drivers.",
                tone: lead.movement < -10 ? "warning" : "muted",
              }
            : undefined,
        recommendation:
          "Start from performance, then use Structure only when hierarchy evidence is relevant.",
        actionLabel: "Analyze performance",
        actionPrompt:
          "Analyze campaign performance for the selected period. Prioritize the largest evidence-backed movement and identify the next drill-down.",
      });
      return;
    }
    const current = entityMetrics(
      reports.campaign?.report,
      selectedCampaign.id,
    );
    const previous = entityMetrics(
      reports.campaignPrevious?.report,
      selectedCampaign.id,
    );
    const movement = deltaPercent(roas(current), roas(previous));
    const drivers = [
      { label: "CVR", magnitude: deltaPercent(cvr(current), cvr(previous)) },
      { label: "CTR", magnitude: deltaPercent(ctr(current), ctr(previous)) },
      { label: "CPM", magnitude: deltaPercent(cpm(current), cpm(previous)) },
    ]
      .filter(
        (item): item is { label: string; magnitude: number } =>
          item.magnitude != null,
      )
      .sort((a, b) => Math.abs(b.magnitude) - Math.abs(a.magnitude))
      .map((item) => ({ ...item, value: signedPercent(item.magnitude) }));
    onSurface({
      page: "campaigns",
      title: selectedCampaign.name,
      subtitle: `Campaign · TikTok · ${selectedCampaign.objective ?? "Objective unavailable"}`,
      accountId: account?.id,
      entityLevel: "campaign",
      entityId: selectedCampaign.id,
      finding:
        movement == null
          ? undefined
          : {
              label:
                movement < -10 ? "Performance alert" : "Performance finding",
              value: `ROAS ${signedPercent(movement)}`,
              detail: `${format(roas(current), 2)}× versus ${format(roas(previous), 2)}× in the previous equal period.`,
              tone:
                movement < -10
                  ? "warning"
                  : movement > 10
                    ? "success"
                    : "muted",
            },
      drivers,
      confidence: {
        label:
          reports.campaign?.report.complete &&
          reports.campaignPrevious?.report.complete
            ? "Moderate"
            : "Limited",
        detail:
          "The movement is measured across equal periods; contributor ranking is not causal proof.",
      },
      recommendation:
        movement != null && movement < -10
          ? "Diagnose ad-level contribution before changing campaign budget or delivery."
          : "No urgent campaign action is established; inspect contributors before changing delivery.",
      actionLabel: "Diagnose",
      actionPrompt: `Diagnose campaign ${selectedCampaign.id} for the selected and previous periods. Identify measured drivers, counter-evidence, and the smallest safe next action. Do not stage a change yet.`,
    });
  }, [account?.id, campaigns, reports, selectedCampaign, onSurface]);

  const visibleCampaigns = campaigns.filter((campaign) => {
    if (filter === "active") return campaign.status === "ENABLE";
    if (filter === "disabled") return campaign.status !== "ENABLE";
    if (filter === "attention") {
      const movement = deltaPercent(
        roas(entityMetrics(reports.campaign?.report, campaign.id)),
        roas(entityMetrics(reports.campaignPrevious?.report, campaign.id)),
      );
      return movement != null && movement < -10;
    }
    return true;
  });
  const campaignGroups = adGroups.filter(
    (group) => group.parent_id === selectedCampaign?.id,
  );
  const campaignGroupIDs = new Set(campaignGroups.map((group) => group.id));
  const campaignAds = ads.filter((ad) =>
    campaignGroupIDs.has(ad.parent_id ?? ""),
  );
  const currentMetrics = selectedCampaign
    ? entityMetrics(reports.campaign?.report, selectedCampaign.id)
    : undefined;
  const previousMetrics = selectedCampaign
    ? entityMetrics(reports.campaignPrevious?.report, selectedCampaign.id)
    : undefined;
  const movement = deltaPercent(roas(currentMetrics), roas(previousMetrics));
  const campaignChanges = changes.filter(
    (change) =>
      change.source.account_id === account?.id &&
      (change.before?.id === selectedCampaign?.id ||
        change.after?.id === selectedCampaign?.id ||
        campaignGroupIDs.has(change.before?.parent_id ?? "") ||
        campaignGroupIDs.has(change.after?.parent_id ?? "")),
  );

  if (loading || loadError)
    return (
      <ReportLoadState
        title={selectedCampaign?.name ?? "Campaigns"}
        loading={loading}
        onRetry={() => setReload((value) => value + 1)}
      />
    );

  return (
    <div className="space-y-6">
      {selectedCampaign ? (
        <>
          <button
            className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
            onClick={() => {
              setSelectedCampaign(undefined);
              setSelectedAdGroup(undefined);
              setSelectedAd(undefined);
              setAdGroups([]);
              setAds([]);
            }}
          >
            <ArrowLeft className="size-3.5" /> Campaigns
          </button>
          <PageHeading
            title={selectedCampaign.name}
            description={selectedCampaign.id}
            action={
              <div className="flex items-center gap-2">
                <Badge
                  tone={
                    selectedCampaign.status === "ENABLE" ? "success" : "muted"
                  }
                >
                  {selectedCampaign.status === "ENABLE" ? "Active" : "Disabled"}
                </Badge>
                <AnalysisMenu entity={selectedCampaign} onAsk={onAsk} />
              </div>
            }
          />
          <div
            className="flex gap-1 border-b border-border"
            role="tablist"
            aria-label="Campaign detail views"
          >
            {(["overview", "adgroups", "ads", "history"] as const).map(
              (tab) => (
                <Button
                  key={tab}
                  role="tab"
                  aria-selected={detailTab === tab}
                  variant="ghost"
                  size="sm"
                  className={cn(
                    "rounded-b-none capitalize",
                    detailTab === tab && "border-b-2 border-foreground",
                  )}
                  onClick={() => setDetailTab(tab)}
                >
                  {tab === "adgroups" ? "Ad Groups" : titleCase(tab)}
                </Button>
              ),
            )}
          </div>
          {detailTab === "overview" && (
            <div className="space-y-6">
              <Card>
                <CardContent className="pt-5">
                  <MetricStrip
                    metrics={
                      currentMetrics ?? {
                        spend: "0",
                        impressions: 0,
                        clicks: 0,
                        conversions: null,
                        revenue: null,
                      }
                    }
                    roas={roas(currentMetrics)?.toString()}
                    currency={account?.currency}
                  />
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>Performance</CardTitle>
                  <CardDescription>Daily ROAS</CardDescription>
                </CardHeader>
                <CardContent>
                  <RoasTrendChart
                    rows={(reports.campaign?.report.rows ?? []).filter(
                      (row) => row.entity_id === selectedCampaign.id,
                    )}
                    currency={account?.currency ?? "USD"}
                  />
                  <div className="mt-3 border-t border-border pt-4">
                    <div className="text-xs font-medium text-muted-foreground">
                      Primary movement
                    </div>
                    <p className="mt-1 text-sm">
                      {movement == null ? (
                        <span className="text-muted-foreground">
                          Not enough comparable data.
                        </span>
                      ) : (
                        <>
                          ROAS changed <TrendMark value={movement} /> versus the
                          previous equal period.
                        </>
                      )}
                    </p>
                  </div>
                </CardContent>
              </Card>
            </div>
          )}
          {detailTab === "adgroups" && (
            <Card className="overflow-hidden">
              <PerformanceTable
                level="ad_group"
                entities={campaignGroups}
                current={reports.groups?.report}
                previous={reports.groupsPrevious?.report}
                period={period}
                onSelect={(group) => {
                  setSelectedAdGroup(group);
                  setDetailTab("ads");
                  void chooseAdGroup(group);
                }}
              />
            </Card>
          )}
          {detailTab === "ads" && (
            <Card className="overflow-hidden">
              <PerformanceTable
                level="ad"
                entities={selectedAdGroup ? ads : campaignAds}
                current={reports.ads?.report}
                previous={reports.adsPrevious?.report}
                period={period}
                onSelect={(ad) => void chooseAd(ad)}
              />
              {detail && (
                <div className="border-t border-border p-5">
                  <div className="grid gap-5 md:grid-cols-[160px_1fr]">
                    <CreativePreview detail={detail} />
                    <div>
                      <div className="text-sm font-semibold">
                        {detail.creative?.name ?? detail.ad.name}
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        {detail.identity?.name ?? "Identity unavailable"} ·{" "}
                        {detail.creative?.review_status ?? "Review unavailable"}
                      </div>
                      <p className="mt-4 text-sm">
                        {detail.primary_text ?? "Primary text unavailable"}
                      </p>
                      <div className="mt-3 flex gap-2">
                        <Badge tone="muted">
                          {detail.format ?? "Format unavailable"}
                        </Badge>
                        <Badge tone="muted">
                          {detail.call_to_action ?? "CTA unavailable"}
                        </Badge>
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </Card>
          )}
          {detailTab === "history" && (
            <Card className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b border-border bg-muted/35 text-left text-xs text-muted-foreground">
                    <tr>
                      <th className="px-4 py-3">Date</th>
                      <th className="px-4 py-3">Spend</th>
                      <th className="px-4 py-3">Results</th>
                      <th className="px-4 py-3">ROAS</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {(reports.campaign?.report.rows ?? [])
                      .filter((row) => row.entity_id === selectedCampaign.id)
                      .map((row) => (
                        <tr key={row.date}>
                          <td className="px-4 py-3">{row.date}</td>
                          <td className="px-4 py-3 tabular-nums">
                            {format(row.metrics.spend)}
                          </td>
                          <td className="px-4 py-3 tabular-nums">
                            {format(row.metrics.conversions, 1)}
                          </td>
                          <td className="px-4 py-3 tabular-nums">
                            {roas(row.metrics) == null
                              ? "—"
                              : `${format(roas(row.metrics), 2)}×`}
                          </td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            </Card>
          )}
        </>
      ) : (
        <>
          <PageHeading
            title="Campaigns"
            action={<AnalysisMenu onAsk={onAsk} />}
          />
          <div
            className="flex gap-1 border-b border-border"
            role="tablist"
            aria-label="Campaign workspace views"
          >
            {(["performance", "structure", "changes"] as const).map((tab) => (
              <Button
                key={tab}
                role="tab"
                aria-selected={view === tab}
                variant="ghost"
                size="sm"
                className={cn(
                  "rounded-b-none capitalize",
                  view === tab && "border-b-2 border-foreground",
                )}
                onClick={() => setView(tab)}
              >
                {titleCase(tab)}
              </Button>
            ))}
          </div>
          {view === "performance" && (
            <Card className="overflow-hidden">
              <div className="flex flex-wrap gap-1 border-b border-border p-2">
                {(["all", "attention", "active", "disabled"] as const).map(
                  (item) => (
                    <Button
                      key={item}
                      variant={filter === item ? "secondary" : "ghost"}
                      size="sm"
                      className="capitalize"
                      onClick={() => setFilter(item)}
                    >
                      {item === "attention"
                        ? "Needs attention"
                        : titleCase(item)}
                    </Button>
                  ),
                )}
              </div>
              <PerformanceTable
                level="campaign"
                entities={visibleCampaigns}
                current={reports.campaign?.report}
                previous={reports.campaignPrevious?.report}
                period={period}
                onSelect={(campaign) => void chooseCampaign(campaign)}
              />
              {loading && (
                <div className="m-4 h-16 animate-pulse rounded-lg bg-muted" />
              )}
            </Card>
          )}
          {view === "structure" && (
            <Card className="overflow-hidden">
              <div className="grid lg:grid-cols-3">
                <HierarchyColumn
                  label="Campaigns"
                  entities={campaigns}
                  empty={
                    loading ? "Loading campaigns…" : "No campaigns returned."
                  }
                  onSelect={(entity) => void chooseCampaign(entity)}
                />
                <HierarchyColumn
                  label="Ad groups"
                  entities={adGroups}
                  empty="Select a campaign to inspect its ad groups."
                  onSelect={(entity) => void chooseAdGroup(entity)}
                />
                <HierarchyColumn
                  label="Ads"
                  entities={ads}
                  empty="Select an ad group to inspect its ads."
                  onSelect={(entity) => void chooseAd(entity)}
                />
              </div>
            </Card>
          )}
          {view === "changes" && (
            <Card>
              <CardHeader>
                <CardTitle>Campaign change activity</CardTitle>
                <CardDescription>
                  Drafts and verified outcomes for this advertiser.
                </CardDescription>
              </CardHeader>
              <CardContent>
                {changes.length ? (
                  <div className="space-y-3">
                    {changes.slice(0, 6).map((change) => (
                      <div
                        key={change.id}
                        className="flex items-center justify-between border-b border-border pb-3 last:border-0"
                      >
                        <div>
                          <div className="text-sm font-medium">
                            {changeTarget(change)}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {change.kind} · {change.reason}
                          </div>
                        </div>
                        <Badge
                          tone={
                            change.state === "applied" ? "success" : "muted"
                          }
                        >
                          {stateText(change.state)}
                        </Badge>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="py-8 text-center text-sm text-muted-foreground">
                    No change activity in this session.
                  </p>
                )}
              </CardContent>
            </Card>
          )}
        </>
      )}
      {onBack && (
        <Button variant="ghost" size="sm" onClick={onBack}>
          <ArrowLeft />
          Back to accounts
        </Button>
      )}
    </div>
  );
}

function CreativesWorkspace({
  account,
  period,
  revision,
  basePath,
  onAsk,
  onSurface,
}: {
  account?: Account;
  period: Period;
  revision: number;
  basePath: string;
  onAsk: (text: string) => void;
  onSurface: (surface: AgentSurface) => void;
}) {
  const [filter, setFilter] = useState<
    "all" | "winners" | "attention" | "undertested"
  >("all");
  const [ads, setAds] = useState<Entity[]>([]);
  const [details, setDetails] = useState<AdDetail[]>([]);
  const [current, setCurrent] = useState<ReportBundle>();
  const [previous, setPrevious] = useState<ReportBundle>();
  const [selected, setSelected] = useState<AdDetail>();
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [reload, setReload] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(false);
    const prior = { start: period.compareStart, end: period.compareEnd };
    void Promise.all([
      api<Entity[]>(`${basePath}/entities/ad`),
      api<ReportBundle>(reportPath(basePath, "ad", period)),
      api<ReportBundle>(reportPath(basePath, "ad", prior)),
    ])
      .then(async ([entities, report, priorReport]) => {
        const records = await Promise.all(
          entities.map((ad) =>
            api<AdDetail>(
              `${basePath}/ads/${encodeURIComponent(ad.id)}/detail`,
            ).catch(() => ({ ad }) as AdDetail),
          ),
        );
        if (!active) return;
        setAds(entities);
        setDetails(records);
        setCurrent(report);
        setPrevious(priorReport);
      })
      .catch(() => active && setLoadError(true))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [basePath, period.start, period.end, revision, reload]);
  const accountROAS = roas(current?.report.totals);
  const averageImpressions = ads.length
    ? current!.report.totals.impressions / ads.length
    : 0;
  const records = details.map((detail) => {
    const metrics = entityMetrics(current?.report, detail.ad.id);
    const priorMetrics = entityMetrics(previous?.report, detail.ad.id);
    const value = roas(metrics);
    const movement = deltaPercent(value, roas(priorMetrics));
    const signal =
      metrics && metrics.impressions < averageImpressions * 0.5
        ? "Under-tested"
        : accountROAS != null && value != null && value >= accountROAS * 1.15
          ? "Winner"
          : movement != null && movement < -15
            ? "Declining lead"
            : "Monitor";
    return { detail, metrics, value, movement, signal };
  });
  const visible = records.filter(
    (record) =>
      filter === "all" ||
      (filter === "winners" && record.signal === "Winner") ||
      (filter === "attention" && record.signal === "Declining lead") ||
      (filter === "undertested" && record.signal === "Under-tested"),
  );
  useEffect(() => {
    if (!selected) {
      onSurface({
        page: "creatives",
        title: "Creative performance",
        subtitle: `${account?.name ?? "Advertiser"} · ${details.length} ads with creative evidence`,
        accountId: account?.id,
        recommendation:
          "Use performance signals to choose what to inspect; do not label fatigue without frequency and asset-age evidence.",
        actionLabel: "Analyze creatives",
        actionPrompt:
          "Analyze ad-level creative performance for the selected period. Separate winners, declining leads, and insufficient delivery without claiming fatigue from aggregate metrics alone.",
      });
      return;
    }
    const record = records.find((item) => item.detail.ad.id === selected.ad.id);
    onSurface({
      page: "creatives",
      title: selected.creative?.name ?? selected.ad.name,
      subtitle: `Creative · ${selected.identity?.name ?? "Identity unavailable"}`,
      accountId: account?.id,
      entityLevel: "ad",
      entityId: selected.ad.id,
      finding: record
        ? {
            label: "Creative signal",
            value: record.signal,
            detail: `ROAS ${format(record.value, 2)}× · CTR ${ctr(record.metrics) == null ? "—" : `${format(ctr(record.metrics)! * 100, 2)}%`} · trend ${signedPercent(record.movement)}.`,
            tone:
              record.signal === "Winner"
                ? "success"
                : record.signal === "Declining lead"
                  ? "warning"
                  : "muted",
          }
        : undefined,
      confidence: {
        label: "Directional",
        detail:
          "Equal-period ad metrics are available, but frequency and asset age are not.",
      },
      recommendation:
        "Inspect delivery, review state, identity, and downstream conversion evidence before changing this ad.",
      actionLabel: "Diagnose creative",
      actionPrompt: `Diagnose ad ${selected.ad.id}. Read its parent, compare equal periods, inspect available asset and review evidence, and do not claim fatigue unless the required evidence exists.`,
    });
  }, [account?.id, details.length, current, previous, selected, onSurface]);
  if (loading || loadError)
    return (
      <ReportLoadState
        title="Creatives"
        loading={loading}
        onRetry={() => setReload((value) => value + 1)}
      />
    );
  return (
    <div className="space-y-6">
      <PageHeading
        title="Creatives"
        action={
          <Button
            variant="default"
            onClick={() =>
              onAsk(
                "Analyze creative performance for the selected period and prioritize the next asset to inspect.",
              )
            }
          >
            <Sparkles />
            Analyze
          </Button>
        }
      />
      <Card className="overflow-hidden">
        <div className="flex flex-wrap gap-1 border-b border-border p-2">
          {(["all", "winners", "attention", "undertested"] as const).map(
            (item) => (
              <Button
                key={item}
                variant={filter === item ? "secondary" : "ghost"}
                size="sm"
                onClick={() => setFilter(item)}
              >
                {item === "attention"
                  ? "Needs attention"
                  : item === "undertested"
                    ? "Under-tested"
                    : titleCase(item)}
              </Button>
            ),
          )}
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[920px] text-left text-sm">
            <thead className="border-b border-border bg-muted/35 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3">Creative</th>
                <th className="px-3 py-3">Spend</th>
                <th className="px-3 py-3">Results</th>
                <th className="px-3 py-3">CPA</th>
                <th className="px-3 py-3">ROAS</th>
                <th className="px-3 py-3">CTR</th>
                <th className="px-3 py-3">Signal</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {visible.map((record) => (
                <tr
                  key={record.detail.ad.id}
                  className="cursor-pointer hover:bg-muted/30"
                  onClick={() => setSelected(record.detail)}
                >
                  <td className="max-w-[340px] px-4 py-3">
                    <div className="flex items-center gap-3">
                      <CreativePreview detail={record.detail} compact />
                      <span className="min-w-0">
                        <strong className="block truncate font-medium">
                          {record.detail.creative?.name ??
                            record.detail.ad.name}
                        </strong>
                        <span className="block truncate text-xs text-muted-foreground">
                          {record.detail.ad.name}
                        </span>
                      </span>
                    </div>
                  </td>
                  <td className="px-3 py-3 tabular-nums">
                    {format(record.metrics?.spend)}
                  </td>
                  <td className="px-3 py-3 tabular-nums">
                    {format(record.metrics?.conversions, 1)}
                  </td>
                  <td className="px-3 py-3 tabular-nums">
                    {cpa(record.metrics) == null
                      ? "—"
                      : format(cpa(record.metrics), 2)}
                  </td>
                  <td className="px-3 py-3 tabular-nums">
                    {record.value == null ? "—" : `${format(record.value, 2)}×`}
                  </td>
                  <td className="px-3 py-3 tabular-nums">
                    {ctr(record.metrics) == null
                      ? "—"
                      : `${format(ctr(record.metrics)! * 100, 2)}%`}
                  </td>
                  <td className="px-3 py-3">
                    <Badge
                      tone={
                        record.signal === "Winner"
                          ? "success"
                          : record.signal === "Declining lead"
                            ? "warning"
                            : "muted"
                      }
                    >
                      {record.signal}
                    </Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
      {selected && (
        <Card>
          <CardHeader className="flex-row items-start justify-between">
            <div>
              <CardTitle>
                {selected.creative?.name ?? selected.ad.name}
              </CardTitle>
              <CardDescription>
                {selected.identity?.name ?? "Identity unavailable"} ·{" "}
                {selected.creative?.review_status ?? "Review unavailable"}
              </CardDescription>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelected(undefined)}
            >
              Close
            </Button>
          </CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-[180px_1fr]">
            <CreativePreview detail={selected} />
            <div className="min-w-0">
              <p className="text-sm leading-relaxed">
                {selected.primary_text ?? "Primary text unavailable"}
              </p>
              <div className="mt-4 flex flex-wrap gap-2">
                <Badge tone="muted">
                  {selected.format ?? "Format unavailable"}
                </Badge>
                <Badge tone="muted">
                  {selected.call_to_action ?? "CTA unavailable"}
                </Badge>
              </div>
              <p className="mt-4 text-xs leading-relaxed text-muted-foreground">
                Frequency and creative age unavailable; fatigue is unconfirmed.
              </p>
              {selected.limitations?.map((limitation) => (
                <p
                  key={limitation}
                  className="mt-2 text-xs leading-relaxed text-muted-foreground"
                >
                  {limitation}
                </p>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function ChangesWorkspace({
  changes,
  applying,
  onAction,
  onRefresh,
}: {
  changes: Change[];
  applying: boolean;
  onAction: (change: Change, action: ChangeAction) => void;
  onRefresh: () => void;
}) {
  const [tab, setTab] = useState<
    "staged" | "applied" | "attention" | "rejected" | "all"
  >("staged");
  const visible = changes.filter(
    (change) =>
      tab === "all" ||
      (tab === "staged" && change.state === "staged") ||
      (tab === "applied" && change.state === "applied") ||
      (tab === "rejected" && change.state === "discarded") ||
      (tab === "attention" &&
        ["indeterminate", "failed", "expired"].includes(change.state)),
  );
  return (
    <div className="space-y-6">
      <PageHeading
        title="Changes"
        description="Review and approve pending changes."
        action={
          <Button variant="secondary" onClick={onRefresh}>
            <RefreshCw />
            Refresh
          </Button>
        }
      />
      <div
        className="flex flex-wrap gap-1 border-b border-border"
        role="tablist"
        aria-label="Change states"
      >
        {(["staged", "applied", "attention", "rejected", "all"] as const).map(
          (item) => (
            <Button
              key={item}
              role="tab"
              aria-selected={tab === item}
              variant="ghost"
              size="sm"
              className={cn(
                "rounded-b-none",
                tab === item && "border-b-2 border-foreground",
              )}
              onClick={() => setTab(item)}
            >
              {item === "staged"
                ? `Staged ${changes.filter((change) => change.state === "staged").length}`
                : item === "applied"
                  ? `Applied ${changes.filter((change) => change.state === "applied").length}`
                  : item === "attention"
                    ? `Needs attention ${changes.filter((change) => ["indeterminate", "failed", "expired"].includes(change.state)).length}`
                    : item === "rejected"
                      ? `Rejected ${changes.filter((change) => change.state === "discarded").length}`
                      : "All"}
            </Button>
          ),
        )}
      </div>
      {visible.length ? (
        <div className="grid gap-4 lg:grid-cols-2">
          {visible.map((change) => (
            <div key={change.id} className="space-y-2">
              <PresentationCard
                card={{ id: change.id, type: "change", change }}
                onSuggest={() => {}}
                onAction={onAction}
                busy={applying}
              />
              {change.state === "applied" && (
                <div className="rounded-lg border border-border bg-muted/30 px-3 py-2 text-xs">
                  <strong>Execution verified by read-back.</strong>
                  <span className="ml-1 text-muted-foreground">
                    Performance impact is not evaluated until a comparable
                    post-change window exists.
                  </span>
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <Card className="border-dashed py-16 text-center">
          <ShieldCheck className="mx-auto size-5" />
          <CardTitle className="mt-4">No changes in this view</CardTitle>
          <CardDescription className="mx-auto mt-2 max-w-sm">
            A recommendation is not a change. Only an exact staged draft appears
            here.
          </CardDescription>
        </Card>
      )}
    </div>
  );
}

function StagedChangesTray({
  changes,
  onReview,
}: {
  changes: Change[];
  onReview: () => void;
}) {
  const first = changes[0];
  if (!first) return null;
  const target = changeTarget(first);
  const summary = changeSummary(first);
  return (
    <div
      className="shrink-0 border-t border-border bg-background px-4 py-3 shadow-[0_-8px_24px_rgba(0,0,0,.04)]"
      aria-label="Staged changes tray"
    >
      <button
        className="mx-auto flex w-full max-w-6xl items-center gap-4 text-left"
        onClick={onReview}
      >
        <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-foreground text-background">
          <FileCheck2 className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold">
            {changes.length} staged change{changes.length === 1 ? "" : "s"}
          </div>
          <div className="truncate text-xs text-muted-foreground">
            {target} · {summary}
          </div>
        </div>
        <Button size="sm" type="button">
          Review
          <ChevronRight />
        </Button>
      </button>
    </div>
  );
}

function ReportPeriodBar({
  period,
  onDays,
}: {
  period?: Period;
  onDays: (days: 7 | 14) => void;
}) {
  return (
    <div
      aria-label="Report period"
      className="shrink-0 border-b border-border bg-muted/20 px-4 py-2 sm:px-6"
    >
      <div className="mx-auto flex max-w-6xl flex-col gap-2 sm:flex-row sm:items-center">
        <div className="min-w-0 flex-1 text-xs tabular-nums text-muted-foreground">
          {period
            ? `${period.start} — ${period.end}`
            : "Loading reporting period…"}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            aria-label="Date range"
            value={period?.days ?? 7}
            onChange={(event) => onDays(Number(event.target.value) as 7 | 14)}
            className="h-8 rounded-md border border-input bg-background px-2 text-xs"
          >
            <option value={7}>Last 7 days</option>
            <option value={14}>Last 14 days</option>
          </select>
          <div className="rounded-md border border-border bg-muted/35 px-2.5 py-1.5 text-xs text-muted-foreground">
            Compare: previous {period?.days ?? 7}d
          </div>
        </div>
      </div>
    </div>
  );
}

function ReportLoadState({
  title,
  loading,
  onRetry,
}: {
  title: string;
  loading: boolean;
  onRetry: () => void;
}) {
  return (
    <div className="space-y-6">
      <PageHeading title={title} />
      <div
        className="rounded-xl border border-border p-5 text-sm"
        role={loading ? "status" : "alert"}
      >
        <p className="text-muted-foreground">
          {loading
            ? "Loading report…"
            : "Report data could not be loaded. Previous metrics are not shown for the selected period."}
        </p>
        {!loading && (
          <Button
            variant="secondary"
            size="sm"
            className="mt-3"
            onClick={onRetry}
          >
            Retry data
          </Button>
        )}
      </div>
    </div>
  );
}

export function Workspace() {
  const [page, setPage] = useState<Page>("today");
  const [account, setAccount] = useState<Account>();
  const [manager, setManager] = useState<ManagerScope>();
  const [selectedAccount, setSelectedAccount] = useState<Account>();
  const [managerReport, setManagerReport] = useState<ManagerReport>();
  const [periodDays, setPeriodDays] = useState<7 | 14>(7);
  const [surface, setSurface] = useState<AgentSurface>({
    page: "today",
    title: "Today",
    subtitle: "Current advertiser operating brief",
  });
  const [config, setConfig] = useState<RuntimeConfig>();
  const [sandboxState, setSandboxState] = useState<SandboxState>();
  const [advancingSandbox, setAdvancingSandbox] = useState(false);
  const [selectedModel, setSelectedModel] = useState<ModelSelection>(() => ({
    provider: "openai-codex",
    model: localStorage.getItem("ad-agent.model") ?? "gpt-5.6-luna",
    reasoning: "medium",
    auth_mode: "chatgpt_oauth",
  }));
  const [error, setError] = useState("");
  const [overview, setOverview] = useState<OverviewData>();
  const overviewRequest = useRef(0);
  const [changes, setChanges] = useState<Change[]>([]);
  const [confirm, setConfirm] = useState<Change>();
  const [applying, setApplying] = useState(false);
  const [assistantOpen, setAssistantOpen] = useState(
    () => localStorage.getItem("ad-agent.assistant") !== "closed",
  );
  const [assistantWidth, setAssistantWidth] = useState(() => {
    const saved = Number(localStorage.getItem("ad-agent.assistant-width"));
    return Number.isFinite(saved) && saved >= 360 && saved <= 720 ? saved : 460;
  });
  const [mobileAssistant, setMobileAssistant] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);
  const [capabilitiesOpen, setCapabilitiesOpen] = useState(
    () => sessionStorage.getItem("ad-agent.open-settings") === "1",
  );
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [memories, setMemories] = useState<Memory[]>([]);
  const [sessionId, setSessionId] = useState(
    () => localStorage.getItem("ad-agent.session") ?? "web",
  );
  const [history, setHistory] = useState<Session>({
    id: "web",
    messages: [],
    model: selectedModel,
  });
  const [message, setMessage] = useState("");
  const [live, dispatch] = useReducer(reduceEvent, emptyLive);
  const [turns, setTurns] = useState<Record<string, typeof emptyLive>>({});
  const [dataRevision, setDataRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [controller, setController] = useState<AbortController>();
  useEffect(() => () => controller?.abort(), [controller]);
  const loadSession = async () => {
    const session = await api<Session>(
      "/session?session_id=" + encodeURIComponent(sessionId),
    );
    setHistory(session);
    dispatch({
      v: "1",
      type: "client.reset",
      turnId: "",
      seq: 0,
      at: "",
      data: {},
    });
    const records: Record<string, typeof emptyLive> = {};
    const replay: Record<string, Event[]> = {};
    const ids = [...new Set(session.messages.map((item) => item.turn_id))];
    // Restore each visible turn, including its tools and cards, with bounded concurrency.
    for (let i = 0; i < ids.length; i += 4) {
      await Promise.all(
        ids.slice(i, i + 4).map(async (id) => {
          const events = await api<Event[]>(
            `/turns/${encodeURIComponent(id)}/events?session_id=${encodeURIComponent(sessionId)}`,
          );
          replay[id] = events;
          records[id] = events.reduce(reduceEvent, emptyLive);
        }),
      );
    }
    setTurns(records);
    const last = ids.at(-1);
    if (last) {
      const record = records[last];
      // Inspector uses the latest turn; transcript retains every turn above.
      const events = replay[last] ?? [];
      if (record) for (const event of events) dispatch(event);
    }
  };
  const loadChanges = async () =>
    setChanges(
      await api<Change[]>(
        "/changes?session_id=" + encodeURIComponent(sessionId),
      ),
    );
  const loadMemories = async () =>
    setMemories(await api<Memory[]>("/memories"));
  const loadDirectOverview = async (days: 7 | 14 = periodDays) => {
    const request = ++overviewRequest.current;
    setOverview(undefined);
    const current = await api<Account>("/advertisers/current");
    if (request !== overviewRequest.current) return;
    setAccount(current);
    const selectedPeriod = periodFor(current.latest_date, days);
    const [currentReport, previousReport] = await Promise.all([
      api<ReportBundle>(reportPath("", "campaign", selectedPeriod)),
      api<ReportBundle>(
        reportPath("", "campaign", {
          start: selectedPeriod.compareStart,
          end: selectedPeriod.compareEnd,
        }),
      ),
    ]);
    if (request !== overviewRequest.current) return;
    setOverview({
      current: currentReport,
      previous: previousReport,
      period: selectedPeriod,
    });
  };
  const loadManagerOverview = async (
    scope: ManagerScope,
    days: 7 | 14 = periodDays,
  ) => {
    const request = ++overviewRequest.current;
    setManagerReport(undefined);
    const latest = scope.accounts
      .map((item) => item.latest_date)
      .filter(Boolean)
      .sort()
      .at(-1);
    if (!latest) return;
    const selectedPeriod = periodFor(latest, days);
    const report = await api<ManagerReport>(
      `/manager/report?start_date=${selectedPeriod.start}&end_date=${selectedPeriod.end}`,
    );
    if (request === overviewRequest.current) setManagerReport(report);
  };
  const advanceSandbox = async (hours: number) => {
    if (advancingSandbox) return;
    setAdvancingSandbox(true);
    try {
      const result = await api<SandboxAdvance>("/sandbox/advance", { hours });
      setSandboxState(result.state);
      await loadDirectOverview();
      setDataRevision((value) => value + 1);
    } catch (reason) {
      setError(String(reason));
    } finally {
      setAdvancingSandbox(false);
    }
  };
  useEffect(() => {
    void api<RuntimeConfig>("/config")
      .then(async (value) => {
        setConfig(value);
        setSelectedModel(value.model.default);
        if (value.mode === "manager") {
          setPage("accounts");
          const scope = await api<ManagerScope>("/manager");
          setManager(scope);
          await loadManagerOverview(scope);
          return;
        }
        await loadDirectOverview();
        if (value.sandbox) setSandboxState(await api<SandboxState>("/sandbox"));
      })
      .catch((reason) => setError(String(reason)));
  }, []);
  useEffect(() => {
    localStorage.setItem("ad-agent.session", sessionId);
    void loadSession().catch((reason) => setError(String(reason)));
    void loadChanges().catch((reason) => setError(String(reason)));
    if (config?.mode === "advertiser")
      void loadMemories().catch((reason) => setError(String(reason)));
  }, [sessionId, config?.mode]);
  useEffect(() => {
    localStorage.setItem(
      "ad-agent.assistant",
      assistantOpen ? "open" : "closed",
    );
  }, [assistantOpen]);
  useEffect(() => {
    localStorage.setItem("ad-agent.assistant-width", String(assistantWidth));
  }, [assistantWidth]);
  const resizeAssistant = (event: React.PointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = assistantWidth;
    const move = (next: PointerEvent) => {
      const maximum = Math.min(720, Math.max(360, window.innerWidth - 560));
      setAssistantWidth(
        Math.max(360, Math.min(maximum, startWidth + startX - next.clientX)),
      );
    };
    const stop = () => {
      document.removeEventListener("pointermove", move);
      document.removeEventListener("pointerup", stop);
    };
    document.addEventListener("pointermove", move);
    document.addEventListener("pointerup", stop);
  };
  useEffect(() => {
    if (page === "changes")
      void loadChanges().catch((reason) => setError(String(reason)));
  }, [page, sessionId]);
  const activeAccount = config?.mode === "manager" ? selectedAccount : account;
  const latestManagerDate = manager?.accounts
    .map((item) => item.latest_date)
    .filter(Boolean)
    .sort()
    .at(-1);
  const period = activeAccount?.latest_date
    ? periodFor(activeAccount.latest_date, periodDays)
    : latestManagerDate
      ? periodFor(latestManagerDate, periodDays)
      : undefined;
  const basePath =
    config?.mode === "manager" && selectedAccount
      ? `/manager/accounts/${encodeURIComponent(selectedAccount.id)}`
      : "";
  const navigate = (next: Page) => {
    if (
      config?.mode === "manager" &&
      !selectedAccount &&
      (next === "campaigns" || next === "creatives")
    ) {
      setPage("accounts");
      return;
    }
    setPage(next);
  };
  const selectManagerAccount = (next?: Account) => {
    setSelectedAccount(next);
    if (next) {
      setPage("campaigns");
      setSurface({
        page: "campaigns",
        title: "Campaign performance",
        subtitle: `${next.name} · loading campaign evidence`,
        accountId: next.id,
      });
    } else {
      setPage("accounts");
    }
  };
  const changePeriod = (days: 7 | 14) => {
    setPeriodDays(days);
    if (config?.mode === "manager") {
      if (manager)
        void loadManagerOverview(manager, days).catch((reason) =>
          setError(String(reason)),
        );
      return;
    }
    void loadDirectOverview(days).catch((reason) => setError(String(reason)));
  };
  useEffect(() => {
    if (page === "today") {
      setSurface({
        page: "today",
        title: "Account briefing",
        subtitle: `${account?.name ?? "Advertiser"} · prioritized operating brief`,
        accountId: account?.id,
        recommendation:
          "Review the highest-impact measured movement before opening the full campaign table.",
        actionLabel: "Analyze today",
        actionPrompt:
          "Give me today's prioritized account briefing for the period on screen. Lead with findings and decision-relevant evidence.",
      });
    } else if (page === "accounts") {
      setSurface({
        page: "accounts",
        title: "Account triage",
        subtitle: `${manager?.name ?? "Manager"} · authorized advertiser scope`,
        recommendation:
          "Choose one account from comparable evidence, then continue in the same campaign and creative workspace.",
        actionLabel: "Triage accounts",
        actionPrompt:
          "Triage the authorized advertiser accounts without combining currencies, and identify the next account to inspect.",
      });
    } else if (page === "changes") {
      const staged = changes.filter(
        (change) => change.state === "staged",
      ).length;
      setSurface({
        page: "changes",
        title: "Change control",
        subtitle:
          config?.mode === "manager" && !selectedAccount
            ? "All authorized accounts"
            : (activeAccount?.name ?? "Current advertiser"),
        accountId: activeAccount?.id,
        finding: staged
          ? {
              label: "Awaiting review",
              value: `${staged} staged`,
              detail:
                "Each draft requires one explicit product approval before execution.",
              tone: "warning",
            }
          : undefined,
        recommendation:
          "Reconcile unknown outcomes before retrying; evaluate performance only after a comparable post-change window exists.",
      });
    }
  }, [
    page,
    account?.id,
    account?.name,
    manager?.id,
    selectedAccount?.id,
    selectedAccount?.name,
    changes,
    config?.mode,
  ]);
  const send = async (text: string) => {
    if (busy || !text.trim()) return;
    setError("");
    setMessage("");
    setBusy(true);
    dispatch({
      v: "1",
      type: "client.reset",
      turnId: "",
      seq: 0,
      at: "",
      data: {},
    });
    const control = new AbortController();
    setController(control);
    setHistory((value) => ({
      ...value,
      messages: [
        ...value.messages,
        { role: "user", text, turn_id: "pending", status: "running" },
      ],
    }));
    try {
      const viewContext: ViewContext = {
        page,
        account_id: activeAccount?.id,
        account_name: activeAccount?.name,
        entity_level: surface.entityLevel,
        entity_id: surface.entityId,
        entity_name: surface.entityId ? surface.title : undefined,
        start_date: period?.start,
        end_date: period?.end,
        compare_start: period?.compareStart,
        compare_end: period?.compareEnd,
      };
      await streamTurn(
        sessionId,
        text,
        selectedModel,
        viewContext,
        control.signal,
        dispatch,
        config?.runtime,
      );
    } catch (reason) {
      setError(
        control.signal.aborted
          ? "Cancellation requested. Refresh the session to inspect saved state."
          : String(reason),
      );
    } finally {
      setBusy(false);
      setController(undefined);
      await loadSession().catch(() => {});
      await loadChanges().catch(() => {});
      if (config?.mode === "advertiser") await loadMemories().catch(() => {});
    }
  };
  const ask = (text: string) => {
    setMessage(text);
    setAssistantOpen(true);
    setMobileAssistant(window.innerWidth < 1024);
  };
  const performChange = async (change: Change, action: ChangeAction) => {
    setApplying(true);
    setError("");
    try {
      await api(`/changes/${encodeURIComponent(change.id)}/${action}`, {
        session_id: sessionId,
      });
      setConfirm(undefined);
      await loadChanges();
    } catch {
      setError(
        "The operation was not confirmed. Refresh and reconcile before retrying.",
      );
    } finally {
      setApplying(false);
    }
  };
  const requestChange = (change: Change, action: ChangeAction) => {
    if (action === "apply") setConfirm(change);
    else void performChange(change, action);
  };
  const newSession = (model = selectedModel) => {
    if (busy || applying) return;
    const id = `web-${Date.now().toString(36)}`;
    setSessionId(id);
    setTurns({});
    setHistory({ id, messages: [], model });
    dispatch({
      v: "1",
      type: "client.reset",
      turnId: "",
      seq: 0,
      at: "",
      data: {},
    });
  };
  const modelLabel =
    config?.model.options.find(
      (option) =>
        modelSelectionKey(option) === modelSelectionKey(selectedModel),
    )?.label ?? selectedModel.model;
  const pending = changes.filter((change) => change.state === "staged").length;
  const stagedChanges = changes.filter((change) => change.state === "staged");
  const pages = config?.mode === "manager" ? managerPages : directPages;
  const assistantProps = {
    sessionId,
    history,
    live,
    turns,
    message,
    setMessage,
    busy,
    controller,
    send,
    newSession: () => newSession(),
    refresh: () => void loadSession(),
    changes,
    onChange: requestChange,
    onOpenInspector: () => {
      setMobileAssistant(false);
      setInspectorOpen(true);
    },
    modelLabel,
    mode: config?.mode ?? "advertiser",
    surface,
    period,
  };
  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <aside className="hidden w-60 shrink-0 flex-col border-r border-border bg-background md:flex">
        <div className="flex h-14 items-center px-4">
          <Brand />
        </div>
        <nav className="space-y-1 border-t border-border px-3 py-4">
          {pages.map(({ id, label, icon: Icon }) => (
            <Button
              key={id}
              variant="ghost"
              className={cn(
                "w-full justify-start text-muted-foreground",
                page === id && "bg-muted text-foreground",
              )}
              onClick={() => navigate(id)}
            >
              <Icon />
              {label}
              {id === "changes" && pending > 0 && (
                <Badge className="ml-auto px-1.5">{pending}</Badge>
              )}
            </Button>
          ))}
        </nav>
        <div className="mt-auto p-3">
          <Button
            variant="ghost"
            className="w-full justify-start text-muted-foreground"
            onClick={() => setCapabilitiesOpen(true)}
          >
            <Settings2 />
            Settings
          </Button>
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header
          aria-label="Workspace toolbar"
          className="flex h-14 shrink-0 items-center gap-3 border-b border-border px-4 sm:px-6"
        >
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            aria-label="Open navigation"
            onClick={() => setMobileNav(true)}
          >
            <Menu />
          </Button>
          <div aria-label="Workspace account" className="min-w-0 flex-1">
            {config?.mode === "manager" ? (
              <select
                aria-label="Account context"
                value={selectedAccount?.id ?? ""}
                onChange={(event) =>
                  selectManagerAccount(
                    manager?.accounts.find(
                      (item) => item.id === event.target.value,
                    ),
                  )
                }
                className="h-7 w-full max-w-64 rounded-md border border-input bg-background px-2 text-xs"
              >
                <option value="">All accounts</option>
                {manager?.accounts.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            ) : (
              <div className="truncate text-sm font-semibold">
                {account?.name ?? "Loading account…"}
              </div>
            )}
            <div className="hidden truncate text-xs text-muted-foreground sm:block">
              {activeAccount
                ? `${activeAccount.currency} · ${activeAccount.timezone}`
                : (manager?.name ?? "Authorized advertising workspace")}
            </div>
          </div>
          <div className="ml-auto flex items-center gap-2">
            {sandboxState && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="sm" aria-label="Sandbox clock">
                    <Clock3 />
                    <span className="hidden sm:inline">Sandbox time</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuLabel>
                    {new Intl.DateTimeFormat("en-US", {
                      timeZone: account?.timezone,
                      dateStyle: "medium",
                      timeStyle: "short",
                    }).format(new Date(sandboxState.current_time))}
                  </DropdownMenuLabel>
                  <DropdownMenuLabel>
                    {account?.timezone} · all pages
                  </DropdownMenuLabel>
                  <DropdownMenuItem
                    disabled={advancingSandbox || busy || applying}
                    onSelect={() => void advanceSandbox(1)}
                  >
                    Advance 1 hour
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    disabled={advancingSandbox || busy || applying}
                    onSelect={() => void advanceSandbox(24)}
                  >
                    Advance 24 hours
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
            <Badge tone="muted">
              {config?.mode === "manager"
                ? `Manager · ${manager?.id ?? "sandbox"}`
                : account?.source.backend === "sandbox"
                  ? `Sandbox · ${account.source.environment}`
                  : "Read-only data"}
            </Badge>
            <Button
              variant="ghost"
              size="icon"
              className="lg:hidden"
              aria-label="Open assistant"
              onClick={() => setMobileAssistant(true)}
            >
              <MessageSquare />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="hidden lg:inline-flex"
              aria-label={assistantOpen ? "Close assistant" : "Open assistant"}
              onClick={() => setAssistantOpen((value) => !value)}
            >
              {assistantOpen ? <PanelRightClose /> : <PanelRightOpen />}
            </Button>
          </div>
        </header>
        {error && (
          <div
            role="alert"
            className="flex items-center justify-between border-b border-red-200 bg-red-50 px-4 py-2 text-xs text-red-900"
          >
            <span>{error}</span>
            <Button variant="ghost" size="sm" onClick={() => setError("")}>
              Dismiss
            </Button>
          </div>
        )}
        <ReportPeriodBar period={period} onDays={changePeriod} />
        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto">
          <main className="mx-auto w-full max-w-6xl px-5 py-7 sm:px-8">
            {page === "today" && (
              <HomeView
                account={account}
                overview={overview}
                changes={changes}
                onAsk={ask}
                onNavigate={navigate}
              />
            )}
            {page === "accounts" && (
              <AccountsView
                manager={manager}
                report={managerReport}
                changes={changes}
                onAsk={ask}
                onNavigate={navigate}
                onSelectAccount={selectManagerAccount}
              />
            )}
            {page === "campaigns" && activeAccount && period && (
              <CampaignWorkspace
                revision={dataRevision}
                account={activeAccount}
                period={period}
                changes={changes}
                basePath={basePath}
                onAsk={ask}
                onSurface={setSurface}
                onBack={
                  config?.mode === "manager"
                    ? () => selectManagerAccount(undefined)
                    : undefined
                }
              />
            )}
            {page === "creatives" && activeAccount && period && (
              <CreativesWorkspace
                revision={dataRevision}
                account={activeAccount}
                period={period}
                basePath={basePath}
                onAsk={ask}
                onSurface={setSurface}
              />
            )}
            {page === "changes" && (
              <ChangesWorkspace
                changes={changes}
                applying={applying}
                onAction={requestChange}
                onRefresh={() => void loadChanges()}
              />
            )}
          </main>
        </div>
        {page !== "changes" && (
          <StagedChangesTray
            changes={stagedChanges}
            onReview={() => navigate("changes")}
          />
        )}
      </div>
      {assistantOpen && (
        <aside
          className="relative hidden shrink-0 border-l border-border lg:block"
          style={{ width: assistantWidth }}
        >
          <div
            role="separator"
            aria-label="Resize assistant"
            aria-orientation="vertical"
            aria-valuemin={360}
            aria-valuemax={720}
            aria-valuenow={assistantWidth}
            tabIndex={0}
            onPointerDown={resizeAssistant}
            onKeyDown={(event) => {
              if (event.key === "ArrowLeft")
                setAssistantWidth((value) => Math.min(720, value + 24));
              if (event.key === "ArrowRight")
                setAssistantWidth((value) => Math.max(360, value - 24));
            }}
            className="group absolute inset-y-0 -left-1 z-20 w-2 cursor-col-resize touch-none focus-visible:outline-none"
          >
            <span className="absolute inset-y-0 left-1/2 w-px -translate-x-1/2 bg-transparent transition-colors group-hover:bg-foreground group-focus:bg-foreground" />
          </div>
          <AssistantPanel {...assistantProps} />
        </aside>
      )}
      <Dialog open={mobileNav} onOpenChange={setMobileNav}>
        <DialogContent className="top-4 translate-y-0">
          <DialogTitle>Ad Desk</DialogTitle>
          <DialogDescription className="sr-only">
            Workspace navigation
          </DialogDescription>
          <div className="mt-5 space-y-1">
            {pages.map(({ id, label, icon: Icon }) => (
              <Button
                key={id}
                variant="ghost"
                className={cn(
                  "w-full justify-start",
                  page === id && "bg-muted",
                )}
                onClick={() => {
                  navigate(id);
                  setMobileNav(false);
                }}
              >
                <Icon />
                {label}
                {id === "changes" && pending > 0 && (
                  <Badge className="ml-auto px-1.5">{pending}</Badge>
                )}
              </Button>
            ))}
          </div>
          <Separator className="my-4" />
          <Button
            variant="ghost"
            className="w-full justify-start"
            onClick={() => {
              setMobileNav(false);
              setCapabilitiesOpen(true);
            }}
          >
            <Settings2 />
            Settings
          </Button>
        </DialogContent>
      </Dialog>
      <Dialog open={mobileAssistant} onOpenChange={setMobileAssistant}>
        <DialogContent
          showClose={false}
          className="inset-0 left-auto top-0 h-[100dvh] w-full max-w-lg translate-x-0 translate-y-0 rounded-none p-0 sm:left-auto"
        >
          <DialogTitle className="sr-only">Ad Agent assistant</DialogTitle>
          <AssistantPanel
            {...assistantProps}
            onClose={() => setMobileAssistant(false)}
          />
        </DialogContent>
      </Dialog>
      <WorkspaceSettings
        open={capabilitiesOpen}
        onOpenChange={setCapabilitiesOpen}
        config={config}
        busy={busy || advancingSandbox || applying}
        sessionId={sessionId}
        onSaved={async (id) => {
          const value = await api<RuntimeConfig>("/config");
          setConfig(value);
          setSelectedModel(value.model.default);
          localStorage.removeItem("ad-agent.model");
          if (id === sessionId) {
            // Settings replace execution, not the conversation, draft or navigation.
            await loadSession();
            await loadChanges();
            setDataRevision((revision) => revision + 1);
            sessionStorage.removeItem("ad-agent.open-settings");
            return;
          }
          setTurns({});
          setHistory({ id, messages: [], model: value.model.default });
          setSessionId(id);
          setPage("today");
          dispatch({
            v: "1",
            type: "client.reset",
            turnId: "",
            seq: 0,
            at: "",
            data: {},
          });
          if (value.mode === "manager") {
            const scope = await api<ManagerScope>("/manager");
            setManager(scope);
            setSelectedAccount(undefined);
            setPage("accounts");
            await loadManagerOverview(scope);
          } else {
            await loadDirectOverview();
          }
          setSandboxState(
            value.sandbox ? await api<SandboxState>("/sandbox") : undefined,
          );
          setDataRevision((value) => value + 1);
          sessionStorage.removeItem("ad-agent.open-settings");
        }}
      />
      <Dialog open={inspectorOpen} onOpenChange={setInspectorOpen}>
        <DialogContent className="right-0 left-auto top-0 h-[100dvh] w-full max-w-md translate-x-0 translate-y-0 rounded-none p-0">
          <div className="border-b border-border px-5 py-4">
            <DialogTitle>
              {config?.mode === "manager" ? "Activity" : "Activity and memory"}
            </DialogTitle>
            <DialogDescription>
              Public execution facts for the latest reply. Private reasoning and
              provider state are never shown.
            </DialogDescription>
          </div>
          <ScrollArea className="h-[calc(100dvh-110px)]">
            <div className="space-y-8 p-5">
              <section>
                <h3 className="text-xs font-medium text-muted-foreground">
                  Latest reply steps
                </h3>
                <div className="mt-3 divide-y divide-border rounded-lg border border-border">
                  {live.tools.length ? (
                    live.tools.map((tool) => (
                      <div key={tool.id} className="px-3 py-2.5 text-xs">
                        <ToolActivityRow
                          tool={tool}
                          names={toolNames}
                          running={busy}
                        />
                      </div>
                    ))
                  ) : (
                    <p className="px-3 py-6 text-center text-xs text-muted-foreground">
                      No tool calls in the latest reply.
                    </p>
                  )}
                </div>
              </section>
              {config?.mode === "advertiser" && (
                <section>
                  <h3 className="text-xs font-medium text-muted-foreground">
                    Business memory
                  </h3>
                  <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                    Durable operator preferences, constraints, and goals only.
                  </p>
                  <div className="mt-3 space-y-2">
                    {memories.length ? (
                      memories.map((memory) => (
                        <div
                          key={memory.id}
                          className="rounded-lg border border-border p-3"
                        >
                          <div className="flex items-center justify-between gap-3">
                            <Badge tone="muted">{memory.kind}</Badge>
                            {memory.key && (
                              <code className="truncate text-xs text-muted-foreground">
                                {memory.key}
                              </code>
                            )}
                          </div>
                          <p className="mt-2 text-sm leading-relaxed">
                            {memory.text}
                          </p>
                        </div>
                      ))
                    ) : (
                      <p className="rounded-lg border border-dashed border-border px-3 py-6 text-center text-xs text-muted-foreground">
                        Nothing saved yet.
                      </p>
                    )}
                  </div>
                </section>
              )}
            </div>
          </ScrollArea>
        </DialogContent>
      </Dialog>
      <Dialog
        open={Boolean(confirm)}
        onOpenChange={(open) => !open && setConfirm(undefined)}
      >
        <DialogContent>
          <DialogTitle>Confirm this one change</DialogTitle>
          <DialogDescription>
            The Change Service will reread the object, rerun guardrails, send at
            most once, and wait for read-back. An unknown outcome will not be
            retried automatically.
          </DialogDescription>
          {confirm && (
            <div className="mt-5 rounded-lg bg-muted p-4">
              <div className="text-sm font-medium">{changeTarget(confirm)}</div>
              {confirm.kind === "operation" ? (
                <div className="mt-2 text-sm text-muted-foreground">
                  {changeKindLabel(confirm)} · Exact fields below
                </div>
              ) : (
                <div className="mt-3 flex items-center justify-between gap-3 tabular-nums">
                  <span className="text-muted-foreground line-through">
                    {confirm.kind === "create"
                      ? "Not created"
                      : confirm.kind === "budget"
                        ? `${format(confirm.before?.budget)} ${confirm.currency}`
                        : confirm.before?.status}
                  </span>
                  <ChevronRight className="size-4" />
                  <strong>
                    {confirm.kind === "create"
                      ? `${confirm.create?.level ?? "entity"} · ${confirm.create?.status ?? "DISABLE"}`
                      : confirm.kind === "budget"
                        ? `${format(confirm.after?.budget)} ${confirm.currency}`
                        : confirm.after?.status}
                  </strong>
                </div>
              )}
            </div>
          )}
          {confirm?.kind === "operation" && (
            <div className="mt-4">
              <OperationReview change={confirm} />
            </div>
          )}
          <div className="mt-6 flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={applying}
              onClick={() => setConfirm(undefined)}
            >
              Return to review
            </Button>
            <Button
              disabled={applying}
              onClick={() => confirm && void performChange(confirm, "apply")}
            >
              {applying ? "Applying and reconciling…" : "Confirm and apply"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default function App() {
  const [checking, setChecking] = useState(true);
  const [ready, setReady] = useState(false);
  const [expired, setExpired] = useState(false);
  useEffect(
    () =>
      onSessionExpired(() => {
        setReady(false);
        setExpired(true);
      }),
    [],
  );
  useEffect(() => {
    let active = true;
    void api<{ csrf: string }>("/auth")
      .then((value) => {
        if (!active) return;
        setCSRF(value.csrf);
        setReady(true);
      })
      .catch(() => {
        if (active) setReady(false);
      })
      .finally(() => {
        if (active) setChecking(false);
      });
    return () => {
      active = false;
    };
  }, []);
  if (checking)
    return (
      <main className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
        Connecting to the local workspace…
      </main>
    );
  return ready ? (
    <Workspace />
  ) : (
    <Login
      expired={expired}
      onReady={() => {
        setExpired(false);
        setReady(true);
      }}
    />
  );
}
