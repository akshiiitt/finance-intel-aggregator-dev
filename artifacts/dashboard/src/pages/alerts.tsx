import { useState, useEffect } from "react";
import { useSearch, useLocation } from "wouter";
import { formatDistanceToNow } from "date-fns";
import {
  useGetAlerts, getGetAlertsQueryKey,
  useGetAlertTriggers, getGetAlertTriggersQueryKey,
  useCreateAlert, useDeleteAlert,
} from "@workspace/api-client-react";
import type { AlertRule, AlertTriggerEntry } from "@workspace/api-client-react";
import { useQueryClient } from "@tanstack/react-query";
import { Bell, Plus, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/shared/PageHeader";
import { DataState } from "@/components/shared/DataState";
import { TokenBadge } from "@/components/shared/Badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import {
  AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription,
  AlertDialogFooter, AlertDialogAction, AlertDialogCancel,
} from "@/components/ui/alert-dialog";
import { toast } from "@/hooks/use-toast";
import { CATEGORIES, type CategoryId } from "@/lib/categories";
import { ALERTS_MS } from "@/lib/query-config";

/** conditions is a loose {[key: string]: unknown} on the wire — this page
 * writes it as {keywords: string[], categories: string[], minFiScore: number}
 * via CreateAlertDialog, but nothing guarantees every row in the database
 * was written by this exact UI (a future admin tool, a manual DB edit, or a
 * schema change could leave a shape this page doesn't expect). A bare type
 * assertion here would let `keywords.map(...)` below throw if `keywords`
 * ever turns out not to be an array — with only one ErrorBoundary for the
 * whole app, that would take down the entire Alerts page, not just one
 * card. Validate each field's actual runtime type instead of trusting it. */
interface Conditions {
  keywords: string[];
  categories: string[];
  minFiScore?: number;
}
function conditionsOf(alert: AlertRule): Conditions {
  const raw = (alert.conditions ?? {}) as Record<string, unknown>;
  const keywords = Array.isArray(raw.keywords) ? raw.keywords.filter((k): k is string => typeof k === "string") : [];
  const categories = Array.isArray(raw.categories) ? raw.categories.filter((c): c is string => typeof c === "string") : [];
  const minFiScore = typeof raw.minFiScore === "number" ? raw.minFiScore : undefined;
  return { keywords, categories, minFiScore };
}

function timeAgo(dt: string | null | undefined): string {
  if (!dt) return "Never";
  try { return formatDistanceToNow(new Date(dt), { addSuffix: true }); }
  catch { return "—"; }
}

const CATEGORY_IDS = Object.keys(CATEGORIES) as CategoryId[];

