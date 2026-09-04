import {
  AlertTriangle,
  ArrowRight,
  BarChart3,
  CircleDollarSign,
  CircleGauge,
  Lightbulb,
  MessageSquareText,
  PauseCircle,
  ShieldCheck,
} from "lucide-react";
import type { Card as CardRecord, Change, Entity, Metrics } from "../types";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import { cn } from "../lib/utils";

export const format = (
  value: string | number | null | undefined,
  digits = 2,
) =>
  value == null
    ? "Unavailable"
    : new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(
        Number(value),
      );

const stateName: Record<string, string> = {
  staged: "Awaiting approval",
  applying: "Applying",
  applied: "Verified",
  discarded: "Dismissed",
  failed: "Failed",
  expired: "Expired",
  indeterminate: "Needs reconciliation",
};
export const stateText = (state: string) => stateName[state] ?? state;

export function MetricStrip({
  metrics,
  roas,
  currency = "USD",
  compact = false,
}: {
  metrics: Metrics;
  roas?: string | null;
  currency?: string;
  compact?: boolean;
}) {
  const values = [
    ["Spend", format(metrics.spend), currency],
    ["Purchase value", format(metrics.revenue), currency],
    ["ROAS", format(roas, 3), "ratio"],
    [
      "Clicks",
      format(metrics.clicks, 0),
      `${format(metrics.impressions, 0)} impressions`,
    ],
  ];
  return (
    <div
      aria-label="Performance metrics"
      className={cn(
        "grid divide-y divide-border sm:grid-cols-2 sm:divide-x sm:divide-y-0 xl:grid-cols-4",
        compact && "xl:grid-cols-2 xl:divide-y xl:divide-x-0",
      )}
    >
      {values.map(([label, value, meta]) => (
        <div key={label} className="px-4 py-3 first:pl-0 sm:first:pl-4">
          <div className="text-[11px] font-medium uppercase tracking-[.12em] text-muted-foreground">
            {label}
          </div>
          <div className="mt-1 text-xl font-semibold tracking-tight tabular-nums">
            {value}
          </div>
          <div className="mt-0.5 text-[11px] text-muted-foreground">{meta}</div>
        </div>
      ))}
    </div>
  );
}

