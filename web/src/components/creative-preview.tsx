import { ImageIcon, Play } from "lucide-react";
import type { AdDetail } from "../types";
import { cn } from "../lib/utils";

export function CreativePreview({
  detail,
  compact = false,
  className,
}: {
  detail: AdDetail;
  compact?: boolean;
  className?: string;
}) {
  const media = detail.media;
  const label = detail.creative?.name ?? detail.ad.name;
  if (!media)
    return (
      <div
        className={cn(
          "flex items-center justify-center overflow-hidden rounded-lg bg-muted text-muted-foreground",
          compact ? "size-12" : "aspect-[4/5] w-full",
          className,
        )}
      >
        <ImageIcon className={compact ? "size-4" : "size-6"} />
      </div>
    );

  if (compact)
    return (
      <div
        className={cn(
          "relative size-12 shrink-0 overflow-hidden rounded-md bg-muted",
          className,
        )}
      >
        <img
          src={media.poster_url ?? media.preview_url}
          alt=""
          loading="lazy"
          className="size-full object-cover"
        />
        {media.kind === "video" && (
          <span className="absolute inset-0 flex items-center justify-center bg-black/15 text-white">
            <Play className="size-3.5 fill-current" />
          </span>
        )}
      </div>
    );

  return (
    <figure className={cn("min-w-0", className)}>
      <div className="relative aspect-[4/5] overflow-hidden rounded-xl bg-muted">
        {media.kind === "video" ? (
          <video
            controls
            playsInline
            preload="metadata"
            poster={media.poster_url}
            aria-label={`${label} creative preview`}
            className="size-full object-cover"
          >
            <source src={media.preview_url} type="video/mp4" />
          </video>
        ) : (
          <img
            src={media.preview_url}
            alt={`${label} creative preview`}
            className="size-full object-cover"
          />
        )}
      </div>
      <figcaption className="mt-2 text-xs leading-relaxed text-muted-foreground">
        {media.source_url ? (
          <a
            href={media.source_url}
            target="_blank"
            rel="noreferrer"
            className="underline decoration-border underline-offset-2 hover:text-foreground"
          >
            {media.attribution ?? "Media source"}
          </a>
        ) : (
          (media.attribution ?? "Creative preview")
        )}
        {media.preview_url.startsWith("/sandbox/creatives/") && (
          <span className="block">Illustrative stock media</span>
        )}
      </figcaption>
    </figure>
  );
}
