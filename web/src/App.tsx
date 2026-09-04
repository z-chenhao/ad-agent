import React, { useEffect, useReducer, useRef, useState } from "react";
import {
  Activity,
  BarChart3,
  Bot,
  ChevronRight,
  CircleHelp,
  Command,
  FileCheck2,
  Gauge,
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
  Square,
  X,
} from "lucide-react";
import { api, setCSRF, streamTurn } from "./api";
import { emptyLive, reduceEvent } from "./reducer";
import type {
  Account,
  Calculation,
  Card as CardRecord,
  Change,
  Entity,
  Event,
  Memory,
  Report,
  RuntimeConfig,
  ModelSelection,
  Portfolio,
  PortfolioReport,
  Session,
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
import { ScrollArea } from "./components/ui/scroll-area";
import { Separator } from "./components/ui/separator";
import { Textarea } from "./components/ui/textarea";
import { cn } from "./lib/utils";
import {
  EntityTable,
  format,
  MetricStrip,
  PresentationCard,
  stateText,
} from "./components/presentation";

type Page = "home" | "portfolio" | "campaigns" | "changes";
type ChangeAction = "apply" | "discard" | "reconcile";

const directPages: {
  id: Page;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}[] = [
  { id: "home", label: "Home", icon: Gauge },
  { id: "campaigns", label: "Campaigns", icon: LayoutList },
  { id: "changes", label: "Changes", icon: FileCheck2 },
];
const portfolioPages: typeof directPages = [
  { id: "portfolio", label: "Portfolio", icon: LayoutList },
  { id: "changes", label: "Changes", icon: FileCheck2 },
];

const toolNames: Record<string, string> = {
  get_advertiser_context: "Read account",
  list_campaigns: "Read campaigns",
  list_ad_groups: "Read ad groups",
  list_ads: "Read ads",
  get_entity: "Verify object",
  get_performance_report: "Read performance",
  run_analysis: "Delegate analysis",
  analysis_calculate: "Compute evidence",
  analysis_get_dataset: "Read snapshot",
  submit_analysis: "Submit analysis",
  present_metrics: "Present metrics",
  present_entities: "Present objects",
  present_digest: "Present briefing",
  present_change_preview: "Present change",
  present_suggestions: "Offer next steps",
  load_skill: "Load workflow",
  stage_budget_change: "Stage budget change",
  stage_status_change: "Stage status change",
  stage_entity_create: "Stage object creation",
  get_pending_changes: "Read changes",
  list_advertisers: "Read advertisers",
  get_portfolio_performance: "Compare account performance",
  run_portfolio_analysis: "Delegate portfolio analysis",
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

function Login({ onReady }: { onReady: () => void }) {
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
            Analyze TikTok advertising performance, prepare bounded changes, and
            keep approval with the operator.
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
            Open your workspace
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
              } catch {
                setError(
                  "Login failed. Check the key in the local operator-key file.",
                );
              } finally {
                setBusy(false);
              }
            }}
          >
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
        <div className="text-sm font-semibold tracking-tight">Ad Agent</div>
        <div className="text-[10px] uppercase tracking-[.16em] text-muted-foreground">
          Operations
        </div>
      </div>
    </div>
  );
}

