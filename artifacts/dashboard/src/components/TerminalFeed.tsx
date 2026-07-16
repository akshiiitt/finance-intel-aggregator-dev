import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { getGetFeedQueryKey } from "@workspace/api-client-react";
import { wsUrl } from "@/lib/api-config";
import { categoryMeta } from "@/lib/categories";
import { PageHeader } from "@/components/shared/PageHeader";

interface LiveArticle {
  id: number;
  title: string;
  summary: string;
  fiScore: number;
  category: string;
  source: string;
  region: string;
  sentiment: string;
  aiModel: string;
  receivedAt: number;
}

const SENTIMENT_ICONS: Record<string, string> = { bullish: "↑", bearish: "↓", neutral: "—" };

const MODEL_LABELS: Record<string, string> = {
  "gemini-flash": "Gemini", "gemini-legacy": "Gemini", "groq-8b": "Groq 8B", "groq-70b": "Groq 70B", keyword: "KW",
};

function scoreTone(score: number): { color: string; border: string } {
  if (score >= 80) return { color: "hsl(var(--primary))", border: "hsl(var(--primary) / 0.4)" };
  if (score >= 60) return { color: "hsl(var(--bullish))", border: "hsl(var(--bullish) / 0.4)" };
  if (score >= 40) return { color: "hsl(var(--chart-3))", border: "hsl(var(--chart-3) / 0.4)" };
  return { color: "hsl(var(--muted-foreground))", border: "hsl(var(--border))" };
}

export function TerminalFeed() {
  const [queue, setQueue] = useState<LiveArticle[]>([]);
  const [connected, setConnected] = useState(false);
  const [eventCount, setEventCount] = useState(0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const queryClient = useQueryClient();
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let worker: Worker;

    // Use Vite's worker syntax
    worker = new Worker(new URL('./terminal-worker.ts', import.meta.url), { type: 'module' });
    
    worker.onmessage = (e: MessageEvent) => {
      const msg = e.data;
      if (msg.type === "CONNECTED") {
        setConnected(true);
      } else if (msg.type === "DISCONNECTED") {
        setConnected(false);
      } else if (msg.type === "ARTICLE") {
        const article = msg.data as Omit<LiveArticle, "receivedAt">;
        setQueue((prev) => [{ ...article, receivedAt: Date.now() }, ...prev].slice(0, 150));
        setEventCount((c) => c + 1);
      }
    };

    worker.postMessage({ type: "CONNECT", url: wsUrl("/api/ws/terminal") });

    return () => {
      worker.postMessage({ type: "DISCONNECT" });
      worker.terminate();
    };
  }, [queryClient]);

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <PageHeader
        title="Live"
        description="Real-time article ingestion stream"
        live={connected}
        actions={
          <div className="tnum flex items-center gap-3 text-2xs text-muted-foreground">
            <span>{eventCount.toLocaleString()} events</span>
            <span className="text-border">·</span>
            <span>{queue.length} buffered</span>
          </div>
        }
      />

      {/* Column headers — hidden on mobile, where only score+title+sentiment show */}
      <div className="hidden sm:flex items-center gap-3 px-5 py-1.5 border-b border-border text-2xs text-muted-foreground shrink-0">
        <span className="w-16">Score</span>
        <span className="w-24">Category</span>
        <span className="w-16 hidden md:inline">Region</span>
        <span className="w-24 hidden lg:inline">Source</span>
        <span className="flex-1">Title</span>
        <span className="w-16 text-right hidden md:inline">Model</span>
      </div>

      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {queue.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full gap-2 text-center px-8">
            <p className="text-sm text-muted-foreground">
              {connected ? "Waiting for ingestion events…" : "Connecting to the pipeline…"}
            </p>
            <p className="text-2xs text-muted-foreground/70">Articles appear here as they're processed by the AI worker.</p>
          </div>
        ) : (
          <div className="divide-y divide-border/50">
            {queue.map((item, idx) => {
              const tone = scoreTone(item.fiScore);
              const meta = categoryMeta(item.category);
              return (
                <div key={`${item.id}-${idx}`} className="flex items-start gap-3 px-5 py-2.5 feed-row">
                  <div
                    className="tnum shrink-0 w-16 flex items-center justify-center h-5 rounded-md border text-2xs font-semibold"
                    style={{ color: tone.color, borderColor: tone.border }}
                  >
                    FI {Number(item.fiScore || 0).toFixed(0)}
                  </div>

                  <span className="shrink-0 w-24 text-2xs font-medium truncate" style={{ color: `hsl(var(--${meta.token}))` }}>
                    {meta.label}
                  </span>

                  <span className="shrink-0 w-16 text-2xs text-muted-foreground uppercase hidden md:inline">{item.region}</span>
                  <span className="shrink-0 w-24 text-2xs text-muted-foreground truncate hidden lg:inline">{item.source}</span>

                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-foreground/85 leading-snug truncate">
                      <span className="mr-1.5 tnum" style={{ color: item.sentiment === "bullish" ? "hsl(var(--bullish))" : item.sentiment === "bearish" ? "hsl(var(--bearish))" : "hsl(var(--muted-foreground))" }}>
                        {SENTIMENT_ICONS[item.sentiment]}
                      </span>
                      {item.title}
                    </p>
                    {item.summary && <p className="text-2xs text-muted-foreground mt-0.5 line-clamp-1">{item.summary}</p>}
                  </div>

                  <span className="shrink-0 w-16 text-right text-2xs text-muted-foreground hidden md:inline">
                    {MODEL_LABELS[item.aiModel] ?? item.aiModel}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
