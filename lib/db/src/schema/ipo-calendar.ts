import { pgTable, text, serial, timestamp, numeric, integer, date } from "drizzle-orm/pg-core";
import { createInsertSchema } from "drizzle-zod";
import { z } from "zod/v4";

export const ipoCalendarTable = pgTable("ipo_calendar", {
  id: serial("id").primaryKey(),
  companyName: text("company_name").notNull(),
  exchange: text("exchange"),
  priceBandLow: numeric("price_band_low"),
  priceBandHigh: numeric("price_band_high"),
  lotSize: integer("lot_size"),
  openDate: date("open_date"),
  closeDate: date("close_date"),
  listingDate: date("listing_date"),
  issueSizeCr: numeric("issue_size_cr"),
  gmp: numeric("gmp"),
  subscriptionX: numeric("subscription_x"),
  status: text("status"),
  sector: text("sector"),
  updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
});

export const insertIpoCalendarSchema = createInsertSchema(ipoCalendarTable).omit({
  id: true,
  updatedAt: true,
});
export type InsertIpoCalendar = z.infer<typeof insertIpoCalendarSchema>;
export type IpoCalendar = typeof ipoCalendarTable.$inferSelect;
