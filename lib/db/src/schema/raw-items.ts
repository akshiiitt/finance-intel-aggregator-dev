import { pgTable, text, serial, timestamp, boolean } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

export const rawItemsTable = pgTable("raw_items", {
  id: serial("id").primaryKey(),
  source: text("source").notNull(),
  sourceType: text("source_type").notNull().default("rss"),
  url: text("url").unique(),
  title: text("title").notNull(),
  snippet: text("snippet"),
  author: text("author"),
  publishedAt: timestamp("published_at", { withTimezone: true }),
  fetchedAt: timestamp("fetched_at", { withTimezone: true }).notNull().defaultNow(),
  contentHash: text("content_hash").unique(),
  processed: boolean("processed").notNull().default(false),
  processingError: text("processing_error"),
});

export const insertRawItemSchema = createInsertSchema(rawItemsTable).omit({
  id: true,
  fetchedAt: true,
});
export type InsertRawItem = z.infer<typeof insertRawItemSchema>;
export type RawItem = typeof rawItemsTable.$inferSelect;
