import {
  AlertTriangle,
  ArrowRight,
  CircleGauge,
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
import { OperationReview, operationTarget } from "./operation-review";

function MetricCardHeading({
  card,
  comparison = false,
}: {
  card: CardRecord;
  comparison?: boolean;
}) {
  const query =
    card.calculation?.query ??
    card.comparison?.current_query ??
    card.report?.query;
  const source =
    card.calculation?.source ?? card.comparison?.source ?? card.report?.source;
  const scope =
    card.metric_scope?.account_id === source?.account_id &&
    card.metric_scope?.level === query?.level &&
    (card.metric_scope?.entity_id ?? "") === (query?.entity_id ?? "")
      ? card.metric_scope
      : undefined;
  const level = query?.level;
  const labels: Record<string, string> = {
    advertiser: "Account",
    campaign: "Campaign",
    ad_group: "Ad group",
    ad: "Ad",
  };
  const totals: Record<string, string> = {
    campaign: "All campaigns",
    ad_group: "All ad groups",
    ad: "All ads",
  };
  // Never substitute the current page selection for a saved report's scope.
  const account = scope?.account_name || source?.account_id;
  const name =
    level === "advertiser"
      ? account
      : query?.entity_id
        ? scope?.entity_name || query.entity_id
        : totals[level ?? ""];
  return (
    <>
      <CardTitle className="break-words" title={query?.entity_id}>
        {name || "Scope unavailable"}
      </CardTitle>
      <CardDescription>
        {labels[level ?? ""] ?? "Report"} ·{" "}
        {comparison ? "Period comparison" : "Performance"}
        {level !== "advertiser" && account ? ` · ${account}` : ""}
      </CardDescription>
    </>
  );
}

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
    [
      "ROAS",
      format(
        roas === undefined &&
          metrics.revenue != null &&
          Number(metrics.spend) > 0
          ? Number(metrics.revenue) / Number(metrics.spend)
          : roas,
        3,
      ),
      metrics.revenue == null
        ? "Purchase value not reported"
        : Number(metrics.spend) === 0
          ? "No spend in this period"
          : "Reported value / spend",
    ],
    [
      "Clicks",
      format(metrics.clicks, 0),
      `${format(metrics.impressions, 0)} impressions`,
    ],
  ];
  return (
    <div
      aria-label="Performance metrics"
      className="metric-grid"
      data-compact={compact}
    >
      {values.map(([label, value, meta]) => (
        <div key={label} className="min-w-0 py-1">
          <div className="text-xs text-muted-foreground">{label}</div>
          <div
            className="metric-value mt-1"
            data-unavailable={value === "Unavailable"}
          >
            {value}
          </div>
          <div className="mt-0.5 text-xs text-muted-foreground">{meta}</div>
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
        No matching objects.
      </div>
    );
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[620px] text-left text-sm">
        <thead className="border-b border-border bg-muted/35 text-xs text-muted-foreground">
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
                    <code className="mt-0.5 block text-xs text-muted-foreground">
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
  const operating = change.kind === "operation";
  const before = creating
    ? "Not created"
    : change.kind === "budget"
      ? `${format(change.before?.budget)} ${change.currency}`
      : change.before?.status;
  const after = creating
    ? (change.created?.status ?? change.create?.status ?? "DISABLE")
    : change.kind === "budget"
      ? `${format(change.after?.budget)} ${change.currency}`
      : change.after?.status;
  return (
    <Card className="overflow-hidden">
      <CardHeader className="flex-row items-start justify-between">
        <div>
          <CardTitle>
            {change.before?.name ??
              change.created?.name ??
              change.create?.name ??
              change.operation?.lines[0]?.name ??
              operationTarget(change)}
          </CardTitle>
          <CardDescription className="mt-1">
            {operating
              ? titleCaseOperation(change.operation?.request.kind)
              : creating
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
        {operating ? (
          <OperationReview change={change} />
        ) : (
          <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 rounded-lg bg-muted/55 px-4 py-3 tabular-nums">
            <span className="text-muted-foreground line-through">{before}</span>
            <ArrowRight className="size-4 text-muted-foreground" />
            <strong className="text-right text-base">{after}</strong>
          </div>
        )}
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

function titleCaseOperation(value?: string) {
  return value
    ? value
        .split("_")
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ")
    : "Advertising operation";
}

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
            className="h-auto min-h-8 max-w-full whitespace-normal py-2 text-left"
            onClick={() => onSuggest(text)}
          >
            <span className="min-w-0 break-words">{text}</span>
            <ArrowRight className="shrink-0" />
          </Button>
        ))}
      </div>
    );
  if (card.type === "digest" && card.digest)
    return (
      <Card className="briefing-card overflow-hidden">
        <CardHeader>
          <CardTitle>{card.digest.title}</CardTitle>
        </CardHeader>
        <ul className="mt-3 list-none divide-y divide-border border-t border-border">
          {card.digest.items.map((item, index) => {
            // Saved tool-bound records, never the current page selection or model labels.
            const subject =
              item.entity ??
              item.change?.before ??
              item.change?.created ??
              item.change?.create;
            const subjectLabel = subject
              ? `${subject.level === "ad_group" ? "Ad group" : titleCaseOperation(subject.level)} · ${subject.name || ("id" in subject ? subject.id : "")}`
              : item.change
                ? operationTarget(item.change)
                : undefined;
            return (
              <li
                key={`${item.headline}-${index}`}
                className="briefing-item min-w-0 space-y-2 px-4 py-3 [overflow-wrap:anywhere]"
              >
                {subjectLabel && (
                  <p className="briefing-subject text-xs leading-relaxed text-muted-foreground">
                    {item.change ? `Change · ${subjectLabel}` : subjectLabel}
                  </p>
                )}
                <h4 className="briefing-finding text-sm font-semibold leading-snug">
                  {item.headline}
                </h4>
                {item.action && (
                  <div className="briefing-next-step space-y-0.5">
                    <p className="text-xs text-muted-foreground">Next step</p>
                    <p className="text-sm font-normal leading-relaxed">
                      {item.action}
                    </p>
                  </div>
                )}
                {item.why && (
                  <details className="text-xs text-muted-foreground">
                    <summary className="w-fit cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2">
                      Evidence
                    </summary>
                    <p className="mt-1 leading-relaxed">{item.why}</p>
                  </details>
                )}
              </li>
            );
          })}
        </ul>
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
            <MetricCardHeading card={card} comparison />
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
              <strong className="metric-value">
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
            <MetricCardHeading card={card} />
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
