import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import {
  Radio, BookOpen, Zap, TrendingUp, DollarSign, BarChart2, Bell, Activity, Building2, Calendar,
} from "lucide-react";
import {
  Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem, CommandShortcut,
} from "@/components/ui/command";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { apiUrl } from "@/lib/api-config";
import type { Article } from "@workspace/api-client-react";
import { sanitizeUrl } from "@/lib/utils";

const NAV_ITEMS = [
  { href: "/", label: "Feed", icon: Radio },
  { href: "/digest", label: "Digest", icon: BookOpen },
  { href: "/terminal", label: "Live", icon: Zap },
  { href: "/market", label: "Markets", icon: TrendingUp },
  { href: "/funding", label: "Funding", icon: DollarSign },
  { href: "/ipo", label: "IPO", icon: Calendar },
  { href: "/analytics", label: "Analytics", icon: BarChart2 },
  { href: "/alerts", label: "Alerts", icon: Bell },
  { href: "/workers", label: "Workers", icon: Activity },
];

/**
 * Global command palette — replaces the dashboard's bespoke search modal
 * AND the mislabeled top-level "Search" nav entry (which pointed at
 * Entity, a drill-down view, not a real destination). This palette covers
 * quick navigation and article search (picking a result opens the source
 * article); Entity itself is reached by clicking a company/investor chip
 * on any ArticleCard, never as a peer nav item.
 *
 * CommandPaletteProvider owns the open state and renders the dialog once
 * (mounted by Layout) — any page can trigger it via `useCommandPalette()`
 * without prop-drilling, e.g. the Feed page's own "Search" button.
 */
const CommandPaletteContext = createContext<{ openPalette: () => void } | null>(null);

export function useCommandPalette() {
  const ctx = useContext(CommandPaletteContext);
  if (!ctx) throw new Error("useCommandPalette must be used within CommandPaletteProvider");
  return ctx;
}

export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(o => !o);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <CommandPaletteContext.Provider value={{ openPalette: () => setOpen(true) }}>
      {children}
      <CommandPalette open={open} onOpenChange={setOpen} />
    </CommandPaletteContext.Provider>
  );
}

function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [, navigate] = useLocation();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Article[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) { setQuery(""); setResults([]); }
  }, [open]);

  useEffect(() => {
    if (query.trim().length < 3) { setResults([]); return; }
    setLoading(true);
    const t = setTimeout(async () => {
      try {
        const res = await fetch(apiUrl(`/api/feed/search?q=${encodeURIComponent(query)}&limit=8`));
        const d = await res.json() as { items: Article[] };
        setResults(d.items ?? []);
      } catch {
        setResults([]);
      } finally {
        setLoading(false);
      }
    }, 250);
    return () => clearTimeout(t);
  }, [query]);

  function go(href: string) {
    onOpenChange(false);
    navigate(href);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="overflow-hidden p-0 gap-0 max-w-lg top-[20%] translate-y-0">
        <DialogTitle className="sr-only">Command palette</DialogTitle>
        <Command shouldFilter={false}>
          <CommandInput placeholder="Search articles, companies, or jump to a page…" value={query} onValueChange={setQuery} />
          <CommandList>
            {query.trim().length < 3 ? (
              <CommandGroup heading="Navigate">
                {NAV_ITEMS.map(({ href, label, icon: Icon }) => (
                  <CommandItem key={href} onSelect={() => go(href)}>
                    <Icon />
                    <span>{label}</span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : (
              <>
                {!loading && results.length === 0 && <CommandEmpty>No articles found.</CommandEmpty>}
                {results.length > 0 && (
                  <CommandGroup heading="Articles">
                    {results.map(article => (
                      <CommandItem
                        key={article.id}
                        onSelect={() => { onOpenChange(false); window.open(sanitizeUrl(article.sourceUrl), "_blank", "noopener,noreferrer"); }}
                      >
                        <Building2 />
                        <span className="truncate">{article.title}</span>
                        <CommandShortcut>{article.source}</CommandShortcut>
                      </CommandItem>
                    ))}
                  </CommandGroup>
                )}
              </>
            )}
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  );
}
