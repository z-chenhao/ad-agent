import { useId, useMemo, useState } from "react";
import type { Metrics, Report } from "../types";

type Point = {
  date: string;
  value: number;
  metrics: Metrics;
  x: number;
  y: number;
};

const WIDTH = 640;
const HEIGHT = 224;
const PADDING = { top: 16, right: 18, bottom: 34, left: 48 };

function metricROAS(metrics: Metrics) {
  const spend = Number(metrics.spend);
  const revenue = Number(metrics.revenue);
  return spend > 0 && Number.isFinite(revenue) ? revenue / spend : null;
}

function shortDate(value: string) {
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  }).format(new Date(`${value}T00:00:00Z`));
}

function money(value: string | null, currency: string) {
  if (value == null) return "Unavailable";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    maximumFractionDigits: 2,
  }).format(Number(value));
}

// Monotone cubic interpolation keeps the curve smooth without inventing peaks
// outside the observed daily values.
function monotonePath(points: Point[]) {
  if (points.length < 2) return "";
  const slopes = points.slice(0, -1).map((point, index) => {
    const next = points[index + 1]!;
    return (next.y - point.y) / (next.x - point.x);
  });
  const tangents = points.map((_, index) => {
    if (index === 0) return slopes[0]!;
    if (index === points.length - 1) return slopes.at(-1)!;
    const before = slopes[index - 1]!;
    const after = slopes[index]!;
    if (before === 0 || after === 0 || Math.sign(before) !== Math.sign(after))
      return 0;
    return (before + after) / 2;
  });
  slopes.forEach((slope, index) => {
    if (slope === 0) {
      tangents[index] = 0;
      tangents[index + 1] = 0;
      return;
    }
    const a = tangents[index]! / slope;
    const b = tangents[index + 1]! / slope;
    const magnitude = a * a + b * b;
    if (magnitude > 9) {
      const scale = 3 / Math.sqrt(magnitude);
      tangents[index] = scale * a * slope;
      tangents[index + 1] = scale * b * slope;
    }
  });
  return points.slice(1).reduce((path, point, index) => {
    const previous = points[index]!;
    const third = (point.x - previous.x) / 3;
    return `${path} C ${previous.x + third} ${previous.y + tangents[index]! * third}, ${point.x - third} ${point.y - tangents[index + 1]! * third}, ${point.x} ${point.y}`;
  }, `M ${points[0]!.x} ${points[0]!.y}`);
}

