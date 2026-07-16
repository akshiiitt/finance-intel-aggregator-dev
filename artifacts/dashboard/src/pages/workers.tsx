import { formatDistanceToNow } from "date-fns";
import {
  useGetWorkerStatus, getGetWorkerStatusQueryKey,
  useTriggerFetch,
} from "@workspace/api-client-react";
import type { WorkerStatusWorkersItem } from "@workspace/api-client-react";
import { useQueryClient } from "@tanstack/react-query";
import { Activity, CheckCircle, AlertCircle, RefreshCw, Zap, Clock, Database, Brain } from "lucide-react";
import { PageHeader } from "@/components/shared/PageHeader";
import { MetricTile } from "@/components/shared/MetricTile";
import { DataState } from "@/components/shared/DataState";
import { DataTable, type DataTableColumn } from "@/components/shared/DataTable";
import { Button } from "@/components/ui/button";
import { toast } from "@/hooks/use-toast";
import { WORKER_STATUS_MS } from "@/lib/query-config";

function StatusDot({ status }: { status: string }) {
  // Color alone carries the state (idle/running/error) — no motion needed;
  // an animated ping on every running row was the kind of ambient
  // decoration the rest of the app deliberately avoids.
  const color =
    status === "running" ? "hsl(var(--primary))" :
    status === "error" ? "hsl(var(--bearish))" :
    "hsl(var(--bullish) / 0.6)";
  return <span className="inline-flex rounded-full w-2 h-2 shrink-0" style={{ background: color }} />;
}

function QuotaBar({ used, limit, label }: { used: number; limit: number; label: string }) {
  const pct = limit > 0 ? Math.min((used / limit) * 100, 100) : 0;
  const high = pct > 80;
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-2xs">
        <span className="text-muted-foreground">{label}</span>
        <span className={`tnum ${high ? "text-bearish" : "text-muted-foreground"}`}>
          {used.toLocaleString()} / {limit.toLocaleString()}
        </span>
      </div>
      <div className="h-[3px] rounded-full bg-secondary overflow-hidden">
        <div className="h-full rounded-full transition-all" style={{ width: `${pct}%`, background: high ? "hsl(var(--bearish))" : "hsl(var(--primary))" }} />
      </div>
    </div>
  );
}

