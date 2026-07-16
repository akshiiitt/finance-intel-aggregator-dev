import { pgTable, text, serial, timestamp, numeric, integer, boolean, jsonb, customType } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

// pgvector's `vector(N)` type has no first-class Drizzle helper in the
// version pinned here, so it's defined as a customType. Values round-trip
// as plain number[] on the JS side; the driver sends/receives the
// "[0.1,0.2,...]" text literal pgvector expects on the wire.
const vector384 = customType<{ data: number[]; driverData: string }>({
  dataType() {
    return "vector(384)";
  },
  toDriver(value: number[]): string {
    return `[${value.join(",")}]`;
  },
  fromDriver(value: string): number[] {
    return value
      .slice(1, -1)
      .split(",")
      .filter((s) => s.length > 0)
      .map(Number);
  },
});

export const processedItemsTable = pgTable("processed_items", {
  id: serial("id").primaryKey(),
  rawItemId: integer("raw_item_id"),
  title: text("title").notNull(),
  summary: text("summary"),
  keyPoints: text("key_points"),
  sourceUrl: text("source_url").notNull(),
  source: text("source").notNull(),
  sourceType: text("source_type"),
  region: text("region"),
  category: text("category"),
  sentiment: text("sentiment"),
  sentimentScore: numeric("sentiment_score"),
  relevanceScore: numeric("relevance_score"),
  fiScore: numeric("fi_score"),
  // Real JSONB arrays as of migration 000004 — previously comma-separated
  // TEXT, which meant "every article mentioning Zepto" required a substring
  // scan and couldn't be indexed. Now backed by GIN indexes on the Go side.
  // The Go API still serializes these as a ", "-joined string on the wire
  // (via the jsonb_to_csv() SQL helper) — this type change only affects
  // what's queryable in Postgres, not the REST contract.
  companies: jsonb("companies").$type<string[]>().notNull().default([]),
  investors: jsonb("investors").$type<string[]>().notNull().default([]),
  amount: numeric("amount"),
  currency: text("currency"),
  roundType: text("round_type"),
  valuation: numeric("valuation"),
  coverageCount: integer("coverage_count").default(1),
  alsoSources: text("also_sources"),
  aiModelUsed: text("ai_model_used"),
  publishedAt: timestamp("published_at", { withTimezone: true }),
  fetchedAt: timestamp("fetched_at", { withTimezone: true }).notNull().defaultNow(),
  // Free semantic layer — populated by the Process worker for every article,
  // no paid AI involved. Used for vector-cosine dedup, semantic search, and
  // niche tagging.
  embedding: vector384("embedding"),
  niches: text("niches").array().notNull().default([]),
  // AI gating: ai_pending marks rows the free Process worker flagged as
  // worth a paid API call (deal-type, or scored high enough to matter).
  // ai_enriched marks rows the paid Enrich worker has actually finished.
  // A row can be ai_pending=false, ai_enriched=false forever — that's the
  // normal, expected state for most of the feed.
  aiPending: boolean("ai_pending").notNull().default(false),
  aiEnriched: boolean("ai_enriched").notNull().default(false),
});

export const insertProcessedItemSchema = createInsertSchema(processedItemsTable).omit({
  id: true,
  fetchedAt: true,
});
export type InsertProcessedItem = z.infer<typeof insertProcessedItemSchema>;
export type ProcessedItem = typeof processedItemsTable.$inferSelect;