function AssistantPanel({
  sessionId,
  history,
  live,
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
  onClose,
}: {
  sessionId: string;
  history: Session;
  live: typeof emptyLive;
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
  onClose?: () => void;
}) {
  const end = useRef<HTMLDivElement>(null);
  const composerID = onClose ? "message-mobile" : "message-desktop";
  const composerLabel = onClose
    ? "Your advertising question on mobile"
    : "Your advertising question";
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end" });
  }, [history.messages.length, live.text, live.cards.length]);
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
  const suggestions =
    mode === "portfolio"
      ? [
          "Triage the advertiser portfolio and name the next account to inspect.",
          "Compare account-level performance without combining currencies.",
          "Show pending changes grouped by advertiser.",
        ]
      : [
          "Give me today's prioritized account briefing.",
          "Compare the latest seven days with the prior seven days.",
          "Show campaigns that need delivery attention.",
        ];
  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border px-4">
        <div className="flex size-7 items-center justify-center rounded-md bg-foreground text-background">
          <Bot className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold">Ad Agent</div>
          <div className="truncate text-[11px] text-muted-foreground">
            Session {sessionId} · {modelLabel}
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Open activity and memory"
          title="Activity and memory"
          onClick={onOpenInspector}
        >
          <Activity />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          aria-label="New session"
          title="New session"
          onClick={newSession}
        >
          <Plus />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Refresh history"
          title="Refresh history"
          onClick={refresh}
        >
          <RefreshCw />
        </Button>
        {onClose && (
          <Button
            variant="ghost"
            size="icon"
            aria-label="Close assistant"
            onClick={onClose}
          >
            <X />
          </Button>
        )}
      </header>
      <ScrollArea className="min-h-0 flex-1">
        <div className="mx-auto flex w-full max-w-xl flex-col gap-5 px-4 py-5">
          {!history.messages.length && !live.text && (
            <div className="py-14 text-center">
              <div className="mx-auto flex size-9 items-center justify-center rounded-lg border border-border">
                <Bot className="size-4" />
              </div>
              <h3 className="mt-4 text-sm font-semibold">
                What should we look at?
              </h3>
              <p className="mx-auto mt-2 max-w-xs text-xs leading-relaxed text-muted-foreground">
                Ask for a briefing, investigate performance, inspect an object,
                or prepare a reviewable change.
              </p>
              <div className="mt-5 flex flex-col gap-2">
                {suggestions.map((text) => (
                  <Button
                    key={text}
                    variant="secondary"
                    size="sm"
                    className="h-auto justify-start whitespace-normal py-2 text-left"
                    onClick={() => void send(text)}
                  >
                    {text}
                  </Button>
                ))}
              </div>
            </div>
          )}
          {history.messages.map((item, index) => (
            <div
              key={`${item.turn_id}-${index}`}
              className={cn(
                "max-w-[92%] text-sm leading-relaxed",
                item.role === "user"
                  ? "ml-auto rounded-2xl rounded-br-md bg-foreground px-3.5 py-2.5 text-background"
                  : "mr-auto",
              )}
            >
              {item.role === "assistant" && (
                <div className="mb-1.5 text-[11px] font-medium text-muted-foreground">
                  Ad Agent
                </div>
              )}
              <p className="whitespace-pre-wrap">{item.text}</p>
              {item.status !== "completed" && item.status !== "running" && (
                <Badge tone="warning" className="mt-2">
                  {stateText(item.status)}
                </Badge>
              )}
            </div>
          ))}
          {live.text && (
            <div className="mr-auto max-w-[92%] text-sm leading-relaxed">
              <div className="mb-1.5 text-[11px] font-medium text-muted-foreground">
                Ad Agent
              </div>
              <p className="whitespace-pre-wrap">
                {live.text}
                <span className={busy ? "streaming-caret" : ""} />
              </p>
            </div>
          )}
          {cards.map((card) => (
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
          {busy && !live.text && !live.cards.length && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="size-1.5 animate-pulse rounded-full bg-foreground" />
              Reading the {mode === "portfolio" ? "portfolio" : "account"} context…
            </div>
          )}
          <div ref={end} />
        </div>
      </ScrollArea>
      <div className="shrink-0 border-t border-border bg-background p-3">
        <form
          className="rounded-xl border border-border bg-background shadow-sm focus-within:ring-2 focus-within:ring-ring"
          onSubmit={(event) => {
            event.preventDefault();
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
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                void send(message);
              }
            }}
          />
          <div className="flex items-center justify-between px-2 pb-2">
            <span className="text-[10px] text-muted-foreground">
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
  overview?: { report: Report; calculation: Calculation | null };
  changes: Change[];
  onAsk: (text: string) => void;
  onNavigate: (page: Page) => void;
}) {
  const staged = changes.filter((change) => change.state === "staged");
  return (
    <div className="space-y-8">
      <PageHeading
        eyebrow="Today"
        title="Account overview"
        description={`${account?.name ?? "Reading account…"} · ${account?.currency ?? "—"} · ${account?.timezone ?? "—"}`}
        action={
          <Button
            variant="secondary"
            onClick={() =>
              onAsk(
                "Give me today's prioritized account briefing. Use one digest and show only decision-relevant evidence.",
              )
            }
          >
            <Bot />
            Ask for a briefing
          </Button>
        }
      />
      {account?.source.backend === "sandbox" && (
        <div className="flex items-center justify-between gap-4 rounded-lg border border-border bg-muted/45 px-4 py-2.5 text-xs">
          <span>
            <strong>Sandbox workspace</strong>
            <span className="ml-2 text-muted-foreground">
              Environment: {account.source.environment}. Persistent fictional
              state for end-to-end validation; no live ads are modified.
            </span>
          </span>
          <span className="shrink-0 text-muted-foreground">
            Through {account.latest_date}
          </span>
        </div>
      )}
      <Card>
        <CardHeader className="flex-row items-end justify-between">
          <div>
            <CardTitle>Latest seven days</CardTitle>
            <CardDescription className="mt-1">
              {overview?.report.query.start_date} —{" "}
              {overview?.report.query.end_date} ·{" "}
              {overview?.report.complete
                ? "complete coverage"
                : "coverage unconfirmed"}
            </CardDescription>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              onAsk(
                "Compare this seven-day period with the prior seven days. Explain contribution and counter-evidence.",
              )
            }
          >
            Analyze
            <ChevronRight />
          </Button>
        </CardHeader>
        <CardContent>
          {overview ? (
            <MetricStrip
              metrics={overview.report.totals}
              roas={overview.calculation?.roas}
              currency={overview.report.currency}
            />
          ) : (
            <div className="h-20 animate-pulse rounded-lg bg-muted" />
          )}
        </CardContent>
      </Card>
      <div className="grid gap-6 lg:grid-cols-[1.25fr_.75fr]">
        <Card>
          <CardHeader>
            <CardTitle>What needs attention</CardTitle>
            <CardDescription>
              A short decision queue, not a dashboard of every available metric.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 p-2 pt-3">
            <DecisionRow
              icon={BarChart3}
              title="Validate the largest performance movement"
              description="Compare equal windows before changing delivery or budget."
              action="Analyze movement"
              onClick={() =>
                onAsk(
                  "Compare the latest seven days with the prior seven days and identify the largest campaign contribution.",
                )
              }
            />
            <DecisionRow
              icon={ShieldCheck}
              title={
                staged.length
                  ? `${staged.length} change${staged.length === 1 ? "" : "s"} awaiting review`
                  : "No changes awaiting review"
              }
              description={
                staged.length
                  ? "Each draft remains unapplied until you approve it."
                  : "The assistant can prepare a bounded draft when an adjustment is needed."
              }
              action={staged.length ? "Review changes" : "Inspect campaigns"}
              onClick={() =>
                onNavigate(staged.length ? "changes" : "campaigns")
              }
            />
            <DecisionRow
              icon={CircleHelp}
              title="Keep data limitations visible"
              description={
                account?.limitations[0] ??
                "Read source, freshness, and attribution before drawing conclusions."
              }
              action="Ask about readiness"
              onClick={() =>
                onAsk(
                  "Audit account and measurement readiness. Separate verified capabilities from unknowns.",
                )
              }
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Recent changes</CardTitle>
            <CardDescription>
              The latest proposals and verified outcomes.
            </CardDescription>
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
                      {change.before?.name ?? change.created?.name ?? change.create?.name ?? "New entity"}
                    </strong>
                    <span className="text-xs text-muted-foreground">
                      {change.kind === "create" ? "Create" : change.kind === "budget" ? "Budget" : "Delivery status"}
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
                No proposed changes in this session.
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function PortfolioView({
  portfolio,
  report,
  changes,
  onAsk,
  onNavigate,
}: {
  portfolio?: Portfolio;
  report?: PortfolioReport;
  changes: Change[];
  onAsk: (text: string) => void;
  onNavigate: (page: Page) => void;
}) {
  const staged = changes.filter((change) => change.state === "staged");
  return (
    <div className="space-y-8">
      <PageHeading
        eyebrow="Authorized scope"
        title="Advertiser portfolio"
        description={`${portfolio?.name ?? "Reading portfolio…"} · ${portfolio?.accounts.length ?? 0} independently scoped accounts`}
        action={
          <Button
            variant="secondary"
            onClick={() =>
              onAsk(
                "Triage this advertiser portfolio. Rank only accounts with comparable evidence, preserve currency and attribution boundaries, and propose the next drill-down.",
              )
            }
          >
            <Bot />
            Triage portfolio
          </Button>
        }
      />
      <div className="flex items-center justify-between gap-4 rounded-lg border border-border bg-muted/45 px-4 py-2.5 text-xs">
        <span>
          <strong>Portfolio sandbox</strong>
          <span className="ml-2 text-muted-foreground">
            Each fictional advertiser has isolated persistent state. Batch
            requests create independent drafts and never imply approval.
          </span>
        </span>
        {staged.length > 0 && (
          <Button variant="ghost" size="sm" onClick={() => onNavigate("changes")}>
            {staged.length} awaiting review
          </Button>
        )}
      </div>
      <Card className="overflow-hidden">
        <CardHeader className="flex-row items-end justify-between">
          <div>
            <CardTitle>Account-level performance</CardTitle>
            <CardDescription className="mt-1">
              {report?.start_date ?? "—"} — {report?.end_date ?? "—"} · no
              cross-currency total
            </CardDescription>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              onAsk(
                "Identify which advertiser needs attention first, then drill into only that account. Explain missing or non-comparable evidence.",
              )
            }
          >
            Prioritize
            <ChevronRight />
          </Button>
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
                  <tr key={item.account.id}>
                    <td className="px-5 py-4">
                      <strong className="block font-medium">{item.account.name}</strong>
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
                        onClick={() =>
                          onAsk(
                            `Diagnose advertiser ${item.account.id} (${item.account.name}). Read its campaigns first and keep every proposed change scoped to this advertiser.`,
                          )
                        }
                      >
                        Diagnose
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
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
      <div>
        <p className="text-[11px] font-medium uppercase tracking-[.15em] text-muted-foreground">
          {eyebrow}
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-[-.025em]">
          {title}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      </div>
      {action}
    </div>
  );
}