export default function Workers() {
  const qc = useQueryClient();

  const { data, isLoading, isError, refetch } = useGetWorkerStatus({
    query: { queryKey: getGetWorkerStatusQueryKey(), refetchInterval: WORKER_STATUS_MS },
  });
  const triggerFetch = useTriggerFetch({
    mutation: {
      onSuccess: () => {
        toast({ title: "Fetch triggered", description: "Workers started processing new articles." });
        void qc.invalidateQueries({ queryKey: getGetWorkerStatusQueryKey() });
      },
      onError: (e) => toast({ title: "Couldn't trigger fetch", description: (e as Error).message, variant: "destructive" }),
    },
  });

  const workers = data?.workers ?? [];
  const lastRun = data?.lastRun ? new Date(data.lastRun) : null;
  const nextRun = data?.nextRun ? new Date(data.nextRun) : null;
  const quota = data?.quota;
  const aiBreak = data?.aiBreakdown;

  const idle = workers.filter(w => w.status === "idle").length;
  const running = workers.filter(w => w.status === "running").length;
  const errored = workers.filter(w => w.status === "error").length;
  const total = workers.reduce((s, w) => s + (w.itemsProcessed ?? 0), 0);

  const columns: DataTableColumn<WorkerStatusWorkersItem>[] = [
    {
      key: "name", header: "Source",
      render: w => (
        <div className="flex items-center gap-2.5">
          <StatusDot status={w.status ?? "idle"} />
          <span className="text-sm font-medium text-foreground/85 truncate">{w.name}</span>
        </div>
      ),
    },
    {
      key: "items", header: "Items", align: "right",
      render: w => (w.itemsProcessed ?? 0) > 0
        ? <span className="text-bullish font-semibold">{w.itemsProcessed}</span>
        : <span className="text-muted-foreground">0</span>,
    },
    {
      key: "lastRun", header: "Last run", align: "right",
      render: w => <span className="text-muted-foreground">{w.lastRun ? formatDistanceToNow(new Date(w.lastRun), { addSuffix: true }) : "Never"}</span>,
    },
    {
      key: "status", header: "Status", align: "right",
      render: w => (
        <span className={`font-semibold capitalize ${w.status === "idle" ? "text-bullish" : w.status === "running" ? "text-primary" : "text-bearish"}`}>
          {w.status ?? "idle"}
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="Worker control"
        description={`${workers.length} active sources · real-time ingestion pipeline`}
        actions={
          <>
            <Button variant="outline" size="icon" onClick={() => void qc.invalidateQueries({ queryKey: getGetWorkerStatusQueryKey() })}>
              <RefreshCw size={14} />
            </Button>
            <Button size="sm" onClick={() => triggerFetch.mutate({ data: {} })} disabled={triggerFetch.isPending}>
              {triggerFetch.isPending ? <RefreshCw size={13} className="animate-spin" /> : <Zap size={13} />}
              {triggerFetch.isPending ? "Fetching…" : "Fetch now"}
            </Button>
          </>
        }
      />

      <div className="flex-1 overflow-y-auto">
        <div className="max-w-[1000px] mx-auto px-5 pb-6">
          <DataState isLoading={isLoading} isError={isError} onRetry={() => void refetch()} errorTitle="Couldn't load worker status">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-5">
              <MetricTile label="Sources" value={workers.length || "—"} icon={<Database size={14} />} hero />
              <MetricTile label="Idle" value={idle} icon={<CheckCircle size={14} className="text-bullish" />} />
              <MetricTile label="Running" value={running} icon={<RefreshCw size={14} className="text-primary" />} />
              <MetricTile label="Errors" value={errored} icon={<AlertCircle size={14} className={errored > 0 ? "text-bearish" : ""} />} />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-5">
              <div className="surface-1 p-4">
                <div className="flex items-center gap-1.5 mb-4">
                  <Clock size={12} className="text-muted-foreground" />
                  <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">Schedule</span>
                </div>
                <div className="space-y-2.5">
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-muted-foreground">Last run</span>
                    <span className="tnum text-foreground/80">{lastRun ? formatDistanceToNow(lastRun, { addSuffix: true }) : "Not yet"}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-muted-foreground">Next run</span>
                    <span className="tnum text-foreground/80">{nextRun ? formatDistanceToNow(nextRun, { addSuffix: true }) : "Unknown"}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-muted-foreground">Total processed</span>
                    <span className="tnum font-semibold text-bullish">{total.toLocaleString()}</span>
                  </div>
                </div>
              </div>

              <div className="surface-1 p-4">
                <div className="flex items-center gap-1.5 mb-4">
                  <Brain size={12} className="text-muted-foreground" />
                  <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">AI quota today</span>
                </div>
                {quota ? (
                  <div className="space-y-3.5">
                    <QuotaBar used={quota.groq8bUsed ?? 0} limit={quota.groq8bLimit ?? 0} label="Groq 8B" />
                    <QuotaBar used={quota.groq70bUsed ?? 0} limit={quota.groq70bLimit ?? 0} label="Groq 70B" />
                    <QuotaBar used={quota.geminiUsed ?? 0} limit={quota.geminiLimit ?? 0} label="Gemini Flash" />
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">No AI keys configured — the free keyword pipeline still runs fully.</p>
                )}
              </div>
            </div>

            {aiBreak && Object.keys(aiBreak).length > 0 && (
              <div className="surface-1 p-4 mb-5">
                <div className="flex items-center gap-1.5 mb-3">
                  <Brain size={12} className="text-muted-foreground" />
                  <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">AI processing today</span>
                </div>
                <div className="flex flex-wrap gap-4 text-sm">
                  {Object.entries(aiBreak).map(([model, cnt]) => (
                    <span key={model}><span className="text-muted-foreground">{model}: </span><span className="tnum font-semibold text-foreground/85">{cnt}</span></span>
                  ))}
                </div>
              </div>
            )}

            <div>
              <div className="flex items-center gap-1.5 mb-3">
                <Activity size={12} className="text-muted-foreground" />
                <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">Feed workers</span>
              </div>
              {workers.length === 0 ? (
                <p className="text-sm text-muted-foreground">No worker data yet. Click "Fetch now" to initialize.</p>
              ) : (
                <DataTable columns={columns} data={workers} keyFn={w => w.name} />
              )}
            </div>
          </DataState>
        </div>
      </div>
    </div>
  );
}
