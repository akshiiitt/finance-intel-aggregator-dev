import { pgTable, text, serial, timestamp, numeric } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

export const marketSnapshotsTable = pgTable("market_snapshots", {
  id: serial("id").primaryKey(),
  symbol: text("symbol").notNull(),
  name: text("name"),
  exchange: text("exchange").notNull(),
  price: numeric("price").notNull(),
  changePct: numeric("change_pct"),
  changeAbs: numeric("change_abs"),
  prevClose: numeric("prev_close"),
  capturedAt: timestamp("captured_at", { withTimezone: true }).notNull().defaultNow(),
});

export const insertMarketSnapshotSchema = createInsertSchema(marketSnapshotsTable).omit({
  id: true,
  capturedAt: true,
});
export type InsertMarketSnapshot = z.infer<typeof insertMarketSnapshotSchema>;
export type MarketSnapshot = typeof marketSnapshotsTable.$inferSelect;
