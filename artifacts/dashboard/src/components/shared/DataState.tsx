import { type ReactNode } from "react";
import { AlertCircle, Inbox } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription, EmptyContent } from "@/components/ui/empty";
import { cn } from "@/lib/utils";

/**
 * The one loading/empty/error wrapper for the whole app. Previously: ~13
 * bespoke empty-state strings, 4 different loading idioms (spinner,
 * RefreshCw, framer bars, pulsing dots), and only 1 of 10 pages handled
 * `isError` at all — every other page showed a spinner or empty state
 * forever on a real failure. This is the single biggest structural fix in
 * the redesign.
 *
 * Copy is calm and sentence-case (Craft Principle #8) — no "NOTHING HERE
 * YET" / all-caps / exclamation points.
 */
export function DataState({
  isLoading,
  isError,
  isEmpty,
  onRetry,
  loadingSkeleton,
  skeletonRows = 4,
  emptyTitle = "Nothing here yet",
  emptyDescription = "Data will appear once the pipeline picks up new items.",
  errorTitle = "Couldn't load this",
  errorDescription = "Something went wrong reaching the server.",
  children,
}: {
  isLoading: boolean;
  isError?: boolean;
  isEmpty?: boolean;
  onRetry?: () => void;
  /** Custom skeleton layout (e.g. a grid of tiles). Falls back to stacked rows. */
  loadingSkeleton?: ReactNode;
  skeletonRows?: number;
  emptyTitle?: string;
  emptyDescription?: string;
  errorTitle?: string;
  errorDescription?: string;
  children: ReactNode;
}) {
  if (isLoading) {
    if (loadingSkeleton) return <>{loadingSkeleton}</>;
    return (
      <div className="flex flex-col gap-3 p-6">
        {Array.from({ length: skeletonRows }).map((_, i) => (
          <Skeleton key={i} className="h-16 w-full shimmer" />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <Empty className={cn("border-0")}>
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <AlertCircle />
          </EmptyMedia>
          <EmptyTitle>{errorTitle}</EmptyTitle>
          <EmptyDescription>{errorDescription}</EmptyDescription>
        </EmptyHeader>
        {onRetry && (
          <EmptyContent>
            <Button variant="outline" size="sm" onClick={onRetry}>Retry</Button>
          </EmptyContent>
        )}
      </Empty>
    );
  }

  if (isEmpty) {
    return (
      <Empty className="border-0">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Inbox />
          </EmptyMedia>
          <EmptyTitle>{emptyTitle}</EmptyTitle>
          <EmptyDescription>{emptyDescription}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    );
  }

  return <>{children}</>;
}
