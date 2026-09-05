import type { Change } from "../types";

export function operationTarget(change: Change) {
  const lines = change.operation?.lines ?? [];
  return (
    lines.find((line) => line.field.endsWith(".campaign.name"))?.after ??
    lines.find((line) => /\.(ad_id|ad_group_id|comment_id)$/.test(line.field))
      ?.after ??
    lines.find((line) => line.field.endsWith(".name"))?.after ??
    "Advertising operation"
  );
}

export function OperationReview({ change }: { change: Change }) {
  return (
    <div
      className="max-h-96 overflow-auto rounded-lg border border-border"
      aria-label="Exact operation changes"
    >
      <dl className="divide-y divide-border">
        {change.operation?.lines.map((line, index) => (
          <div
            key={`${line.field}-${index}`}
            className="space-y-1 px-3 py-2.5 text-xs"
          >
            <dt className="font-medium text-muted-foreground break-words">
              {line.field.replaceAll("_", " ")}
            </dt>
            <dd className="space-y-1 break-words [overflow-wrap:anywhere]">
              {line.before && line.before !== line.after && (
                <div className="text-muted-foreground line-through">
                  {line.before}
                </div>
              )}
              <div className="font-medium whitespace-pre-wrap">
                {line.after}
              </div>
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
