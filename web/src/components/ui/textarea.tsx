import * as React from "react";
import { cn } from "../../lib/utils";

export function Textarea({
  className,
  ...props
}: React.ComponentProps<"textarea">) {
  return (
    <textarea
      className={cn(
        "min-h-20 w-full resize-none rounded-md border-0 bg-transparent px-3 py-2 text-sm leading-relaxed outline-none placeholder:text-muted-foreground",
        className,
      )}
      {...props}
    />
  );
}
