import { type ReactNode } from "react";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { cn } from "@/lib/utils";

export interface DataTableColumn<T> {
  key: string;
  header: string;
  /** Right-align numeric/tabular columns. */
  align?: "left" | "right";
  className?: string;
  render: (row: T) => ReactNode;
}

/**
 * Generic table with a real mobile fallback: below md, rows render as
 * stacked key-value cards instead of the old pattern of hiding columns
 * (which on IPO, for example, dropped price band/issue size/dates/subs and
 * left only company+GMP+status — gutting the table's actual value).
 */
export function DataTable<T>({
  columns,
  data,
  keyFn,
  className,
}: {
  columns: DataTableColumn<T>[];
  data: T[];
  keyFn: (row: T) => string | number;
  className?: string;
}) {
  return (
    <>
      {/* Desktop / tablet: real table */}
      <div className={cn("hidden md:block", className)}>
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map(col => (
                <TableHead key={col.key} className={cn(col.align === "right" && "text-right", col.className)}>
                  {col.header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map(row => (
              <TableRow key={keyFn(row)}>
                {columns.map(col => (
                  <TableCell key={col.key} className={cn(col.align === "right" && "text-right tnum", col.className)}>
                    {col.render(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Mobile: stacked key-value cards — every column stays visible. */}
      <div className="flex flex-col gap-2 p-4 md:hidden">
        {data.map(row => (
          <div key={keyFn(row)} className="surface-1 p-4">
            {columns.map(col => (
              <div key={col.key} className="flex items-baseline justify-between gap-3 py-1 first:pt-0 last:pb-0">
                <span className="text-2xs font-medium uppercase tracking-wide text-muted-foreground shrink-0">
                  {col.header}
                </span>
                <span className={cn("text-sm text-right", col.align === "right" && "tnum")}>{col.render(row)}</span>
              </div>
            ))}
          </div>
        ))}
      </div>
    </>
  );
}
