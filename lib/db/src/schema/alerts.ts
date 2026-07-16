import { pgTable, text, serial, timestamp, boolean, integer, jsonb, numeric } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

export const alertsTable = pgTable("alerts", {
  id: serial("id").primaryKey(),
  name: text("name").notNull(),
  type: text("type").notNull(),
  conditions: jsonb("conditions").notNull(),
  isActive: boolean("is_active").notNull().default(true),
  lastTriggered: timestamp("last_triggered", { withTimezone: true }),
  triggerCount: integer("trigger_count").notNull().default(0),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
});

export const alertTriggersTable = pgTable("alert_triggers", {
  id: serial("id").primaryKey(),
  alertId: integer("alert_id").notNull(),
  articleId: integer("article_id").notNull(),
  title: text("title").notNull(),
  source: text("source"),
  category: text("category"),
  fiScore: numeric("fi_score"),
  triggeredAt: timestamp("triggered_at", { withTimezone: true }).notNull().defaultNow(),
});

export const insertAlertSchema = createInsertSchema(alertsTable).omit({
  id: true,
  createdAt: true,
  lastTriggered: true,
  triggerCount: true,
});
export type InsertAlert = z.infer<typeof insertAlertSchema>;
export type Alert = typeof alertsTable.$inferSelect;
export type AlertTrigger = typeof alertTriggersTable.$inferSelect;
