import * as React from "react";
import { cn } from "../../lib/utils";

export function Badge({
  className,
  tone = "default",
  ...props
}: React.ComponentProps<"span"> & {
  tone?: "default" | "muted" | "success" | "warning" | "danger";
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium",
        tone === "default" &&
          "border-foreground/15 bg-foreground text-background",
        tone === "muted" && "border-border bg-muted text-muted-foreground",
        tone === "success" &&
          "border-emerald-200 bg-emerald-50 text-emerald-800",
        tone === "warning" && "border-amber-200 bg-amber-50 text-amber-800",
        tone === "danger" && "border-red-200 bg-red-50 text-red-800",
        className,
      )}
      {...props}
    />
  );
}