function CreateAlertDialog({
  open, onOpenChange, onCreated, initialKeyword,
}: { open: boolean; onOpenChange: (o: boolean) => void; onCreated: () => void; initialKeyword?: string }) {
  const [name, setName] = useState(initialKeyword ? `${initialKeyword} watch` : "");
  const [keywords, setKeywords] = useState(initialKeyword ?? "");
  const [cats, setCats] = useState<string[]>([]);
  const [minFi, setMinFi] = useState("");
  const createAlert = useCreateAlert();

  // Re-sync when a new keyword arrives from "Alert me" on an article card
  // while the dialog is already mounted (open toggles false->true again).
  useEffect(() => {
    if (open && initialKeyword) {
      setName(`${initialKeyword} watch`);
      setKeywords(initialKeyword);
    }
  }, [open, initialKeyword]);

  function reset() {
    setName(""); setKeywords(""); setCats([]); setMinFi("");
  }

  async function submit() {
    if (!name.trim()) {
      toast({ title: "Name is required", variant: "destructive" });
      return;
    }
    if (!keywords.trim() && cats.length === 0) {
      toast({ title: "Add keywords or select at least one category", variant: "destructive" });
      return;
    }
    try {
      await createAlert.mutateAsync({
        data: {
          name: name.trim(),
          type: "custom",
          conditions: {
            keywords: keywords ? keywords.split(",").map(k => k.trim()).filter(Boolean) : undefined,
            categories: cats.length > 0 ? cats : undefined,
            minFiScore: minFi ? Number(minFi) : undefined,
          },
        },
      });
      toast({ title: "Alert created", description: `"${name.trim()}" is now watching for matches.` });
      onCreated();
      onOpenChange(false);
      reset();
    } catch (e) {
      toast({ title: "Couldn't create the alert", description: (e as Error).message, variant: "destructive" });
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Create alert</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Label htmlFor="alert-name">Alert name</Label>
            <Input id="alert-name" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Zomato funding news" className="mt-1.5" />
          </div>
          <div>
            <Label htmlFor="alert-keywords">Keywords (comma-separated)</Label>
            <Input id="alert-keywords" value={keywords} onChange={e => setKeywords(e.target.value)} placeholder="e.g. Zepto, quick commerce" className="mt-1.5" />
          </div>
          <div>
            <Label>Categories (optional)</Label>
            <div className="flex flex-wrap gap-1.5 mt-1.5">
              {CATEGORY_IDS.map(cat => {
                const active = cats.includes(cat);
                const meta = CATEGORIES[cat];
                return (
                  <button
                    key={cat}
                    type="button"
                    onClick={() => setCats(p => p.includes(cat) ? p.filter(c => c !== cat) : [...p, cat])}
                    className="text-xs px-2.5 py-1 rounded-md border transition-colors"
                    style={{
                      color: active ? `hsl(var(--${meta.token}))` : "hsl(var(--muted-foreground))",
                      borderColor: active ? `hsl(var(--${meta.token}) / 0.35)` : "hsl(var(--border))",
                      background: active ? `hsl(var(--${meta.token}) / 0.1)` : "transparent",
                    }}
                  >
                    {meta.label}
                  </button>
                );
              })}
            </div>
          </div>
          <div>
            <Label htmlFor="alert-fi">Minimum FI score (0–100, optional)</Label>
            <Input id="alert-fi" type="number" min={0} max={100} value={minFi} onChange={e => setMinFi(e.target.value)} placeholder="e.g. 60" className="mt-1.5" />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={submit} disabled={createAlert.isPending}>
            {createAlert.isPending ? "Creating…" : "Create alert"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function AlertCard({ alert, onDelete }: { alert: AlertRule; onDelete: (id: number) => void }) {
  const { keywords = [], categories = [], minFiScore } = conditionsOf(alert);
  const [confirmOpen, setConfirmOpen] = useState(false);

  return (
    <div className="surface-1 p-4" style={{ opacity: alert.isActive ? 1 : 0.6 }}>
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-2 min-w-0">
          <Bell size={13} className={alert.isActive ? "text-primary/70" : "text-muted-foreground"} />
          <span className="font-medium text-sm truncate text-foreground/90">{alert.name}</span>
          <TokenBadge token={alert.isActive ? "bullish" : "muted-foreground"} label={alert.isActive ? "Active" : "Paused"} />
        </div>
        <button
          onClick={() => setConfirmOpen(true)}
          className="text-muted-foreground/50 hover:text-bearish transition-colors shrink-0 p-1"
        >
          <Trash2 size={13} />
        </button>
      </div>

      <div className="flex flex-wrap gap-1.5 mb-3">
        {keywords.map(kw => <TokenBadge key={kw} token="primary" label={`"${kw}"`} />)}
        {categories.map(cat => <TokenBadge key={cat} token="chart-5" label={cat} />)}
        {minFiScore != null && <TokenBadge token="bullish" label={`FI ≥ ${minFiScore}`} />}
      </div>

      <div className="flex items-center gap-3 text-2xs text-muted-foreground pt-2.5 border-t border-border">
        <span>Created {timeAgo(alert.createdAt)}</span>
        <span className="text-border">·</span>
        <span className="tnum">
          {alert.triggerCount > 0
            ? <>Fired <span className="font-semibold text-foreground/70">{alert.triggerCount}</span>× · last {timeAgo(alert.lastTriggered)}</>
            : "Never fired"}
        </span>
      </div>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete "{alert.name}"?</AlertDialogTitle>
            <AlertDialogDescription>
              This alert will stop watching for matches. This can't be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => onDelete(alert.id)}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function TriggerRow({ trigger, alertName }: { trigger: AlertTriggerEntry; alertName: string }) {
  const fiPct = (trigger.fiScore ?? 0) / 100;
  const scoreColor = fiPct >= 0.7 ? "text-primary" : fiPct >= 0.5 ? "text-bullish" : "text-muted-foreground";
  return (
    <div className="flex items-start gap-3 px-5 py-3 feed-row">
      {/* Static, not pulsing — this is a historical event, not a live one. */}
      <span className="w-1.5 h-1.5 rounded-full mt-1.5 shrink-0 bg-primary/60" />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-0.5">
          <span className="text-2xs font-medium text-primary/70">{alertName}</span>
          <span className="text-border">·</span>
          <span className="text-2xs text-muted-foreground">{timeAgo(trigger.triggeredAt)}</span>
        </div>
        <p className="text-sm leading-snug line-clamp-2 text-foreground/80">{trigger.title}</p>
      </div>
      {trigger.fiScore != null && (
        <span className={`tnum text-xs font-semibold shrink-0 pt-1 ${scoreColor}`}>{Math.round(trigger.fiScore)}</span>
      )}
    </div>
  );
}

export default function AlertsPage() {
  const [showCreate, setShowCreate] = useState(false);
  const [prefillKeyword, setPrefillKeyword] = useState<string | undefined>(undefined);
  const [tab, setTab] = useState<"rules" | "history">("rules");
  const qc = useQueryClient();
  const search = useSearch();
  const [, navigate] = useLocation();

  // "Alert me" on an article card lands here as /alerts?keyword=... or
  // /alerts?create=1 — open the create dialog pre-filled, then drop the
  // param so a refresh doesn't reopen it. The keyword is captured into
  // real state (not re-derived from `search` on every render): navigate()
  // below clears the URL in the same tick as setShowCreate/setPrefillKeyword,
  // and React 18 can batch all three into one re-render — if the keyword
  // were read fresh from `search` at render time, that render could already
  // see the cleared URL and the dialog would open empty.
  useEffect(() => {
    const params = new URLSearchParams(search);
    if (params.has("keyword") || params.has("create")) {
      setPrefillKeyword(params.get("keyword") ?? undefined);
      setShowCreate(true);
      navigate("/alerts", { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search]);

  const { data: ad, isLoading: al, isError: ae, refetch: ar } = useGetAlerts({ query: { queryKey: getGetAlertsQueryKey(), refetchInterval: ALERTS_MS } });
  const { data: td, isLoading: tl, isError: te, refetch: tr } = useGetAlertTriggers({ query: { queryKey: getGetAlertTriggersQueryKey(), refetchInterval: ALERTS_MS } });
  const deleteAlert = useDeleteAlert();

  const alerts = ad?.alerts ?? [];
  const triggers = td?.triggers ?? [];
  const activeCount = alerts.filter(a => a.isActive).length;
  const alertNameById = new Map(alerts.map(a => [a.id, a.name]));

  async function handleDelete(id: number) {
    try {
      await deleteAlert.mutateAsync({ id });
      void qc.invalidateQueries({ queryKey: getGetAlertsQueryKey() });
      toast({ title: "Alert deleted" });
    } catch (e) {
      toast({ title: "Couldn't delete the alert", description: (e as Error).message, variant: "destructive" });
    }
  }

  const TABS = [
    { id: "rules" as const, label: "Rules", count: alerts.length },
    { id: "history" as const, label: "History", count: triggers.length },
  ];

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="Alert center"
        description="Track keywords, companies, and categories in real time"
        actions={
          <>
            {activeCount > 0 && <TokenBadge token="bullish" label={`${activeCount} active`} />}
            <Button size="sm" onClick={() => setShowCreate(true)}>
              <Plus size={13} /> New alert
            </Button>
          </>
        }
      />

      <div className="flex px-5 shrink-0 border-b border-border">
        {TABS.map(t => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`flex items-center gap-1.5 px-3.5 py-2.5 text-xs font-medium border-b-2 -mb-px transition-colors ${
              tab === t.id ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {t.label}
            {t.count > 0 && <span className="text-2xs px-1.5 py-0.5 rounded-md bg-secondary text-muted-foreground">{t.count}</span>}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto">
        {tab === "rules" ? (
          <div className="p-5">
            <DataState
              isLoading={al}
              isError={ae}
              isEmpty={alerts.length === 0}
              onRetry={() => void ar()}
              emptyTitle="No alerts yet"
              emptyDescription="Create your first alert to get notified when stories match your criteria."
            >
              <div className="flex flex-col gap-2.5">
                {alerts.map(alert => <AlertCard key={alert.id} alert={alert} onDelete={handleDelete} />)}
              </div>
            </DataState>
          </div>
        ) : (
          <DataState
            isLoading={tl}
            isError={te}
            isEmpty={triggers.length === 0}
            onRetry={() => void tr()}
            emptyTitle="No triggers yet"
            emptyDescription="Triggers appear when articles match your alert rules."
          >
            {triggers.map(t => (
              <TriggerRow key={t.id} trigger={t} alertName={alertNameById.get(t.alertId) ?? "Alert"} />
            ))}
          </DataState>
        )}
      </div>

      <CreateAlertDialog
        open={showCreate}
        onOpenChange={setShowCreate}
        onCreated={() => void qc.invalidateQueries({ queryKey: getGetAlertsQueryKey() })}
        initialKeyword={prefillKeyword}
      />
    </div>
  );
}