export function Workspace() {
  const [page, setPage] = useState<Page>("home");
  const [account, setAccount] = useState<Account>();
  const [portfolio, setPortfolio] = useState<Portfolio>();
  const [portfolioReport, setPortfolioReport] = useState<PortfolioReport>();
  const [config, setConfig] = useState<RuntimeConfig>();
  const [selectedModel, setSelectedModel] = useState<ModelSelection>(() => ({
    provider: "openai-codex",
    model: localStorage.getItem("ad-agent.model") ?? "gpt-5.6-luna",
    reasoning: "medium",
    auth_mode: "chatgpt_oauth",
  }));
  const [error, setError] = useState("");
  const [overview, setOverview] = useState<{
    report: Report;
    calculation: Calculation | null;
  }>();
  const [entities, setEntities] = useState<Entity[]>([]);
  const [level, setLevel] = useState<"campaign" | "ad_group" | "ad">(
    "campaign",
  );
  const [path, setPath] = useState<Entity[]>([]);
  const [changes, setChanges] = useState<Change[]>([]);
  const [confirm, setConfirm] = useState<Change>();
  const [applying, setApplying] = useState(false);
  const [assistantOpen, setAssistantOpen] = useState(
    () => localStorage.getItem("ad-agent.assistant") !== "closed",
  );
  const [mobileAssistant, setMobileAssistant] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);
  const [capabilitiesOpen, setCapabilitiesOpen] = useState(false);
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
  const [busy, setBusy] = useState(false);
  const [controller, setController] = useState<AbortController>();
  const loadSession = async () => {
    const session = await api<Session>(
      "/session?session_id=" + encodeURIComponent(sessionId),
    );
    setHistory(session);
    if (session.model?.provider) setSelectedModel(session.model);
    dispatch({
      v: "0",
      type: "client.reset",
      turnId: "",
      seq: 0,
      at: "",
      data: {},
    });
    const turn = session.messages.at(-1)?.turn_id;
    if (turn) {
      const events = await api<Event[]>(
        `/turns/${encodeURIComponent(turn)}/events?session_id=${encodeURIComponent(sessionId)}`,
      );
      for (const event of events) dispatch(event);
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
  useEffect(() => {
    void api<RuntimeConfig>("/config")
      .then(async (value) => {
        setConfig(value);
        setSelectedModel((current) =>
          value.model.options.some(
            (candidate) =>
              modelSelectionKey(candidate) === modelSelectionKey(current),
          )
            ? current
            : value.model.default,
        );
        if (value.mode === "portfolio") {
          setPage("portfolio");
          const scope = await api<Portfolio>("/portfolio");
          setPortfolio(scope);
          const latest = scope.accounts
            .map((item) => item.latest_date)
            .filter(Boolean)
            .sort()
            .at(-1);
          if (latest) {
            const rangeStart = new Date(latest + "T00:00:00Z");
            rangeStart.setUTCDate(rangeStart.getUTCDate() - 6);
            setPortfolioReport(
              await api<PortfolioReport>(
                `/portfolio/report?start_date=${rangeStart.toISOString().slice(0, 10)}&end_date=${latest}`,
              ),
            );
          }
          return;
        }
        const current = await api<Account>("/advertisers/current");
        setAccount(current);
        const rangeStart = new Date(current.latest_date + "T00:00:00Z");
        rangeStart.setUTCDate(rangeStart.getUTCDate() - 6);
        setOverview(
          await api(
            `/report?level=campaign&start_date=${rangeStart.toISOString().slice(0, 10)}&end_date=${current.latest_date}`,
          ),
        );
      })
      .catch((reason) => setError(String(reason)));
  }, []);
  useEffect(() => {
    localStorage.setItem("ad-agent.session", sessionId);
    void loadSession().catch((reason) => setError(String(reason)));
    void loadChanges().catch((reason) => setError(String(reason)));
    if (config?.mode === "single_advertiser")
      void loadMemories().catch((reason) => setError(String(reason)));
  }, [sessionId, config?.mode]);
  useEffect(() => {
    localStorage.setItem(
      "ad-agent.assistant",
      assistantOpen ? "open" : "closed",
    );
  }, [assistantOpen]);
  useEffect(() => {
    if (page === "campaigns" && config?.mode === "single_advertiser")
      void api<Entity[]>(
        `/entities/${level}?parent_id=${encodeURIComponent(path.at(-1)?.id ?? "")}`,
      )
        .then(setEntities)
        .catch((reason) => setError(String(reason)));
    if (page === "changes")
      void loadChanges().catch((reason) => setError(String(reason)));
  }, [page, level, path, sessionId]);
  const send = async (text: string) => {
    if (busy || !text.trim()) return;
    setError("");
    setMessage("");
    setBusy(true);
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
      await streamTurn(
        sessionId,
        text,
        selectedModel,
        control.signal,
        dispatch,
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
      if (config?.mode === "single_advertiser")
        await loadMemories().catch(() => {});
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
    const id = `web-${Date.now().toString(36)}`;
    setSessionId(id);
    setHistory({ id, messages: [], model });
    dispatch({
      v: "0",
      type: "client.reset",
      turnId: "",
      seq: 0,
      at: "",
      data: {},
    });
  };
  const selectModel = (key: string) => {
    const option = config?.model.options.find(
      (candidate) => modelSelectionKey(candidate) === key,
    );
    if (!option || busy) return;
    const { label: _label, ...configured } = option;
    const selection: ModelSelection = { ...configured, reasoning: "medium" };
    setSelectedModel(selection);
    localStorage.setItem("ad-agent.model", option.model);
    newSession(selection);
  };
  const modelLabel =
    config?.model.options.find(
      (option) => modelSelectionKey(option) === modelSelectionKey(selectedModel),
    )?.label ?? selectedModel.model;
  const pending = changes.filter((change) => change.state === "staged").length;
  const pages = config?.mode === "portfolio" ? portfolioPages : directPages;
  const pageTitle = pages.find((item) => item.id === page)?.label ?? "Home";
  const assistantProps = {
    sessionId,
    history,
    live,
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
    mode: config?.mode ?? "single_advertiser",
  };
  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <aside className="hidden w-56 shrink-0 flex-col border-r border-border bg-muted/25 md:flex">
        <div className="flex h-14 items-center px-4">
          <Brand />
        </div>
        <nav className="space-y-1 px-2 py-4">
          {pages.map(({ id, label, icon: Icon }) => (
            <Button
              key={id}
              variant="ghost"
              className={cn(
                "w-full justify-start text-muted-foreground",
                page === id &&
                  "bg-background text-foreground shadow-sm ring-1 ring-border",
              )}
              onClick={() => setPage(id)}
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
            Capabilities
          </Button>
          <div className="mt-3 rounded-lg border border-border bg-background p-3">
            <div className="flex items-center gap-2 text-xs font-medium">
              <span className="size-1.5 rounded-full bg-emerald-500" />
              {config?.runtime === "j"
                ? "J-agent"
                : config?.runtime === "claude"
                  ? "Claude Agent SDK"
                : config?.runtime === "pi"
                  ? "Pi"
                  : "Agent"}{" "}
              + {modelLabel}
            </div>
            <p className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
              {selectedModel.auth_mode === "chatgpt_oauth"
                ? "ChatGPT OAuth"
                : "Direct HTTP API key"} · local runtime
            </p>
          </div>
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border px-4">
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            aria-label="Open navigation"
            onClick={() => setMobileNav(true)}
          >
            <Menu />
          </Button>
          <div className="text-sm text-muted-foreground">
            Workspace <span className="mx-1.5">/</span>
            <span className="font-medium text-foreground">{pageTitle}</span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Badge tone="muted">
              {config?.mode === "portfolio"
                ? `Portfolio · ${portfolio?.id ?? "sandbox"}`
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
        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto">
          <main className="mx-auto w-full max-w-6xl px-5 py-8 sm:px-8">
            {page === "home" && (
              <HomeView
                account={account}
                overview={overview}
                changes={changes}
                onAsk={ask}
                onNavigate={setPage}
              />
            )}
            {page === "portfolio" && (
              <PortfolioView
                portfolio={portfolio}
                report={portfolioReport}
                changes={changes}
                onAsk={ask}
                onNavigate={setPage}
              />
            )}
            {page === "campaigns" && (
              <div className="space-y-6">
                <PageHeading
                  eyebrow="Structure"
                  title="Campaign hierarchy"
                  description="Inspect one level at a time. Browse never changes delivery."
                  action={
                    <Button
                      variant="secondary"
                      onClick={() =>
                        ask(
                          "Audit the current campaign hierarchy for disabled parents, budget placement, and scope mismatches.",
                        )
                      }
                    >
                      <Bot />
                      Ask for an audit
                    </Button>
                  }
                />
                <Card className="overflow-hidden">
                  <div className="flex min-h-14 flex-wrap items-center gap-2 border-b border-border px-4">
                    <button
                      className="text-sm font-medium"
                      onClick={() => {
                        setLevel("campaign");
                        setPath([]);
                      }}
                    >
                      Campaigns
                    </button>
                    {path.map((entity, index) => (
                      <React.Fragment key={entity.id}>
                        <ChevronRight className="size-3.5 text-muted-foreground" />
                        <button
                          className="max-w-48 truncate text-sm text-muted-foreground hover:text-foreground"
                          onClick={() => {
                            const next = path.slice(0, index + 1);
                            setPath(next);
                            setLevel(
                              entity.level === "campaign" ? "ad_group" : "ad",
                            );
                          }}
                        >
                          {entity.name}
                        </button>
                      </React.Fragment>
                    ))}
                    <span className="ml-auto text-xs text-muted-foreground">
                      {entities.length} objects
                    </span>
                  </div>
                  <EntityTable
                    entities={entities}
                    onSelect={
                      level === "ad"
                        ? undefined
                        : (entity) => {
                            setPath((value) => [...value, entity]);
                            setLevel(level === "campaign" ? "ad_group" : "ad");
                          }
                    }
                  />
                </Card>
              </div>
            )}
            {page === "changes" && (
              <div className="space-y-6">
                <PageHeading
                  eyebrow="Human approval"
                  title="Proposed changes"
                  description="Review the exact before and after values. Each approval is single use."
                  action={
                    <Button
                      variant="secondary"
                      onClick={() => void loadChanges()}
                    >
                      <RefreshCw />
                      Refresh
                    </Button>
                  }
                />
                {changes.length ? (
                  <div className="grid gap-4 lg:grid-cols-2">
                    {changes.map((change) => (
                      <PresentationCard
                        key={change.id}
                        card={{ id: change.id, type: "change", change }}
                        onSuggest={ask}
                        onAction={requestChange}
                        busy={applying}
                      />
                    ))}
                  </div>
                ) : (
                  <Card className="border-dashed py-16 text-center">
                    <ShieldCheck className="mx-auto size-5" />
                    <CardTitle className="mt-4">No proposed changes</CardTitle>
                    <CardDescription className="mx-auto mt-2 max-w-sm">
                      Ask the assistant for a specific adjustment. It will read
                      the target, stage a draft, and return the preview here.
                    </CardDescription>
                  </Card>
                )}
              </div>
            )}
          </main>
        </div>
      </div>
      {assistantOpen && (
        <aside className="hidden w-[420px] shrink-0 border-l border-border lg:block">
          <AssistantPanel {...assistantProps} />
        </aside>
      )}
      <Dialog open={mobileNav} onOpenChange={setMobileNav}>
        <DialogContent className="top-4 translate-y-0">
          <DialogTitle>Workspace navigation</DialogTitle>
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
                  setPage(id);
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
            Capabilities
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
      <Dialog open={capabilitiesOpen} onOpenChange={setCapabilitiesOpen}>
        <DialogContent>
          <DialogTitle>Installed capabilities</DialogTitle>
          <DialogDescription>
            The host projects the same tools, grounding rules, skills, and
            safety behavior into every runtime.
          </DialogDescription>
          <div className="mt-5 rounded-lg border border-border p-3">
            <label htmlFor="model-selection" className="text-sm font-medium">
              Model
            </label>
            <select
              id="model-selection"
              value={modelSelectionKey(selectedModel)}
              disabled={busy}
              onChange={(event) => selectModel(event.target.value)}
              className="mt-2 h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-offset-background focus:ring-2 focus:ring-ring"
            >
              {config?.model.options.map((option) => (
                <option
                  key={modelSelectionKey(option)}
                  value={modelSelectionKey(option)}
                >
                  {option.label}
                </option>
              ))}
            </select>
            <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
              Provider: {selectedModel.provider}. Connection: {selectedModel.auth_mode === "chatgpt_oauth" ? "ChatGPT OAuth" : selectedModel.api}. Changing the model starts a new session so provider checkpoints cannot be mixed.
            </p>
          </div>
          <div className="mt-3 space-y-3">
            {config?.harness.capabilities.map((capability) => (
              <div
                key={capability.name}
                className="rounded-lg border border-border p-3"
              >
                <div className="text-sm font-medium">{capability.name}</div>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  {capability.description}
                </p>
              </div>
            ))}
          </div>
          <Separator className="my-5" />
          <p className="text-xs leading-relaxed text-muted-foreground">
            Grounding, staging follow-through, close-on-presentation, partial
            presentation, independent read dispatch, and post-turn memory
            extraction are enforced by the Go host. The memory write filter
            rejects advertising identifiers, credentials, and transient
            performance. Backend write execution is{" "}
            {config?.writes ? "available behind approval" : "disabled"}.
          </p>
        </DialogContent>
      </Dialog>
      <Dialog open={inspectorOpen} onOpenChange={setInspectorOpen}>
        <DialogContent className="right-0 left-auto top-0 h-[100dvh] w-full max-w-md translate-x-0 translate-y-0 rounded-none p-0">
          <div className="border-b border-border px-5 py-4">
            <DialogTitle>
              {config?.mode === "portfolio" ? "Activity" : "Activity and memory"}
            </DialogTitle>
            <DialogDescription>
              Public execution facts for the latest reply. Private reasoning and
              provider state are never shown.
            </DialogDescription>
          </div>
          <ScrollArea className="h-[calc(100dvh-110px)]">
            <div className="space-y-8 p-5">
              <section>
                <h3 className="text-xs font-semibold uppercase tracking-[.12em] text-muted-foreground">
                  Latest reply steps
                </h3>
                <div className="mt-3 divide-y divide-border rounded-lg border border-border">
                  {live.tools.length ? (
                    live.tools.map((tool) => (
                      <div
                        key={tool.id}
                        className="flex items-center justify-between gap-3 px-3 py-2.5 text-sm"
                      >
                        <span className="min-w-0 truncate">
                          {toolNames[tool.name] ?? tool.name}
                          {tool.role === "analysis" ? " · analyst" : ""}
                        </span>
                        <Badge tone={tool.ok === false ? "warning" : "muted"}>
                          {tool.ok === undefined
                            ? "Running"
                            : tool.ok
                              ? "Done"
                              : "Blocked"}
                        </Badge>
                      </div>
                    ))
                  ) : (
                    <p className="px-3 py-6 text-center text-xs text-muted-foreground">
                      No tool calls in the latest reply.
                    </p>
                  )}
                </div>
              </section>
              {config?.mode === "single_advertiser" && <section>
                <h3 className="text-xs font-semibold uppercase tracking-[.12em] text-muted-foreground">
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
                            <code className="truncate text-[10px] text-muted-foreground">
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
              </section>}
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
            The host will reread the object, rerun guardrails, send at most
            once, and wait for read-back. An unknown outcome will not be retried
            automatically.
          </DialogDescription>
          {confirm && (
            <div className="mt-5 rounded-lg bg-muted p-4">
              <div className="text-sm font-medium">{confirm.before?.name ?? confirm.create?.name ?? "New entity"}</div>
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
  useEffect(() => {
    void api<{ csrf: string }>("/auth")
      .then((value) => {
        setCSRF(value.csrf);
        setReady(true);
      })
      .catch(() => setReady(false))
      .finally(() => setChecking(false));
  }, []);
  if (checking)
    return (
      <main className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
        Connecting to the local workspace…
      </main>
    );
  return ready ? <Workspace /> : <Login onReady={() => setReady(true)} />;
}
