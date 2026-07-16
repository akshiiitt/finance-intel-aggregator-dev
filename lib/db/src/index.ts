import { drizzle } from "drizzle-orm/node-postgres";
import pg from "pg";
import * as schema from "./schema";
import { join } from "path";
import { existsSync } from "fs";

try {
  let currentPath = process.cwd();
  let envLoaded = false;
  for (let i = 0; i < 3; i++) {
    const envPath = join(currentPath, ".env");
    if (existsSync(envPath)) {
      (process as any).loadEnvFile(envPath);
      envLoaded = true;
      break;
    }
    currentPath = join(currentPath, "..");
  }
  if (!envLoaded) {
    (process as any).loadEnvFile();
  }
} catch (err) {
  // Ignore
}

const { Pool } = pg;

if (!process.env.DATABASE_URL) {
  throw new Error(
    "DATABASE_URL must be set. Did you forget to provision a database?",
  );
}

export const pool = new Pool({ connectionString: process.env.DATABASE_URL });
export const db = drizzle(pool, { schema });

export * from "./schema";
