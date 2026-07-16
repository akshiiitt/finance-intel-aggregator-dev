import { pgTable, serial, date, text, integer, timestamp } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

// One row per day: the AI-generated morning briefing built from the day's
// top-scored stories. See internal/worker/digest on the Go backend.
export const dailyDigestsTable = pgTable("daily_digests", {
  id: serial("id").primaryKey(),
  digestDate: date("digest_date").notNull().unique(),
  content: text("content").notNull(),
  topStoryIds: integer("top_story_ids").array().notNull().default([]),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
});

export const insertDailyDigestSchema = createInsertSchema(dailyDigestsTable).omit({
  id: true,
  createdAt: true,
});
export type InsertDailyDigest = z.infer<typeof insertDailyDigestSchema>;
export type DailyDigest = typeof dailyDigestsTable.$inferSelect;