export function EntityTable({
  entities,
  onSelect,
}: {
  entities: Entity[];
  onSelect?: (entity: Entity) => void;
}) {
  if (!entities.length)
    return (
      <div className="px-4 py-10 text-center text-sm text-muted-foreground">
        No matching objects. Empty data is not an API failure.
      </div>
    );
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[620px] text-left text-sm">
        <thead className="border-b border-border bg-muted/35 text-[11px] uppercase tracking-[.08em] text-muted-foreground">
          <tr>
            <th className="px-4 py-2.5 font-medium">Object</th>
            <th className="px-4 py-2.5 font-medium">Status</th>
            <th className="px-4 py-2.5 font-medium">Budget</th>
            <th className="px-4 py-2.5 font-medium">Objective</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {entities.map((entity) => (
            <tr key={entity.id} className="group hover:bg-muted/30">
              <td className="px-4 py-3">
                <button
                  disabled={!onSelect}
                  onClick={() => onSelect?.(entity)}
                  className="flex w-full items-center justify-between text-left disabled:cursor-default"
                >
                  <span>
                    <strong className="block font-medium">{entity.name}</strong>
                    <code className="mt-0.5 block text-[11px] text-muted-foreground">
                      {entity.id}
                    </code>
                  </span>
                  {onSelect && (
                    <ArrowRight className="size-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                  )}
                </button>
              </td>
              <td className="px-4 py-3">
                <Badge tone={entity.status === "ENABLE" ? "success" : "muted"}>
                  {entity.status === "ENABLE" ? "Enabled" : "Disabled"}
                </Badge>
              </td>
              <td className="px-4 py-3 tabular-nums">
                {entity.budget == null ? "—" : format(entity.budget)}
                <span className="ml-1 text-xs text-muted-foreground">
                  {entity.budget_mode === "BUDGET_MODE_DAY" ? "/ day" : "total"}
                </span>
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {entity.objective || "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ChangePreview({
  change,
  onAction,
  busy,
}: {
  change: Change;
  onAction: (change: Change, action: "apply" | "discard" | "reconcile") => void;
  busy?: boolean;
}) {
  const creating = change.kind === "create";
  const before = creating
    ? "Not created"
    : change.kind === "budget"
      ? `${format(change.before?.budget)} ${change.currency}`
      : change.before?.status;
  const after = creating
    ? change.created?.status ?? change.create?.status ?? "DISABLE"
    : change.kind === "budget"
      ? `${format(change.after?.budget)} ${change.currency}`
      : change.after?.status;
  return (
    <Card className="overflow-hidden">
      <CardHeader className="flex-row items-start justify-between">
        <div>
          <CardTitle>{change.before?.name ?? change.created?.name ?? change.create?.name ?? "New entity"}</CardTitle>
          <CardDescription className="mt-1">
            {creating
              ? `Create ${change.create?.level ?? "entity"}`
              : change.kind === "budget"
              ? "Budget change"
              : "Delivery status change"}{" "}
            · {change.source.environment}
          </CardDescription>
        </div>
        <Badge
          tone={
            change.state === "applied"
              ? "success"
              : change.state === "indeterminate"
                ? "warning"
                : "muted"
          }
        >
          {stateText(change.state)}
        </Badge>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 rounded-lg bg-muted/55 px-4 py-3 tabular-nums">
          <span className="text-muted-foreground line-through">{before}</span>
          <ArrowRight className="size-4 text-muted-foreground" />
          <strong className="text-right text-base">{after}</strong>
        </div>
        <p className="mt-3 text-xs leading-relaxed text-muted-foreground">
          {change.reason}
        </p>
        {change.spend_increasing && (
          <div className="mt-3 flex gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            This draft may increase spend. Confirm scope before applying.
          </div>
        )}
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {change.state === "staged" && (
            <>
              <Button
                size="sm"
                disabled={busy}
                onClick={() => onAction(change, "apply")}
              >
                <ShieldCheck />
                Approve
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={busy}
                onClick={() => onAction(change, "discard")}
              >
                Dismiss
              </Button>
              <span className="text-[11px] text-muted-foreground">
                Nothing applies until you approve.
              </span>
            </>
          )}
          {change.state === "indeterminate" && (
            <Button
              size="sm"
              variant="secondary"
              disabled={busy}
              onClick={() => onAction(change, "reconcile")}
            >
              <CircleGauge />
              Reconcile by reading
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

const digestIcon = {
  opportunity: Lightbulb,
  warning: AlertTriangle,
  delivery: PauseCircle,
  measurement: BarChart3,
  change: CircleDollarSign,
};

export function PresentationCard({
  card,
  onSuggest,
  onAction,
  busy,
}: {
  card: CardRecord;
  onSuggest: (text: string) => void;
  onAction: (change: Change, action: "apply" | "discard" | "reconcile") => void;
  busy?: boolean;
}) {
  if (card.pending)
    return (
      <Card className="animate-pulse p-4">
        <div className="h-3 w-24 rounded bg-muted" />
        <div className="mt-3 h-16 rounded-lg bg-muted/70" />
      </Card>
    );
  if (card.type === "suggestions")
    return (
      <div className="flex flex-wrap gap-2">
        {card.suggestions?.map((text) => (
          <Button
            key={text}
            variant="secondary"
            size="sm"
            className="h-auto min-h-8 whitespace-normal text-left"
            onClick={() => onSuggest(text)}
          >
            {text}
            <ArrowRight />
          </Button>
        ))}
      </div>
    );
  if (card.type === "digest" && card.digest)
    return (
      <Card className="overflow-hidden">
        <CardHeader>
          <CardTitle>{card.digest.title}</CardTitle>
        </CardHeader>
        <div className="mt-3 divide-y divide-border border-t border-border">
          {card.digest.items.map((item, index) => {
            const Icon = digestIcon[item.kind] ?? MessageSquareText;
            return (
              <div
                key={`${item.headline}-${index}`}
                className="flex gap-3 px-4 py-3"
              >
                <div className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md bg-muted">
                  <Icon className="size-3.5" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium leading-snug">
                    {item.headline}
                  </div>
                  {item.why && (
                    <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                      {item.why}
                    </p>
                  )}
                  {item.action && (
                    <Button
                      variant="link"
                      size="sm"
                      className="mt-1 text-xs"
                      onClick={() => onSuggest(item.action!)}
                    >
                      {item.action}
                      <ArrowRight />
                    </Button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </Card>
    );
  if (card.type === "change" && card.change)
    return (
      <ChangePreview change={card.change} onAction={onAction} busy={busy} />
    );
  if (card.type === "entities")
    return (
      <Card className="overflow-hidden">
        <CardHeader>
          <CardTitle>Account objects</CardTitle>
          {card.annotation && (
            <CardDescription>{card.annotation}</CardDescription>
          )}
        </CardHeader>
        <div className="mt-3 border-t border-border">
          <EntityTable entities={card.entities ?? []} />
        </div>
      </Card>
    );
  if (card.type === "metrics") {
    const comparison = card.comparison;
    if (comparison)
      return (
        <Card>
          <CardHeader>
            <CardTitle>Period comparison</CardTitle>
            <CardDescription>
              {comparison.previous_query.start_date} —{" "}
              {comparison.previous_query.end_date} →{" "}
              {comparison.current_query.start_date} —{" "}
              {comparison.current_query.end_date} · {comparison.timezone}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-baseline gap-2 rounded-lg bg-muted/55 px-4 py-3">
              <span className="text-sm text-muted-foreground">
                ROAS {format(comparison.previous_roas, 3)}
              </span>
              <ArrowRight className="size-4 text-muted-foreground" />
              <strong className="text-xl">
                {format(comparison.current_roas, 3)}
              </strong>
              <span className="text-xs text-muted-foreground">
                {format(comparison.delta_roas, 3)} pts
              </span>
            </div>
            <details className="mt-3 text-xs text-muted-foreground">
              <summary className="cursor-pointer font-medium text-foreground">
                Method and limitations
              </summary>
              <p className="mt-2 leading-relaxed">{comparison.method}</p>
              {[...new Set(comparison.limitations)].map((item) => (
                <p key={item} className="mt-1">
                  {item}
                </p>
              ))}
            </details>
          </CardContent>
        </Card>
      );
    const metrics = card.calculation?.totals ?? card.report?.totals;
    if (metrics)
      return (
        <Card>
          <CardHeader>
            <CardTitle>Performance snapshot</CardTitle>
            <CardDescription>
              {card.calculation?.query.start_date ??
                card.report?.query.start_date}{" "}
              —{" "}
              {card.calculation?.query.end_date ?? card.report?.query.end_date}{" "}
              · {card.calculation?.timezone ?? card.report?.timezone}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <MetricStrip
              metrics={metrics}
              roas={card.calculation?.roas}
              currency={card.calculation?.currency ?? card.report?.currency}
              compact
            />
          </CardContent>
        </Card>
      );
  }
  return (
    <Card className="border-dashed p-4 text-xs text-muted-foreground">
      This component could not be rendered safely. Record <code>{card.id}</code>
      .
    </Card>
  );
}
