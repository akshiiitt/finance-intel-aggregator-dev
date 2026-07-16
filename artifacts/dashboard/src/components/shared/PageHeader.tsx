import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * One page-header shape for the whole app — replaces the inline
 * `<h1 className="font-mono text-sm ...">` repeated (with slightly
 * different styling each time) at the top of every page.
 *
 * At most one status pill (the "eyebrow" per Craft Principle #8) and icons
 * are used sparingly — never decorative next to the title itself.
 */
export function PageHeader({
  title,
  description,
  live,
  actions,
  className,
}: {
  title: string;
  description?: string;
  /** Small "Live" indicator — the one legitimate ambient pulse per page. */
  live?: boolean;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start justify-between gap-4 px-6 py-5", className)}>
      <div className="min-w-0">
        <div className="flex items-center gap-2.5">
          <h1 className="text-xl font-semibold tracking-tight text-foreground truncate">{title}</h1>
          {live && (
            <span className="flex items-center gap-1.5 shrink-0">
              <span className="w-1.5 h-1.5 rounded-full bg-bullish animate-live" />
              <span className="text-2xs font-medium tracking-wide text-muted-foreground uppercase">Live</span>
            </span>
          )}
        </div>
        {description && (
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  );
}
