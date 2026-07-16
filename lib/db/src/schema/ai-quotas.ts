import { pgTable, date, integer, timestamp } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

// Mirrors migrations/000002_add_ai_quotas.up.sql on the Go backend.
// Was missing from the Drizzle schema entirely — added so this file is
// actually the source of truth it's meant to be.
export const aiQuotasTable = pgTable("ai_quotas", {
  quotaDate: date("quota_date").primaryKey(),
  geminiCalls: integer("gemini_calls").notNull().default(0),
  groq8bCalls: integer("groq8b_calls").notNull().default(0),
  groq70bCalls: integer("groq70b_calls").notNull().default(0),
  updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
});

export const insertAiQuotaSchema = createInsertSchema(aiQuotasTable);
export type InsertAiQuota = z.infer<typeof insertAiQuotaSchema>;
export type AiQuota = typeof aiQuotasTable.$inferSelect;