export function RoasTrendChart({
  rows,
  currency,
}: {
  rows: Report["rows"];
  currency: string;
}) {
  const gradientID = useId();
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const points = useMemo(() => {
    const values = [...rows]
      .sort((a, b) => a.date.localeCompare(b.date))
      .map((row) => ({ ...row, value: metricROAS(row.metrics) }))
      .filter(
        (row): row is typeof row & { value: number } => row.value != null,
      );
    if (values.length < 2) return [];
    const observedMin = Math.min(...values.map((row) => row.value));
    const observedMax = Math.max(...values.map((row) => row.value));
    const spread = Math.max(observedMax - observedMin, observedMax * 0.08, 0.1);
    const min = Math.max(0, observedMin - spread * 0.18);
    const max = observedMax + spread * 0.18;
    const plotWidth = WIDTH - PADDING.left - PADDING.right;
    const plotHeight = HEIGHT - PADDING.top - PADDING.bottom;
    return values.map<Point>((row, index) => ({
      date: row.date,
      value: row.value,
      metrics: row.metrics,
      x: PADDING.left + (index / (values.length - 1)) * plotWidth,
      y: PADDING.top + ((max - row.value) / (max - min)) * plotHeight,
    }));
  }, [rows]);
  if (points.length < 2)
    return (
      <div className="py-12 text-sm text-muted-foreground">
        Daily ROAS trend is unavailable.
      </div>
    );

  const values = points.map((point) => point.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const active = activeIndex == null ? null : (points[activeIndex] ?? null);
  const path = monotonePath(points);
  const areaPath = `${path} L ${points.at(-1)!.x} ${HEIGHT - PADDING.bottom} L ${points[0]!.x} ${HEIGHT - PADDING.bottom} Z`;
  const labels = [0, Math.floor((points.length - 1) / 2), points.length - 1];

  return (
    <div
      className="relative w-full touch-none"
      tabIndex={0}
      aria-label={`Daily ROAS chart from ${points[0]!.date} to ${points.at(-1)!.date}. Use pointer movement to inspect exact values.`}
      onFocus={() => setActiveIndex(points.length - 1)}
      onBlur={() => setActiveIndex(null)}
      onPointerLeave={() => setActiveIndex(null)}
      onPointerMove={(event) => {
        const bounds = event.currentTarget.getBoundingClientRect();
        const ratio = Math.max(
          0,
          Math.min(1, (event.clientX - bounds.left) / bounds.width),
        );
        setActiveIndex(Math.round(ratio * (points.length - 1)));
      }}
    >
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="h-56 w-full overflow-visible"
        role="img"
        aria-hidden="true"
      >
        <defs>
          <linearGradient id={gradientID} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0.12" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
          </linearGradient>
        </defs>
        {[0, 0.5, 1].map((ratio) => {
          const y =
            PADDING.top + ratio * (HEIGHT - PADDING.top - PADDING.bottom);
          const value = max - ratio * (max - min);
          return (
            <g key={ratio}>
              <line
                x1={PADDING.left}
                x2={WIDTH - PADDING.right}
                y1={y}
                y2={y}
                className="stroke-border"
                strokeDasharray="3 5"
              />
              <text
                x={PADDING.left - 8}
                y={y + 3}
                textAnchor="end"
                className="fill-muted-foreground text-[10px] tabular-nums"
              >
                {value.toFixed(2)}×
              </text>
            </g>
          );
        })}
        <path
          d={areaPath}
          fill={`url(#${gradientID})`}
          className="text-foreground"
        />
        <path
          d={path}
          fill="none"
          className="stroke-foreground"
          strokeWidth="2.25"
          strokeLinecap="round"
          strokeLinejoin="round"
          vectorEffect="non-scaling-stroke"
        />
        {labels.map((index) => {
          const point = points[index]!;
          return (
            <text
              key={point.date}
              x={point.x}
              y={HEIGHT - 9}
              textAnchor={
                index === 0
                  ? "start"
                  : index === points.length - 1
                    ? "end"
                    : "middle"
              }
              className="fill-muted-foreground text-[10px]"
            >
              {shortDate(point.date)}
            </text>
          );
        })}
        {active && (
          <g>
            <line
              x1={active.x}
              x2={active.x}
              y1={PADDING.top}
              y2={HEIGHT - PADDING.bottom}
              className="stroke-foreground/25"
              strokeDasharray="3 4"
            />
            <circle
              cx={active.x}
              cy={active.y}
              r="5"
              className="fill-background stroke-foreground"
              strokeWidth="2.5"
            />
          </g>
        )}
      </svg>
      {active && (
        <div
          role="status"
          className="pointer-events-none absolute top-2 min-w-36 rounded-lg border border-border bg-popover px-3 py-2 text-xs shadow-lg"
          style={{
            left: `${(active.x / WIDTH) * 100}%`,
            transform:
              activeIndex === 0
                ? "translateX(0)"
                : activeIndex === points.length - 1
                  ? "translateX(-100%)"
                  : "translateX(-50%)",
          }}
        >
          <div className="font-medium">{shortDate(active.date)}</div>
          <div className="mt-1 grid grid-cols-[auto_auto] gap-x-4 gap-y-0.5 text-muted-foreground">
            <span>ROAS</span>
            <strong className="text-right font-medium text-foreground tabular-nums">
              {active.value.toFixed(2)}×
            </strong>
            <span>Spend</span>
            <span className="text-right tabular-nums">
              {money(active.metrics.spend, currency)}
            </span>
            <span>Value</span>
            <span className="text-right tabular-nums">
              {money(active.metrics.revenue, currency)}
            </span>
          </div>
        </div>
      )}
      <span className="sr-only">
        {points
          .map((point) => `${point.date}: ${point.value.toFixed(2)} ROAS`)
          .join("; ")}
      </span>
    </div>
  );
}
