import React, { useState } from "react";
import { useLocation } from "wouter";
import { motion, AnimatePresence } from "framer-motion";
import { ExternalLink, ChevronDown, ChevronUp, Building2, Users, BellPlus } from "lucide-react";
import type { Article } from "@workspace/api-client-react";
import { categoryMeta } from "@/lib/categories";
import { formatAmount, formatValuation, timeAgo, parseKeyPoints, splitList } from "@/lib/format";
import { staggerDelay } from "@/lib/motion";
import { ScoreDial } from "@/components/shared/ScoreDial";
import { SentimentMark } from "@/components/shared/Badge";
import { cn, sanitizeUrl } from "@/lib/utils";

export const ArticleCard = React.memo(function ArticleCard({
  article,
  variant = "default",
  rank,
  index = 0,
}: {
  article: Article;
  variant?: "default" | "compact" | "deal";
  rank?: number;
  index?: number;
}) {
  const [expanded, setExpanded] = useState(false);
  const [, navigate] = useLocation();
  const meta = categoryMeta(article.category);
  const color = `hsl(var(--${meta.token}))`;
  const ago = timeAgo(article.publishedAt ?? article.fetchedAt);
  const amt = formatAmount(article.amount, article.currency);
  const valuation = variant === "deal" ? formatValuation(article.valuation, article.currency) : "";
  const companies = splitList(article.companies);
  const investors = splitList(article.investors);
  const keyPoints = variant === "default" ? parseKeyPoints(article.keyPoints) : [];
  const fiPct = (article.fiScore ?? 0) / 100;
  const hasSummary = !!(article.summary && article.summary !== article.title && article.summary.length > 20);

  return (
    <motion.article
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.24, delay: staggerDelay(index), ease: [0.16, 1, 0.3, 1] }}
      className={cn("feed-row group relative flex gap-3 px-5", variant === "compact" ? "py-2.5" : "py-3.5")}
    >
      <div
        className="absolute left-0 top-2.5 bottom-2.5 w-[2px] rounded-r transition-opacity"
        style={{ background: color, opacity: 0.25 + fiPct * 0.5 }}
      />

      {rank !== undefined && (
        <span className="tnum text-2xs text-muted-foreground/50 w-4 text-right shrink-0 pt-1.5 select-none">
          {rank}
        </span>
      )}

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1.5">
          <span className="flex items-center gap-1 shrink-0">
            <span className="w-[5px] h-[5px] rounded-full shrink-0" style={{ background: color }} />
            <span className="text-2xs font-medium tracking-wide" style={{ color }}>{meta.label}</span>
          </span>

          {article.roundType && (
            <span className="text-2xs text-muted-foreground">{article.roundType}</span>
          )}

          {amt && (
            <span className={cn("tnum font-semibold text-bullish", variant === "deal" ? "text-sm" : "text-xs")}>
              {amt}
            </span>
          )}

          {(article.coverageCount ?? 1) > 1 && (
            <span className="text-2xs text-muted-foreground" title={article.alsoSources ?? ""}>
              +{(article.coverageCount ?? 1) - 1} outlets
            </span>
          )}

          <span className="ml-auto flex items-center gap-1.5 shrink-0">
            <span className="text-2xs text-muted-foreground">{article.source}</span>
            {ago && <><span className="text-muted-foreground/30">·</span><span className="text-2xs text-muted-foreground">{ago}</span></>}
          </span>
        </div>

        <a
          href={sanitizeUrl(article.sourceUrl)}
          target="_blank"
          rel="noopener noreferrer"
          className={cn(
            "block leading-snug text-foreground/90 transition-colors hover:text-foreground line-clamp-2 mb-1 pr-2",
            variant === "compact" ? "text-sm" : "text-base",
          )}
        >
          {article.title}
        </a>

        {hasSummary && variant !== "compact" && (
          <p className="text-sm leading-relaxed line-clamp-2 mb-1.5 pr-2 text-muted-foreground">
            {article.summary}
          </p>
        )}

        {valuation && <div className="tnum text-2xs text-muted-foreground mb-1.5">{valuation}</div>}

        {keyPoints.length > 0 && (
          <div className="mb-1">
            <button
              onClick={() => setExpanded(e => !e)}
              className="flex items-center gap-1 text-2xs text-muted-foreground hover:text-foreground transition-colors"
            >
              {expanded ? <ChevronUp size={10} /> : <ChevronDown size={10} />}
              {expanded ? "Hide" : `${keyPoints.length} key points`}
            </button>
            <AnimatePresence>
              {expanded && (
                <motion.ul
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: "auto" }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.18 }}
                  className="mt-1.5 space-y-1 overflow-hidden"
                >
                  {keyPoints.map((pt, i) => (
                    <li key={i} className="flex items-start gap-2 text-sm leading-relaxed text-foreground/80">
                      <span className="shrink-0 mt-0.5 text-xs font-semibold text-primary">–</span>
                      {pt}
                    </li>
                  ))}
                </motion.ul>
              )}
            </AnimatePresence>
          </div>
        )}

        {(companies.length > 0 || investors.length > 0) && (
          <div className="flex flex-wrap gap-1.5 mt-1.5">
            {companies.slice(0, 3).map(c => (
              <button
                key={c}
                onClick={(e) => { e.stopPropagation(); navigate(`/entity?name=${encodeURIComponent(c)}`); }}
                title={`View ${c} in entity search`}
                className="flex items-center gap-1 text-2xs px-1.5 py-0.5 rounded-md border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors"
              >
                <Building2 size={9} className="opacity-60" />{c}
              </button>
            ))}
            {investors.slice(0, 2).map(inv => (
              <button
                key={inv}
                onClick={(e) => { e.stopPropagation(); navigate(`/entity?name=${encodeURIComponent(inv)}`); }}
                title={`View ${inv} in entity search`}
                className="flex items-center gap-1 text-2xs px-1.5 py-0.5 rounded-md border text-[hsl(var(--chart-2))] hover:brightness-125 transition-[filter]"
                style={{ borderColor: "hsl(var(--chart-2) / 0.25)", background: "hsl(var(--chart-2) / 0.08)" }}>
                <Users size={9} />{inv}
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex flex-col items-end gap-1.5 shrink-0 pt-0.5">
        {variant === "default" && <ScoreDial score={article.fiScore} size={28} />}
        {variant === "deal" && article.fiScore != null && (
          <span className="tnum text-xs font-semibold text-muted-foreground">{Math.round(article.fiScore)}</span>
        )}
        <SentimentMark sentiment={article.sentiment} />
        <div className="mt-auto flex items-center gap-1 opacity-40 group-hover:opacity-90 transition-opacity">
          <button
            onClick={(e) => {
              e.stopPropagation();
              const keyword = companies[0] ?? investors[0];
              navigate(keyword ? `/alerts?keyword=${encodeURIComponent(keyword)}` : "/alerts?create=1");
            }}
            className="text-muted-foreground hover:text-primary transition-colors"
            title="Create an alert for this"
          >
            <BellPlus size={13} />
          </button>
          <a href={sanitizeUrl(article.sourceUrl)} target="_blank" rel="noopener noreferrer" className="text-muted-foreground hover:text-foreground transition-colors">
            <ExternalLink size={13} />
          </a>
        </div>
      </div>
    </motion.article>
  );
});
